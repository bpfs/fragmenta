package fragmenta

import (
	"io"
)

// FragDB 定义了格式的主要接口
type FragDB interface {
	// 基本操作
	Close() error
	Commit() error
	GetHeader() *FragmentaHeader

	// 元数据操作
	SetMetadata(tag uint16, value []byte) error
	GetMetadata(tag uint16) ([]byte, error)
	DeleteMetadata(tag uint16) error
	BatchMetadataOp(batch *BatchMetadataOperation) error
	ListMetadata() (map[uint16][]byte, error)

	// 内容操作
	WriteBlock(data []byte, options *BlockOptions) (uint32, error)
	ReadBlock(blockID uint32) ([]byte, error)
	WriteFromReader(reader io.Reader, options *BlockOptions) error
	ReadToWriter(writer io.Writer) error

	// 查询操作
	QueryByTag(tag uint16, value []byte) ([]interface{}, error)
	QueryMetadata(query *MetadataQuery) (*QueryResult, error)

	// 索引操作
	VerifyIndices() (*IndexStatus, error)
	RebuildIndices() error
	StartQueryService() error

	// 高级操作
	ConvertToDirectoryMode() error
	ConvertToContainerMode() error
	OptimizeStorage() error
}

// Fragmenta 是FragDB接口的别名，用于内部实现
type Fragmenta = FragDB

// MetadataManager 提供元数据管理功能
type MetadataManager interface {
	// SetMetadata 设置一个元数据项
	SetMetadata(tag uint16, data []byte) error

	// GetMetadata 获取一个元数据项
	GetMetadata(tag uint16) ([]byte, error)

	// DeleteMetadata 删除一个元数据项
	DeleteMetadata(tag uint16) error

	// ListMetadata 列出所有元数据
	ListMetadata() (map[uint16][]byte, error)

	// BatchOperation 执行批量元数据操作
	BatchOperation(batch *BatchMetadataOperation) error

	// QueryMetadata 查询元数据
	QueryMetadata(query *MetadataQuery) (*QueryResult, error)

	// Flush 将元数据刷新到磁盘
	Flush() error
}

// BlockManager 提供块级别的数据管理能力
type BlockManager interface {
	// WriteBlock 写入数据块，返回块ID和错误
	WriteBlock(data []byte, options *BlockOptions) (uint32, error)

	// ReadBlock 读取指定ID的数据块
	ReadBlock(blockID uint32) ([]byte, error)

	// DeleteBlock 删除指定ID的数据块
	DeleteBlock(blockID uint32) error

	// LinkBlocks 链接两个数据块
	LinkBlocks(sourceID, targetID uint32) error

	// GetBlockInfo 获取块信息
	GetBlockInfo(blockID uint32) (*BlockHeader, error)

	// OptimizeBlocks 优化块存储
	OptimizeBlocks() error
}

// IndexManager 索引管理接口
type IndexManager interface {
	// AddIndex 添加索引
	AddIndex(key []byte, blockID uint32, offset uint32, size uint32) error

	// RemoveIndex 移除索引
	RemoveIndex(key []byte) error

	// FindByKey 通过键查找
	FindByKey(key []byte) ([]IndexEntry, error)

	// FindByPattern 通过模式查找
	FindByPattern(pattern []byte) ([]IndexEntry, error)

	// UpdateIndices 更新索引
	UpdateIndices() error

	// GetStatus 获取索引状态
	GetStatus() (*IndexStatus, error)
}

// QueryService 查询服务接口
type QueryService interface {
	// Query 通用查询
	Query(metadataQuery *MetadataQuery) (*QueryResult, error)

	// GetBlocksByTag 按标签获取块
	GetBlocksByTag(tag uint16, value []byte) ([]uint32, error)

	// GetRelatedBlocks 获取相关块
	GetRelatedBlocks(blockID uint32, relation string) ([]uint32, error)

	// Start 启动查询服务
	Start() error

	// Stop 停止查询服务
	Stop() error
}

// ContentAddressableStorage 内容寻址存储接口
type ContentAddressableStorage interface {
	// Store 存储数据，返回内容哈希
	Store(data []byte) ([]byte, error)
	// Retrieve 通过哈希获取数据
	Retrieve(contentHash []byte) ([]byte, error)
	// Verify 验证内容完整性
	Verify(contentHash []byte) (bool, error)
}

// FSWatcher 文件系统监视器接口
type FSWatcher interface {
	// Start 启动监视
	Start() error
	// Stop 停止监视
	Stop() error
	// RegisterHandler 注册变更处理器
	RegisterHandler(handler interface{})
}

// 工厂方法函数定义

// CreateFragmenta 创建新的格式文件
func CreateFragmenta(path string, options *FragmentaOptions) (Fragmenta, error) {
	// 调用NewFragmenta实现
	return NewFragmenta(path, options)
}

// OpenFragmenta 打开现有格式文件
func OpenFragmenta(path string) (Fragmenta, error) {
	// 调用NewFragmentaFromExisting实现
	return NewFragmentaFromExisting(path)
}

// InitializeStorage 初始化存储
func InitializeStorage(rootPath string, options *StorageOptions) (Fragmenta, error) {
	// 调用NewStorage实现
	return NewStorage(rootPath, options)
}
