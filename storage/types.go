// package storage 提供DeFSF格式的存储类型定义
package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// StorageType 存储类型
type StorageType int

const (
	// StorageTypeContainer 容器模式
	StorageTypeContainer StorageType = iota
	// StorageTypeDirectory 目录模式
	StorageTypeDirectory
	// StorageTypeHybrid 混合模式
	StorageTypeHybrid
	// StorageTypeInline 内联模式
	StorageTypeInline
)

// StorageConfig 存储配置
type StorageConfig struct {
	Type                 StorageType
	Path                 string
	AutoConvertThreshold uint64
	BlockSize            uint32
	InlineThreshold      uint32
	DedupEnabled         bool
	CacheSize            uint64
	CachePolicy          string
	// 新增字段：存储策略相关
	StrategyName               string                 // 策略名称，如"simple"或"adaptive"
	EnableStrategyOptimization bool                   // 是否启用策略优化
	StrategyParams             map[string]interface{} // 策略参数
	HotBlockThreshold          uint32                 // 热块阈值
	ColdBlockTimeMinutes       uint32                 // 冷块时间阈值(分钟)
	PerformanceTarget          string                 // 性能目标："balanced","speed","space"
	AutoBalanceEnabled         bool                   // 是否自动平衡存储分布
}

// StorageStats 存储统计信息
type StorageStats struct {
	TotalBlocks        uint32
	TotalSize          uint64
	UsedSpace          uint64
	FreeSpace          uint64
	FragmentationRatio float64
}

// BlockInfo 块信息
type BlockInfo struct {
	ID        uint32
	Size      uint32
	Offset    uint64
	CreatedAt time.Time
	UpdatedAt time.Time
	Checksum  []byte
	RefCount  uint32
}

// BlockLocation 块位置
type BlockLocation struct {
	// StorageType 存储类型
	StorageType StorageType
	// FilePath 文件路径（目录模式下）
	FilePath string
	// Offset 偏移（容器模式下）
	Offset uint64
	// IsInline 是否内联
	IsInline bool
}

// CacheEntry 缓存条目
type CacheEntry struct {
	BlockID     uint32
	Data        []byte
	AccessCount uint32
	LastAccess  time.Time
}

// BlockCache 块缓存
type BlockCache struct {
	Entries     map[uint32]*CacheEntry
	MaxSize     uint64
	CurrentSize uint64
	Policy      string
}

// ContainerStorage 容器存储
type ContainerStorage struct {
	Path          string
	File          *os.File
	BlockMap      map[uint32]uint64
	FreeSpaceList []interface{}
	mutex         sync.RWMutex
	Stats         *StorageStats
}

// WriteBlock 写入块
func (cs *ContainerStorage) WriteBlock(id uint32, data []byte) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// 查找块是否已存在
	if offset, ok := cs.BlockMap[id]; ok {
		// 定位到块的位置
		_, err := cs.File.Seek(int64(offset), io.SeekStart)
		if err != nil {
			return err
		}

		// 读取块大小
		var oldSize uint32
		err = binary.Read(cs.File, binary.BigEndian, &oldSize)
		if err != nil {
			return err
		}

		// 如果新数据大小和旧数据大小一样，可以直接覆盖
		if uint32(len(data)) == oldSize {
			// 回到块数据的起始位置
			_, err = cs.File.Seek(int64(offset)+4, io.SeekStart)
			if err != nil {
				return err
			}

			// 写入数据
			_, err = cs.File.Write(data)
			return err
		}

		// 否则需要删除旧块，重新分配空间
		// 将旧空间添加到空闲列表
		// 实际实现应适当处理空闲空间管理
		cs.Stats.UsedSpace -= uint64(oldSize + 4)
		cs.Stats.FreeSpace += uint64(oldSize + 4)

		// 重新分配空间
		// 在文件末尾写入新块
		newOffset, err := cs.allocateSpace(uint32(len(data)))
		if err != nil {
			return err
		}

		cs.BlockMap[id] = newOffset
		return nil
	}

	// 分配新空间
	newOffset, err := cs.allocateSpace(uint32(len(data)))
	if err != nil {
		return err
	}

	// 更新块映射
	cs.BlockMap[id] = newOffset
	cs.Stats.TotalBlocks++

	return nil
}

// ReadBlock 读取块
func (cs *ContainerStorage) ReadBlock(id uint32) ([]byte, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	// 查找块
	offset, ok := cs.BlockMap[id]
	if !ok {
		return nil, ErrBlockNotFound
	}

	// 定位到块的位置
	_, err := cs.File.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, err
	}

	// 读取块大小
	var size uint32
	err = binary.Read(cs.File, binary.BigEndian, &size)
	if err != nil {
		return nil, err
	}

	// 读取块数据
	data := make([]byte, size)
	_, err = io.ReadFull(cs.File, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// DeleteBlock 删除块
func (cs *ContainerStorage) DeleteBlock(id uint32) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// 查找块
	offset, ok := cs.BlockMap[id]
	if !ok {
		return ErrBlockNotFound
	}

	// 定位到块的位置
	_, err := cs.File.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return err
	}

	// 读取块大小
	var size uint32
	err = binary.Read(cs.File, binary.BigEndian, &size)
	if err != nil {
		return err
	}

	// 更新统计信息
	cs.Stats.UsedSpace -= uint64(size + 4)
	cs.Stats.FreeSpace += uint64(size + 4)
	cs.Stats.TotalBlocks--

	// 从映射中删除
	delete(cs.BlockMap, id)

	// 将空间添加到空闲列表
	// 实际实现应适当处理空闲空间管理

	return nil
}

// GetBlockInfo 获取块信息
func (cs *ContainerStorage) GetBlockInfo(id uint32) (*BlockInfo, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	// 查找块
	offset, ok := cs.BlockMap[id]
	if !ok {
		return nil, ErrBlockNotFound
	}

	// 定位到块的位置
	_, err := cs.File.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, err
	}

	// 读取块大小
	var size uint32
	err = binary.Read(cs.File, binary.BigEndian, &size)
	if err != nil {
		return nil, err
	}

	// 创建块信息
	info := &BlockInfo{
		ID:     id,
		Size:   size,
		Offset: offset,
	}

	return info, nil
}

// Optimize 优化存储
func (cs *ContainerStorage) Optimize() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// 在实际实现中，应进行碎片整理等操作

	return nil
}

// allocateSpace 分配空间
func (cs *ContainerStorage) allocateSpace(size uint32) (uint64, error) {
	// 简单实现：在文件末尾分配空间
	offset, err := cs.File.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	// 写入块大小
	err = binary.Write(cs.File, binary.BigEndian, size)
	if err != nil {
		return 0, err
	}

	// 分配空间用于数据
	zeros := make([]byte, size)
	_, err = cs.File.Write(zeros)
	if err != nil {
		return 0, err
	}

	// 更新统计信息
	cs.Stats.UsedSpace += uint64(size + 4)
	cs.Stats.TotalSize += uint64(size + 4)

	return uint64(offset), nil
}

// DirectoryStorage 目录存储
type DirectoryStorage struct {
	BasePath   string
	MetaPath   string
	BlocksPath string
	TempPath   string
	BlockMap   map[uint32]string
	mutex      sync.RWMutex
	Stats      *StorageStats
}

// WriteBlock 写入块
func (ds *DirectoryStorage) WriteBlock(id uint32, data []byte) error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// 创建块文件路径
	filePath := ds.getBlockPath(id)

	// 检查是否需要删除现有文件
	if oldPath, ok := ds.BlockMap[id]; ok {
		// 获取旧文件大小
		info, err := os.Stat(oldPath)
		if err == nil {
			// 更新统计信息
			ds.Stats.UsedSpace -= uint64(info.Size())
		}

		// 删除旧文件
		_ = os.Remove(oldPath)
	} else {
		// 新块
		ds.Stats.TotalBlocks++
	}

	// 写入块文件
	err := os.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}

	// 更新映射和统计信息
	ds.BlockMap[id] = filePath
	ds.Stats.UsedSpace += uint64(len(data))

	return nil
}

// ReadBlock 读取块
func (ds *DirectoryStorage) ReadBlock(id uint32) ([]byte, error) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	// 查找块
	filePath, ok := ds.BlockMap[id]
	if !ok {
		return nil, ErrBlockNotFound
	}

	// 读取块文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// DeleteBlock 删除块
func (ds *DirectoryStorage) DeleteBlock(id uint32) error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// 查找块
	filePath, ok := ds.BlockMap[id]
	if !ok {
		return ErrBlockNotFound
	}

	// 获取文件大小
	info, err := os.Stat(filePath)
	if err == nil {
		// 更新统计信息
		ds.Stats.UsedSpace -= uint64(info.Size())
	}

	// 删除文件
	err = os.Remove(filePath)
	if err != nil {
		return err
	}

	// 从映射中删除
	delete(ds.BlockMap, id)
	ds.Stats.TotalBlocks--

	return nil
}

// GetBlockInfo 获取块信息
func (ds *DirectoryStorage) GetBlockInfo(id uint32) (*BlockInfo, error) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	// 查找块
	filePath, ok := ds.BlockMap[id]
	if !ok {
		return nil, ErrBlockNotFound
	}

	// 获取文件信息
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	// 创建块信息
	blockInfo := &BlockInfo{
		ID:        id,
		Size:      uint32(info.Size()),
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
	}

	return blockInfo, nil
}

// Optimize 优化存储
func (ds *DirectoryStorage) Optimize() error {
	// 在目录模式下，可能需要整理文件夹结构
	return nil
}

// getBlockPath 获取块文件路径
func (ds *DirectoryStorage) getBlockPath(id uint32) string {
	// 创建层次化的路径，避免单个目录下文件过多
	dir1 := id % 256
	dir2 := (id / 256) % 256

	// 创建目录
	dirPath := os.ExpandEnv(ds.BlocksPath + "/" +
		fmt.Sprintf("%02x", dir1) + "/" + fmt.Sprintf("%02x", dir2))
	os.MkdirAll(dirPath, 0755)

	// 返回文件路径
	return dirPath + "/" + fmt.Sprintf("%08x", id) + ".blk"
}

// HybridStorage 混合存储
type HybridStorage struct {
	Config            *StorageConfig
	Container         *ContainerStorage
	Directory         *DirectoryStorage
	InlineBlocks      map[string][]byte
	mutex             sync.RWMutex
	Stats             *StorageStats
	securityManager   interface{} // 安全管理器引用
	encryptionEnabled bool        // 加密状态标志
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	// 读操作相关指标
	TotalReads     uint64          // 总读取次数
	ReadLatencies  []time.Duration // 读取延迟历史
	AvgReadLatency time.Duration   // 平均读取延迟

	// 写操作相关指标
	TotalWrites     uint64          // 总写入次数
	WriteLatencies  []time.Duration // 写入延迟历史
	AvgWriteLatency time.Duration   // 平均写入延迟

	// 命中率相关
	CacheHits      uint64 // 缓存命中次数
	CacheMisses    uint64 // 缓存未命中次数
	StrategyHits   uint64 // 策略预测命中次数
	StrategyMisses uint64 // 策略预测未命中次数

	// 同步指标
	mutex sync.Mutex
}

// NewPerformanceMetrics 创建新的性能指标记录器
func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		ReadLatencies:  make([]time.Duration, 0, 100),
		WriteLatencies: make([]time.Duration, 0, 100),
	}
}

// RecordRead 记录读取操作
func (pm *PerformanceMetrics) RecordRead(latency time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.TotalReads++

	// 最多保存最近100次操作的延迟
	if len(pm.ReadLatencies) >= 100 {
		// 移除最旧的记录
		pm.ReadLatencies = pm.ReadLatencies[1:]
	}
	pm.ReadLatencies = append(pm.ReadLatencies, latency)

	// 计算平均延迟
	var sum time.Duration
	for _, lat := range pm.ReadLatencies {
		sum += lat
	}
	pm.AvgReadLatency = sum / time.Duration(len(pm.ReadLatencies))
}

// RecordWrite 记录写入操作
func (pm *PerformanceMetrics) RecordWrite(latency time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.TotalWrites++

	// 最多保存最近100次操作的延迟
	if len(pm.WriteLatencies) >= 100 {
		// 移除最旧的记录
		pm.WriteLatencies = pm.WriteLatencies[1:]
	}
	pm.WriteLatencies = append(pm.WriteLatencies, latency)

	// 计算平均延迟
	var sum time.Duration
	for _, lat := range pm.WriteLatencies {
		sum += lat
	}
	pm.AvgWriteLatency = sum / time.Duration(len(pm.WriteLatencies))
}

// RecordCacheHit 记录缓存命中
func (pm *PerformanceMetrics) RecordCacheHit() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.CacheHits++
}

// RecordCacheMiss 记录缓存未命中
func (pm *PerformanceMetrics) RecordCacheMiss() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.CacheMisses++
}

// RecordStrategyHit 记录策略命中
func (pm *PerformanceMetrics) RecordStrategyHit() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.StrategyHits++
}

// RecordStrategyMiss 记录策略未命中
func (pm *PerformanceMetrics) RecordStrategyMiss() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.StrategyMisses++
}

// GetCacheHitRate 获取缓存命中率
func (pm *PerformanceMetrics) GetCacheHitRate() float64 {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	total := pm.CacheHits + pm.CacheMisses
	if total == 0 {
		return 0
	}
	return float64(pm.CacheHits) / float64(total)
}

// GetStrategyHitRate 获取策略命中率
func (pm *PerformanceMetrics) GetStrategyHitRate() float64 {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	total := pm.StrategyHits + pm.StrategyMisses
	if total == 0 {
		return 0
	}
	return float64(pm.StrategyHits) / float64(total)
}

// StorageManager 存储管理器接口
type StorageManager interface {
	// 存储操作
	WriteBlock(id uint32, data []byte) error
	ReadBlock(id uint32) ([]byte, error)
	DeleteBlock(id uint32) error
	GetBlockInfo(id uint32) (*BlockInfo, error)

	// 配置和维护
	Init(config *StorageConfig) error
	Close() error
	GetStats() (*StorageStats, error)
	Optimize() error
	ConvertType(newType StorageType) error

	// 存储模式管理
	GetStorageModeSuggestion() (StorageType, string, error)

	// 安全相关功能
	SetSecurityManager(securityManager interface{}) error
	IsEncryptionEnabled() bool
	SetEncryptionEnabled(enabled bool) error
	EncryptBlock(id uint32, data []byte) ([]byte, error)
	DecryptBlock(id uint32, data []byte) ([]byte, error)
}
