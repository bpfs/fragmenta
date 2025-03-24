// package index 提供DeFSF格式的索引功能
package index

import (
	"sync"
	"time"
)

// IndexConfig 索引配置
type IndexConfig struct {
	// IndexPath 索引文件路径
	IndexPath string
	// AutoSave 是否自动保存
	AutoSave bool
	// AutoRebuild 是否自动重建
	AutoRebuild bool
	// MaxCacheSize 最大缓存大小
	MaxCacheSize int64
	// 新增: 异步更新配置
	AsyncUpdate bool
	// 新增: 最大工作线程数
	MaxWorkers int
	// 新增: 索引压缩级别 (0-不压缩, 1-轻度压缩, 2-中度压缩, 3-高度压缩)
	CompressionLevel int
	// 新增: 索引分片数
	NumShards int
	// 新增: 启用前缀压缩
	EnablePrefixCompression bool
	// 新增: 更新间隔（毫秒）
	UpdateInterval int64
	// 新增: 批量更新阈值
	BatchThreshold int
}

// IndexStatus 索引状态
type IndexStatus struct {
	// TotalItems 总项目数
	TotalItems int
	// IndexedItems 已索引项目数
	IndexedItems int
	// LastUpdateTime 最后更新时间
	LastUpdateTime time.Time
	// IsUpdating 是否正在更新
	IsUpdating bool
	// Progress 进度(0-100)
	Progress int
	// Error 错误信息
	Error string
	// 新增: 待处理更新数
	PendingUpdates int
	// 新增: 活跃工作线程数
	ActiveWorkers int
	// 新增: 压缩率
	CompressionRatio float64
	// 新增: 内存使用量(字节)
	MemoryUsage int64
	// 新增: 分片状态
	ShardStatus []ShardStatus
}

// 新增: 分片状态
type ShardStatus struct {
	// 分片ID
	ShardID int
	// 条目数
	ItemCount int32
	// 是否可用
	Available bool
	// 读取次数
	ReadCount int64
	// 写入次数
	WriteCount int64
	// 最后访问时间
	LastAccess time.Time
}

// 新增: 更新操作类型
type UpdateOperation int

const (
	// 添加操作
	OpAdd UpdateOperation = iota
	// 删除操作
	OpRemove
	// 更新操作
	OpUpdate
)

// 新增: 异步更新任务
type UpdateTask struct {
	// 操作类型
	Operation UpdateOperation
	// 标签
	Tag uint32
	// ID
	ID uint32
	// 任务创建时间
	CreatedAt time.Time
	// 任务优先级 (数值越小优先级越高)
	Priority int
}

// 新增: 索引元数据
type IndexMetadata struct {
	// 索引版本
	Version string
	// 创建时间
	CreatedAt time.Time
	// 最后修改时间
	ModifiedAt time.Time
	// 索引项数量
	ItemCount int
	// 分片数
	ShardCount int
	// 压缩算法
	CompressionAlgorithm string
	// 校验和
	Checksum string
}

// 新增: 前缀压缩节点
type PrefixNode struct {
	// 前缀
	Prefix string
	// 共享前缀的项目数
	Count int
	// 子节点
	Children map[string]*PrefixNode
	// IDs列表
	IDs []uint32
	// 节点深度
	Depth int
}

// IndexManager 索引管理器接口
type IndexManager interface {
	// AddIndex 添加索引
	AddIndex(tag uint32, id uint32) error
	// RemoveIndex 移除索引
	RemoveIndex(tag uint32, id uint32) error
	// FindByKey 根据键查找
	FindByKey(tag uint32) ([]uint32, error)
	// FindByPattern 根据模式查找
	FindByPattern(pattern string) (map[uint32][]uint32, error)
	// UpdateIndices 更新索引
	UpdateIndices() error
	// GetStatus 获取索引状态
	GetStatus() *IndexStatus
	// LoadIndex 加载索引
	LoadIndex(path string) error
	// SaveIndex 保存索引
	SaveIndex(path string) error
	// IndexMetadata 索引元数据
	IndexMetadata(id uint32, tags []uint32) error
	// FindByTag 根据标签查找
	FindByTag(tag uint32) ([]uint32, error)

	// 新增: 异步添加索引
	AsyncAddIndex(tag uint32, id uint32) error
	// 新增: 异步移除索引
	AsyncRemoveIndex(tag uint32, id uint32) error
	// 新增: 批量添加索引
	BatchAddIndices(tags []uint32, ids []uint32) error
	// 新增: 批量移除索引
	BatchRemoveIndices(tags []uint32, ids []uint32) error
	// 新增: 获取索引元数据
	GetIndexMetadata() *IndexMetadata
	// 新增: 按分片获取索引
	FindByTagInShard(tag uint32, shardID int) ([]uint32, error)
	// 新增: 优化索引
	OptimizeIndex() error
	// 新增: 获取待处理更新任务数
	GetPendingTaskCount() int
	// 新增: 获取前缀树
	GetPrefixTree(tag uint32) (*PrefixNode, error)
	// 新增: 前缀搜索
	FindByPrefix(tag uint32, prefix string) ([]uint32, error)
	// 新增: 范围搜索
	FindByRange(tag uint32, start, end uint32) ([]uint32, error)
	// 新增: 复合查询
	FindCompound(conditions []IndexQueryCondition) ([]uint32, error)
}

// 新增: 查询条件
type IndexQueryCondition struct {
	// 标签
	Tag uint32
	// 操作类型
	Operation string // "eq", "neq", "lt", "lte", "gt", "gte", "prefix", "suffix", "contains"
	// 值
	Value interface{}
}

// IndexShard 表示一个索引分片
type IndexShard struct {
	// 分片ID
	ID int

	// 索引映射：标签 -> ID列表
	TagIndices map[uint32][]uint32

	// 内容索引
	ContentIndices map[string][]uint32

	// 统计信息
	Status ShardStatus

	// 锁
	mutex sync.RWMutex
}

// QueryExecutionStats 查询执行统计信息
type QueryExecutionStats struct {
	// 查询开始时间
	StartTime time.Time

	// 查询完成时间
	EndTime time.Time

	// 执行时长
	Duration time.Duration

	// 扫描索引数量
	ScannedIndices int

	// 返回结果数
	ResultCount int

	// 索引命中率
	IndexHitRate float64

	// 分片访问分布
	ShardDistribution map[int]int

	// 内存使用峰值
	PeakMemoryUsage int64

	// 缓存命中率
	CacheHitRate float64
}
