package config

import (
	"time"
)

// Config 系统总体配置
type Config struct {
	// 存储策略配置
	Storage StoragePolicy `json:"storage"`

	// 性能配置
	Performance PerformanceConfig `json:"performance"`

	// 安全策略
	Security SecurityPolicy `json:"security"`

	// 索引策略
	Index IndexPolicy `json:"index"`

	// 系统配置
	System SystemConfig `json:"system"`

	// 元数据
	Metadata ConfigMetadata `json:"metadata"`
}

// StoragePolicy 存储策略配置
type StoragePolicy struct {
	// 存储模式: 容器模式、目录模式或混合模式
	Mode StorageMode `json:"mode"`

	// 容器模式自动转换阈值(字节)
	AutoConvertThreshold int64 `json:"autoConvertThreshold"`

	// 块存储策略
	BlockStrategy BlockStrategy `json:"blockStrategy"`

	// 缓存策略
	CacheStrategy CacheStrategy `json:"cacheStrategy"`

	// 压缩设置
	Compression CompressionSettings `json:"compression"`
}

// StorageMode 存储模式
type StorageMode string

const (
	// 容器模式(单文件)
	ContainerMode StorageMode = "container"

	// 目录模式
	DirectoryMode StorageMode = "directory"

	// 混合模式
	HybridMode StorageMode = "hybrid"
)

// BlockStrategy 块存储策略
type BlockStrategy struct {
	// 块大小(字节)
	BlockSize int `json:"blockSize"`

	// 预分配块数量
	PreallocateBlocks int `json:"preallocateBlocks"`

	// 块缓存大小
	BlockCacheSize int `json:"blockCacheSize"`

	// 允许不对齐写入
	AllowUnalignedWrites bool `json:"allowUnalignedWrites"`
}

// CacheStrategy 缓存策略
type CacheStrategy struct {
	// 元数据缓存大小
	MetadataCacheSize int `json:"metadataCacheSize"`

	// 元数据缓存TTL(秒)
	MetadataCacheTTL int `json:"metadataCacheTTL"`

	// 数据缓存大小
	DataCacheSize int64 `json:"dataCacheSize"`

	// 预读取策略
	PrefetchStrategy string `json:"prefetchStrategy"`

	// 预读取窗口大小
	PrefetchWindowSize int64 `json:"prefetchWindowSize"`
}

// CompressionSettings 压缩设置
type CompressionSettings struct {
	// 启用压缩
	Enabled bool `json:"enabled"`

	// 压缩算法
	Algorithm string `json:"algorithm"`

	// 压缩级别
	Level int `json:"level"`

	// 最小压缩大小
	MinSize int `json:"minSize"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	// 并行处理配置
	Parallelism ParallelismConfig `json:"parallelism"`

	// IO配置
	IO IOConfig `json:"io"`

	// 内存配置
	Memory MemoryConfig `json:"memory"`
}

// ParallelismConfig 并行处理配置
type ParallelismConfig struct {
	// 最大工作线程数
	MaxWorkers int `json:"maxWorkers"`

	// 工作队列长度
	WorkQueueLength int `json:"workQueueLength"`

	// 批处理大小
	BatchSize int `json:"batchSize"`
}

// IOConfig IO配置
type IOConfig struct {
	// 使用直接IO
	UseDirectIO bool `json:"useDirectIO"`

	// 使用异步IO
	UseAsyncIO bool `json:"useAsyncIO"`

	// 文件描述符缓存大小
	FDCacheSize int `json:"fdCacheSize"`

	// 写入合并窗口(毫秒)
	WriteMergeWindow int `json:"writeMergeWindow"`
}

// MemoryConfig 内存配置
type MemoryConfig struct {
	// 最大内存使用量
	MaxMemoryUsage string `json:"maxMemoryUsage"`

	// 内存回收阈值(%)
	ReclamationThreshold int `json:"reclamationThreshold"`

	// 使用内存池
	UseMemoryPool bool `json:"useMemoryPool"`
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	// 加密设置
	Encryption EncryptionSettings `json:"encryption"`

	// 访问控制
	AccessControl AccessControlSettings `json:"accessControl"`
}

// EncryptionSettings 加密设置
type EncryptionSettings struct {
	// 启用加密
	Enabled bool `json:"enabled"`

	// 加密算法
	Algorithm string `json:"algorithm"`

	// 密钥源
	KeySource string `json:"keySource"`
}

// AccessControlSettings 访问控制设置
type AccessControlSettings struct {
	// 启用访问控制
	Enabled bool `json:"enabled"`

	// 访问控制模型
	Model string `json:"model"`
}

// IndexPolicy 索引策略
type IndexPolicy struct {
	// 启用索引
	Enabled bool `json:"enabled"`

	// 索引类型列表
	Types []string `json:"types"`

	// 索引模式 (同步/异步)
	Mode string `json:"mode"`

	// 持久化模式
	PersistenceMode string `json:"persistenceMode"`

	// 索引字段列表
	Fields []IndexField `json:"fields"`
}

// IndexField 索引字段配置
type IndexField struct {
	// 字段名称
	Name string `json:"name"`

	// 是否启用
	Enable bool `json:"enable"`
}

// SystemConfig 系统配置
type SystemConfig struct {
	// 根路径
	RootPath string `json:"rootPath"`

	// 临时路径
	TempPath string `json:"tempPath"`

	// 日志级别
	LogLevel string `json:"logLevel"`

	// 自动清理临时文件
	AutoCleanupTemp bool `json:"autoCleanupTemp"`

	// 启用遥测
	EnableTelemetry bool `json:"enableTelemetry"`
}

// ConfigMetadata 配置元数据
type ConfigMetadata struct {
	// 版本
	Version string `json:"version"`

	// 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`

	// 描述
	Description string `json:"description"`
}
