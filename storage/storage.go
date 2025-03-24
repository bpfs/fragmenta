// package storage 提供DeFSF格式的存储管理功能
package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 错误定义
var (
	// ErrInvalidMode 表示存储模式无效
	ErrInvalidMode = errors.New("无效的存储模式")

	// ErrInvalidOperation 表示操作无效
	ErrInvalidOperation = errors.New("无效的操作")

	// ErrBlockNotFound 表示请求的块不存在
	ErrBlockNotFound = errors.New("块不存在")
)

// StorageManagerImpl 存储管理器实现
type StorageManagerImpl struct {
	// 配置
	config *StorageConfig

	// 存储模式
	containerStorage *ContainerStorage
	directoryStorage *DirectoryStorage
	hybridStorage    *HybridStorage

	// 同步
	mutex sync.RWMutex

	// 缓存
	blockCache *BlockCache

	// 自动检查通道
	autoCheckStopCh chan struct{}

	// 安全管理器
	securityManager interface{}

	// 加密状态
	encryptionEnabled bool
}

// NewStorageManager 创建存储管理器
func NewStorageManager(config *StorageConfig) (*StorageManagerImpl, error) {
	if config == nil {
		config = &StorageConfig{
			Type:                 StorageTypeContainer,
			Path:                 "",
			AutoConvertThreshold: 10 * 1024 * 1024, // 10MB
			BlockSize:            4096,
			InlineThreshold:      512,
			DedupEnabled:         false,
			CacheSize:            10 * 1024 * 1024, // 10MB
			CachePolicy:          "lru",
		}
	}

	// 创建存储管理器
	sm := &StorageManagerImpl{
		config: config,
		blockCache: &BlockCache{
			Entries:     make(map[uint32]*CacheEntry),
			MaxSize:     config.CacheSize,
			CurrentSize: 0,
			Policy:      config.CachePolicy,
		},
		autoCheckStopCh: make(chan struct{}),
	}

	// 根据存储模式初始化
	var err error
	switch config.Type {
	case StorageTypeContainer:
		sm.containerStorage, err = sm.initContainerStorage()
		if err != nil {
			logger.Error("初始化容器存储失败", "error", err)
			return nil, err
		}
	case StorageTypeDirectory:
		sm.directoryStorage, err = sm.initDirectoryStorage()
		if err != nil {
			logger.Error("初始化目录存储失败", "error", err)
			return nil, err
		}
	case StorageTypeHybrid:
		sm.hybridStorage, err = sm.initHybridStorage()
		if err != nil {
			logger.Error("初始化混合存储失败", "error", err)
			return nil, err
		}
	default:
		logger.Error("无效的存储模式", "error", ErrInvalidMode)
		return nil, ErrInvalidMode
	}

	// 启动自动检查协程
	if config.AutoConvertThreshold > 0 {
		go sm.startAutoCheck()
	}

	return sm, nil
}

// Init 初始化存储
func (sm *StorageManagerImpl) Init(config *StorageConfig) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.config = config

	// 重新初始化存储
	var err error
	switch config.Type {
	case StorageTypeContainer:
		sm.containerStorage, err = sm.initContainerStorage()
		if err != nil {
			logger.Error("重新初始化容器存储失败", "error", err)
			return err
		}
		sm.directoryStorage = nil
		sm.hybridStorage = nil
	case StorageTypeDirectory:
		sm.directoryStorage, err = sm.initDirectoryStorage()
		if err != nil {
			logger.Error("重新初始化目录存储失败", "error", err)
			return err
		}
		sm.containerStorage = nil
		sm.hybridStorage = nil
	case StorageTypeHybrid:
		sm.hybridStorage, err = sm.initHybridStorage()
		if err != nil {
			logger.Error("重新初始化混合存储失败", "error", err)
			return err
		}
		sm.containerStorage = nil
		sm.directoryStorage = nil
	default:
		logger.Error("无效的存储模式", "error", ErrInvalidMode)
		return ErrInvalidMode
	}

	// 初始化缓存
	sm.blockCache = &BlockCache{
		Entries:     make(map[uint32]*CacheEntry),
		MaxSize:     config.CacheSize,
		CurrentSize: 0,
		Policy:      config.CachePolicy,
	}

	return nil
}

// Close 关闭存储
func (sm *StorageManagerImpl) Close() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 停止自动检查协程
	close(sm.autoCheckStopCh)

	// 关闭所有存储
	var err error
	if sm.containerStorage != nil {
		if sm.containerStorage.File != nil {
			err = sm.containerStorage.File.Close()
		}
	}

	// 清理缓存
	sm.blockCache.Entries = make(map[uint32]*CacheEntry)
	sm.blockCache.CurrentSize = 0

	return err
}

// SetSecurityManager 设置安全管理器
func (sm *StorageManagerImpl) SetSecurityManager(securityManager interface{}) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.securityManager = securityManager
	return nil
}

// IsEncryptionEnabled 检查加密是否启用
func (sm *StorageManagerImpl) IsEncryptionEnabled() bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.encryptionEnabled
}

// SetEncryptionEnabled 设置加密状态
func (sm *StorageManagerImpl) SetEncryptionEnabled(enabled bool) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 如果要启用加密，但没有设置安全管理器，返回错误
	if enabled && sm.securityManager == nil {
		return fmt.Errorf("未设置安全管理器，无法启用加密")
	}

	sm.encryptionEnabled = enabled
	return nil
}

// EncryptBlock 加密数据块
func (sm *StorageManagerImpl) EncryptBlock(id uint32, data []byte) ([]byte, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 如果加密未启用或未设置安全管理器，直接返回原始数据
	if !sm.encryptionEnabled || sm.securityManager == nil {
		return data, nil
	}

	// 使用安全管理器加密数据
	if secMgr, ok := sm.securityManager.(interface {
		EncryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error)
	}); ok {
		return secMgr.EncryptBlock(context.Background(), id, data)
	}

	return data, fmt.Errorf("安全管理器不支持加密操作")
}

// DecryptBlock 解密数据块
func (sm *StorageManagerImpl) DecryptBlock(id uint32, data []byte) ([]byte, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 如果加密未启用或未设置安全管理器，直接返回原始数据
	if !sm.encryptionEnabled || sm.securityManager == nil {
		return data, nil
	}

	// 使用安全管理器解密数据
	if secMgr, ok := sm.securityManager.(interface {
		DecryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error)
	}); ok {
		return secMgr.DecryptBlock(context.Background(), id, data)
	}

	return data, fmt.Errorf("安全管理器不支持解密操作")
}

// WriteBlock 写入块
func (sm *StorageManagerImpl) WriteBlock(id uint32, data []byte) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 加密数据（如果启用）
	writeData := data
	var err error
	if sm.encryptionEnabled && sm.securityManager != nil {
		// 直接使用安全管理器，而不是调用EncryptBlock（避免死锁）
		if secMgr, ok := sm.securityManager.(interface {
			EncryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error)
		}); ok {
			writeData, err = secMgr.EncryptBlock(context.Background(), id, data)
			if err != nil {
				logger.Error("加密数据失败", "error", err)
				return err
			}
		} else {
			logger.Warning("安全管理器不支持加密操作，将使用原始数据")
		}
	}

	// 根据存储模式写入
	switch {
	case sm.containerStorage != nil:
		err = sm.containerStorage.WriteBlock(id, writeData)
	case sm.directoryStorage != nil:
		err = sm.directoryStorage.WriteBlock(id, writeData)
	case sm.hybridStorage != nil:
		// 将uint32 ID转换为string键
		idKey := fmt.Sprintf("%d", id)
		err = sm.hybridStorage.WriteBlock(idKey, writeData)
	default:
		err = ErrInvalidMode
	}

	if err != nil {
		logger.Error("写入数据块失败", "error", err)
		return err
	}

	// 更新缓存
	sm.updateCache(id, data)

	return nil
}

// ReadBlock 读取块
func (sm *StorageManagerImpl) ReadBlock(id uint32) ([]byte, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 检查缓存
	if entry, ok := sm.blockCache.Entries[id]; ok {
		entry.AccessCount++
		entry.LastAccess = time.Now()
		return entry.Data, nil
	}

	// 从存储读取
	var data []byte
	var err error

	switch {
	case sm.containerStorage != nil:
		data, err = sm.containerStorage.ReadBlock(id)
	case sm.directoryStorage != nil:
		data, err = sm.directoryStorage.ReadBlock(id)
	case sm.hybridStorage != nil:
		// 将uint32 ID转换为string键
		idKey := fmt.Sprintf("%d", id)
		data, err = sm.hybridStorage.ReadBlock(idKey)
	default:
		return nil, ErrInvalidMode
	}

	if err != nil {
		if err != ErrBlockNotFound {
			logger.Error("读取数据块失败", "error", err)
		}
		return nil, err
	}

	// 解密数据（如果启用）
	if sm.encryptionEnabled && sm.securityManager != nil {
		// 直接使用安全管理器，而不是调用DecryptBlock（避免死锁）
		if secMgr, ok := sm.securityManager.(interface {
			DecryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error)
		}); ok {
			data, err = secMgr.DecryptBlock(context.Background(), id, data)
			if err != nil {
				logger.Error("解密数据失败", "error", err)
				return nil, err
			}
		} else {
			logger.Warning("安全管理器不支持解密操作，将使用原始数据")
		}
	}

	// 更新缓存
	sm.updateCache(id, data)

	return data, nil
}

// DeleteBlock 删除块
func (sm *StorageManagerImpl) DeleteBlock(id uint32) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 从缓存中删除
	if _, ok := sm.blockCache.Entries[id]; ok {
		delete(sm.blockCache.Entries, id)
	}

	// 从存储中删除
	var err error
	switch {
	case sm.containerStorage != nil:
		err = sm.containerStorage.DeleteBlock(id)
	case sm.directoryStorage != nil:
		err = sm.directoryStorage.DeleteBlock(id)
	case sm.hybridStorage != nil:
		// 将uint32 ID转换为string键
		idKey := fmt.Sprintf("%d", id)
		err = sm.hybridStorage.DeleteBlock(idKey)
	default:
		err = ErrInvalidMode
	}

	if err != nil {
		if err != ErrBlockNotFound {
			logger.Error("删除数据块失败", "error", err)
		}
		return err
	}

	return nil
}

// GetBlockInfo 获取块信息
func (sm *StorageManagerImpl) GetBlockInfo(id uint32) (*BlockInfo, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	switch {
	case sm.containerStorage != nil:
		return sm.containerStorage.GetBlockInfo(id)
	case sm.directoryStorage != nil:
		return sm.directoryStorage.GetBlockInfo(id)
	case sm.hybridStorage != nil:
		// 将uint32 ID转换为string键
		idKey := fmt.Sprintf("%d", id)
		info, _, err := sm.hybridStorage.GetBlockInfo(idKey)
		// 忽略location信息，符合接口定义
		return info, err
	default:
		return nil, ErrInvalidMode
	}
}

// GetStats 获取统计信息
func (sm *StorageManagerImpl) GetStats() (*StorageStats, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 根据存储模式获取
	switch sm.config.Type {
	case StorageTypeContainer:
		return sm.containerStorage.Stats, nil
	case StorageTypeDirectory:
		return sm.directoryStorage.Stats, nil
	case StorageTypeHybrid:
		return sm.hybridStorage.Stats, nil
	default:
		return nil, ErrInvalidMode
	}
}

// Optimize 优化存储
func (sm *StorageManagerImpl) Optimize() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 根据存储模式优化
	switch sm.config.Type {
	case StorageTypeContainer:
		return sm.containerStorage.Optimize()
	case StorageTypeDirectory:
		return sm.directoryStorage.Optimize()
	case StorageTypeHybrid:
		return sm.hybridStorage.Optimize()
	default:
		return ErrInvalidMode
	}
}

// ConvertType 转换存储模式
func (sm *StorageManagerImpl) ConvertType(newType StorageType) error {
	// 首先验证新模式，不加锁
	if newType != StorageTypeContainer &&
		newType != StorageTypeDirectory &&
		newType != StorageTypeHybrid {
		return ErrInvalidMode
	}

	sm.mutex.Lock()
	// 如果类型相同，直接返回
	if sm.config.Type == newType {
		sm.mutex.Unlock()
		return nil
	}

	// 记录旧模式
	oldType := sm.config.Type
	sm.mutex.Unlock()

	// 获取当前存储统计（必须在锁外调用以避免死锁）
	stats, err := sm.GetStats()
	if err != nil {
		logger.Error("获取存储统计失败", "error", err)
		return fmt.Errorf("获取存储统计失败: %w", err)
	}

	// 如果没有数据，直接变更类型并初始化新的存储（加锁）
	if stats.TotalBlocks == 0 {
		sm.mutex.Lock()
		sm.config.Type = newType

		// 根据新类型初始化相应的存储
		var initErr error
		switch newType {
		case StorageTypeContainer:
			sm.containerStorage, initErr = sm.initContainerStorage()
		case StorageTypeDirectory:
			sm.directoryStorage, initErr = sm.initDirectoryStorage()
		case StorageTypeHybrid:
			sm.hybridStorage, initErr = sm.initHybridStorage()
		}

		if initErr != nil {
			// 初始化失败，回滚类型
			sm.config.Type = oldType
			sm.mutex.Unlock()
			logger.Error("初始化新存储失败", "error", initErr)
			return fmt.Errorf("初始化新存储失败: %w", initErr)
		}

		sm.mutex.Unlock()
		logger.Info("存储为空，直接转换模式", "旧模式", oldType, "新模式", newType)
		return nil
	}

	// 如果是目录模式，检查并处理文件/目录冲突
	if newType == StorageTypeDirectory {
		// 检查路径是否为文件
		fileInfo, err := os.Stat(sm.config.Path)
		if err == nil && !fileInfo.IsDir() {
			// 删除文件之前先取得锁，确保没有其他操作正在访问文件
			sm.mutex.Lock()
			// 再次检查类型，防止并发修改
			if sm.config.Type == oldType {
				sm.mutex.Unlock()
				// 是文件，需要删除
				err = os.Remove(sm.config.Path)
				if err != nil {
					logger.Error("删除已存在的存储文件失败", "path", sm.config.Path, "error", err)
					return fmt.Errorf("删除已存在的存储文件失败: %w", err)
				}
				logger.Info("已删除旧的存储文件，准备创建目录", "path", sm.config.Path)
			} else {
				sm.mutex.Unlock()
				// 类型已经被其他goroutine修改，返回
				return nil
			}
		}
	}

	// 创建临时目录存储转换数据
	tempDir, err := os.MkdirTemp("", "storage_convert_*")
	if err != nil {
		logger.Error("创建临时目录失败", "error", err)
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	logger.Info("开始转换存储模式", "从", oldType, "到", newType, "临时目录", tempDir)

	// 设置临时存储配置
	tempConfig := &StorageConfig{
		Type:                 StorageTypeDirectory, // 使用目录模式作为中间转换格式
		Path:                 tempDir,
		BlockSize:            sm.config.BlockSize,
		AutoConvertThreshold: 0, // 禁用自动转换
		InlineThreshold:      sm.config.InlineThreshold,
		DedupEnabled:         sm.config.DedupEnabled,
		CacheSize:            sm.config.CacheSize,
		CachePolicy:          sm.config.CachePolicy,
	}

	// 创建临时存储管理器（不会启动自动检查）
	tempSM, err := NewStorageManager(tempConfig)
	if err != nil {
		logger.Error("创建临时存储管理器失败", "error", err)
		return fmt.Errorf("创建临时存储管理器失败: %w", err)
	}
	defer tempSM.Close()

	// 通过ID范围复制数据
	maxBlockID := uint32(stats.TotalBlocks * 2) // 使用足够大的范围

	blocksCopied := 0
	for id := uint32(0); id < maxBlockID && blocksCopied < int(stats.TotalBlocks); id++ {
		data, err := sm.ReadBlock(id)
		if err != nil {
			// 块不存在，继续
			if errors.Is(err, ErrBlockNotFound) {
				continue
			}
			logger.Error("读取块数据失败", "id", id, "error", err)
			continue
		}

		// 写入临时存储
		err = tempSM.WriteBlock(id, data)
		if err != nil {
			logger.Error("写入临时存储失败", "id", id, "error", err)
			continue
		}

		blocksCopied++
	}

	logger.Info("已复制块到临时存储", "总块数", stats.TotalBlocks, "实际复制", blocksCopied)

	// 初始化新类型的存储
	var newStorage interface{}
	var initErr error

	// 根据新类型初始化相应的存储
	switch newType {
	case StorageTypeContainer:
		newStorage, initErr = sm.initContainerStorage()
	case StorageTypeDirectory:
		// 需要创建目录的场景
		if oldType == StorageTypeContainer && newType == StorageTypeDirectory {
			if err := os.MkdirAll(sm.config.Path, 0755); err != nil {
				logger.Error("创建存储目录失败", "error", err)
				return fmt.Errorf("创建存储目录失败: %w", err)
			}
		}
		newStorage, initErr = sm.initDirectoryStorage()
	case StorageTypeHybrid:
		newStorage, initErr = sm.initHybridStorage()
	}

	if initErr != nil {
		logger.Error("初始化新存储失败", "error", initErr)
		return fmt.Errorf("初始化新存储失败: %w", initErr)
	}

	// 更新存储类型（加锁）
	sm.mutex.Lock()
	// 再次检查类型，防止并发修改
	if sm.config.Type != oldType {
		sm.mutex.Unlock()
		return fmt.Errorf("存储类型已被其他线程修改")
	}

	// 更新存储管理器的存储实例
	switch newType {
	case StorageTypeContainer:
		sm.containerStorage = newStorage.(*ContainerStorage)
	case StorageTypeDirectory:
		sm.directoryStorage = newStorage.(*DirectoryStorage)
	case StorageTypeHybrid:
		sm.hybridStorage = newStorage.(*HybridStorage)
	}

	// 更新类型
	sm.config.Type = newType
	sm.mutex.Unlock()

	// 从临时存储复制回主存储
	restoredBlocks := 0
	for id := uint32(0); id < maxBlockID; id++ {
		data, err := tempSM.ReadBlock(id)
		if err != nil {
			// 块不存在，继续
			if errors.Is(err, ErrBlockNotFound) {
				continue
			}
			logger.Error("从临时存储读取块失败", "id", id, "error", err)
			continue
		}

		err = sm.WriteBlock(id, data)
		if err != nil {
			logger.Error("写回主存储失败", "id", id, "error", err)
			continue
		}
		restoredBlocks++
	}

	logger.Info("存储模式转换成功",
		"旧模式", oldType,
		"新模式", newType,
		"块数", blocksCopied,
		"恢复块数", restoredBlocks)

	return nil
}

// 内部辅助方法

// NewContainerStorage 创建新的容器存储
func NewContainerStorage(config *StorageConfig) (*ContainerStorage, error) {
	if config.Path == "" {
		return nil, errors.New("path not specified")
	}

	// 检查文件是否存在
	_, err := os.Stat(config.Path)
	if os.IsNotExist(err) {
		// 创建新文件
		file, err := os.Create(config.Path)
		if err != nil {
			logger.Error("创建新文件失败", "error", err)
			return nil, err
		}

		cs := &ContainerStorage{
			Path:          config.Path,
			File:          file,
			BlockMap:      make(map[uint32]uint64),
			FreeSpaceList: []interface{}{},
			Stats: &StorageStats{
				TotalBlocks:        0,
				TotalSize:          0,
				UsedSpace:          0,
				FreeSpace:          0,
				FragmentationRatio: 0.0,
			},
		}

		return cs, nil
	} else if err != nil {
		logger.Error("检查文件是否存在失败", "error", err)
		return nil, err
	}

	// 打开现有文件
	file, err := os.OpenFile(config.Path, os.O_RDWR, 0)
	if err != nil {
		logger.Error("打开现有文件失败", "error", err)
		return nil, err
	}

	cs := &ContainerStorage{
		Path:          config.Path,
		File:          file,
		BlockMap:      make(map[uint32]uint64),
		FreeSpaceList: []interface{}{},
		Stats: &StorageStats{
			TotalBlocks:        0,
			TotalSize:          0,
			UsedSpace:          0,
			FreeSpace:          0,
			FragmentationRatio: 0.0,
		},
	}

	// 加载块映射
	// 实际实现应从文件中加载

	return cs, nil
}

// NewDirectoryStorage 创建新的目录存储
func NewDirectoryStorage(config *StorageConfig) (*DirectoryStorage, error) {
	if config.Path == "" {
		return nil, errors.New("path not specified")
	}

	// 确保目录存在
	err := os.MkdirAll(config.Path, 0755)
	if err != nil {
		logger.Error("创建目录失败", "error", err)
		return nil, err
	}

	// 创建子目录
	blocksPath := filepath.Join(config.Path, "blocks")
	err = os.MkdirAll(blocksPath, 0755)
	if err != nil {
		logger.Error("创建子目录失败", "error", err)
		return nil, err
	}

	tempPath := filepath.Join(config.Path, "temp")
	err = os.MkdirAll(tempPath, 0755)
	if err != nil {
		logger.Error("创建临时目录失败", "error", err)
		return nil, err
	}

	ds := &DirectoryStorage{
		BasePath:   config.Path,
		MetaPath:   filepath.Join(config.Path, "meta.idx"),
		BlocksPath: blocksPath,
		TempPath:   tempPath,
		BlockMap:   make(map[uint32]string),
		Stats: &StorageStats{
			TotalBlocks:        0,
			TotalSize:          0,
			UsedSpace:          0,
			FreeSpace:          0,
			FragmentationRatio: 0.0,
		},
	}

	// 加载块映射
	// 实际实现应从meta.idx文件中加载

	return ds, nil
}

// initContainerStorage 初始化容器存储
func (sm *StorageManagerImpl) initContainerStorage() (*ContainerStorage, error) {
	return NewContainerStorage(sm.config)
}

// initDirectoryStorage 初始化目录存储
func (sm *StorageManagerImpl) initDirectoryStorage() (*DirectoryStorage, error) {
	return NewDirectoryStorage(sm.config)
}

// initHybridStorage 初始化混合存储
func (sm *StorageManagerImpl) initHybridStorage() (*HybridStorage, error) {
	// 使用NewHybridStorage函数创建混合存储
	hs, err := NewHybridStorage(sm.config)
	if err != nil {
		return nil, err
	}
	return hs, nil
}

// updateCache 更新缓存
func (sm *StorageManagerImpl) updateCache(id uint32, data []byte) {
	// 检查缓存空间
	if uint64(len(data)) > sm.blockCache.MaxSize {
		return // 数据过大，不缓存
	}

	// 如果已存在，先移除
	if entry, ok := sm.blockCache.Entries[id]; ok {
		sm.blockCache.CurrentSize -= uint64(len(entry.Data))
	}

	// 需要清理缓存
	if sm.blockCache.CurrentSize+uint64(len(data)) > sm.blockCache.MaxSize {
		sm.evictCache(uint64(len(data)))
	}

	// 添加到缓存
	sm.blockCache.Entries[id] = &CacheEntry{
		BlockID:     id,
		Data:        data,
		AccessCount: 1,
		LastAccess:  time.Now(),
	}
	sm.blockCache.CurrentSize += uint64(len(data))
}

// evictCache 清理缓存
func (sm *StorageManagerImpl) evictCache(requiredSpace uint64) {
	// 简单LRU实现
	if sm.blockCache.Policy == "lru" {
		// 按最后访问时间排序
		type cacheItem struct {
			id         uint32
			lastAccess time.Time
			size       uint64
		}

		items := make([]cacheItem, 0, len(sm.blockCache.Entries))
		for id, entry := range sm.blockCache.Entries {
			items = append(items, cacheItem{
				id:         id,
				lastAccess: entry.LastAccess,
				size:       uint64(len(entry.Data)),
			})
		}

		// 按访问时间排序
		for i := 0; i < len(items)-1; i++ {
			for j := i + 1; j < len(items); j++ {
				if items[i].lastAccess.After(items[j].lastAccess) {
					items[i], items[j] = items[j], items[i]
				}
			}
		}

		// 从最旧的开始移除
		spaceFreed := uint64(0)
		for _, item := range items {
			if spaceFreed >= requiredSpace {
				break
			}

			entry, ok := sm.blockCache.Entries[item.id]
			if ok {
				spaceFreed += uint64(len(entry.Data))
				sm.blockCache.CurrentSize -= uint64(len(entry.Data))
				delete(sm.blockCache.Entries, item.id)
			}
		}
	}
}

// checkAndAutoConvert 检查是否需要自动转换存储模式
func (sm *StorageManagerImpl) checkAndAutoConvert() {
	// 仅当配置了自动转换阈值时才检查
	if sm.config.AutoConvertThreshold <= 0 {
		return
	}

	// 获取当前统计信息（获取读锁）
	sm.mutex.RLock()
	stats, err := sm.getStatsNoLock()
	currentType := sm.config.Type
	threshold := sm.config.AutoConvertThreshold
	sm.mutex.RUnlock()

	if err != nil {
		logger.Error("获取存储统计信息失败", "error", err)
		return
	}

	// 基于当前状态评估最佳存储模式（不持有锁）
	recommendedMode, reason := sm.evaluateStorageModeNoLock(stats, currentType, threshold)

	// 如果建议的模式与当前模式不同，且满足转换条件，则执行转换
	if recommendedMode != currentType {
		logger.Info("触发自动存储模式转换",
			"当前大小", stats.UsedSpace,
			"转换阈值", threshold,
			"当前模式", currentType,
			"建议模式", recommendedMode,
			"原因", reason)

		// 执行转换（转换函数会加自己的锁）
		err = sm.ConvertType(recommendedMode)
		if err != nil {
			logger.Error("自动转换存储模式失败", "error", err)
			return
		}

		logger.Info("自动转换存储模式成功",
			"当前块数", stats.TotalBlocks,
			"总大小", stats.UsedSpace,
			"新模式", recommendedMode)
	}
}

// getStatsNoLock 获取统计信息（内部使用，不加锁）
func (sm *StorageManagerImpl) getStatsNoLock() (*StorageStats, error) {
	// 根据存储模式获取
	switch sm.config.Type {
	case StorageTypeContainer:
		return sm.containerStorage.Stats, nil
	case StorageTypeDirectory:
		return sm.directoryStorage.Stats, nil
	case StorageTypeHybrid:
		return sm.hybridStorage.Stats, nil
	default:
		return nil, ErrInvalidMode
	}
}

// evaluateStorageModeNoLock 评估最适合的存储模式（内部使用，不加锁）
func (sm *StorageManagerImpl) evaluateStorageModeNoLock(stats *StorageStats, currentType StorageType, threshold uint64) (StorageType, string) {
	// 如果总大小小于阈值的50%，推荐使用容器模式
	if stats.UsedSpace < threshold/2 {
		return StorageTypeContainer, "存储大小较小，适合容器模式"
	}

	// 如果总大小超过阈值，推荐使用目录模式
	if stats.UsedSpace >= threshold {
		return StorageTypeDirectory, "存储大小超过阈值，适合目录模式"
	}

	// 大量小块数据（平均大小小于InlineThreshold）且大小接近阈值，推荐使用混合模式
	if stats.TotalBlocks > 0 {
		averageBlockSize := stats.UsedSpace / uint64(stats.TotalBlocks)
		if averageBlockSize < uint64(sm.config.InlineThreshold) &&
			stats.UsedSpace >= threshold*3/4 {
			return StorageTypeHybrid, "存储块大小分布不均，适合混合模式"
		}
	}

	// 碎片率高，推荐使用目录模式
	if stats.FragmentationRatio > 0.3 {
		return StorageTypeDirectory, "碎片率高，适合目录模式"
	}

	// 默认返回当前模式，不做改变
	return currentType, "当前模式运行良好，无需改变"
}

// EvaluateStorageMode 评估最适合的存储模式
func (sm *StorageManagerImpl) EvaluateStorageMode(stats *StorageStats) (StorageType, string) {
	return sm.evaluateStorageModeNoLock(stats, sm.config.Type, sm.config.AutoConvertThreshold)
}

// GetStorageModeSuggestion 获取存储模式建议
func (sm *StorageManagerImpl) GetStorageModeSuggestion() (StorageType, string, error) {
	stats, err := sm.GetStats()
	if err != nil {
		logger.Error("获取存储统计信息失败", "error", err)
		return sm.config.Type, "无法评估", err
	}

	recommendedMode, reason := sm.EvaluateStorageMode(stats)
	// 返回推荐的存储模式和原因
	return recommendedMode, fmt.Sprintf("当前模式: %v, 建议模式: %v, 原因: %s",
		sm.config.Type, recommendedMode, reason), nil
}

// startAutoCheck 启动自动检查
func (sm *StorageManagerImpl) startAutoCheck() {
	// 设置检查间隔，根据存储大小自动调整
	checkInterval := 30 * time.Second

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取统计信息
			stats, err := sm.GetStats()
			if err != nil {
				logger.Error("自动检查获取统计信息失败", "error", err)
				continue
			}

			// 动态调整检查间隔
			if stats.TotalBlocks > 10000 {
				checkInterval = 5 * time.Minute
			} else if stats.TotalBlocks > 1000 {
				checkInterval = 2 * time.Minute
			} else {
				checkInterval = 30 * time.Second
			}
			ticker.Reset(checkInterval)

			// 在非IO高峰期检查是否需要转换模式
			sm.mutex.Lock()
			sm.checkAndAutoConvert()
			sm.mutex.Unlock()

		case <-sm.autoCheckStopCh:
			return
		}
	}
}
