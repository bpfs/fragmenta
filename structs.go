package fragmenta

import (
	"time"
)

// FragmentaHeader 定义FragDB文件头部结构
type FragmentaHeader struct {
	Magic          uint32   // 魔数，用于标识文件格式
	Version        uint16   // 版本号
	Flags          uint16   // 文件标志
	Timestamp      int64    // 创建时间戳
	LastModified   int64    // 最后修改时间戳
	StorageMode    uint8    // 存储模式
	Reserved1      uint8    // 保留字段
	Reserved2      uint16   // 保留字段
	MetadataOffset uint64   // 元数据区起始偏移
	MetadataSize   uint64   // 元数据区大小
	BlockOffset    uint64   // 数据块区起始偏移
	BlockSize      uint64   // 数据块区大小
	IndexOffset    uint64   // 索引区起始偏移
	IndexSize      uint64   // 索引区大小
	TotalSize      uint64   // 文件总大小
	UserDefinedID  [16]byte // 用户定义的唯一标识
	CheckSum       [32]byte // 校验和（SHA-256）
}

// BlockHeader 定义数据块头部结构
type BlockHeader struct {
	BlockID       uint32   // 块ID
	BlockType     uint8    // 块类型
	Flags         uint8    // 块标志
	Reserved      uint16   // 保留字段
	Size          uint32   // 数据大小
	Checksum      [16]byte // 块数据校验和（MD5）
	PreviousBlock uint32   // 前一个块ID（如果是链式存储）
	NextBlock     uint32   // 下一个块ID（如果是链式存储）
	Timestamp     int64    // 创建时间戳
}

// ExtendedBlockHeader 扩展块头部，用于特殊块类型
type ExtendedBlockHeader struct {
	BlockHeader               // 继承基本块头部
	ExtendedType       uint8  // 扩展类型
	CompressionType    uint8  // 压缩类型
	EncryptionType     uint8  // 加密类型
	Reserved           uint8  // 保留字段
	OriginalSize       uint32 // 原始数据大小（压缩前）
	ExtendedAttributes []byte // 扩展属性
}

// MetadataEntry 元数据条目结构
type MetadataEntry struct {
	Tag      uint16 // 元数据标签
	Size     uint16 // 数据大小
	Value    []byte // 元数据值
	Flags    uint8  // 元数据标志
	Reserved uint8  // 保留字段
}

// IndexEntry 索引条目结构
type IndexEntry struct {
	Key       []byte // 索引键
	BlockID   uint32 // 所引用的数据块ID
	Offset    uint32 // 数据块内部偏移
	Size      uint32 // 引用的数据大小
	Type      uint8  // 索引类型
	Flags     uint8  // 索引标志
	Reserved  uint16 // 保留字段
	Timestamp int64  // 创建时间戳
}

// FragmentaOptions 格式选项
type FragmentaOptions struct {
	StorageMode       uint8  // 存储模式（容器或目录）
	BlockSize         uint32 // 块大小
	IndexUpdateMode   uint8  // 索引更新模式
	MaxIndexCacheSize uint32 // 最大索引缓存大小
	DedupEnabled      bool   // 是否启用重复数据删除
}

// StorageOptions 存储选项
type StorageOptions struct {
	DefaultMode          uint8  // 默认存储模式
	AutoConvertThreshold uint64 // 自动转换模式的阈值
	BlockSize            uint32 // 块大小
	InlineThreshold      uint32 // 内联存储阈值
	DedupEnabled         bool   // 是否启用重复数据删除
	CacheSize            uint32 // 缓存大小
	CachePolicy          string // 缓存策略
}

// BlockOptions 块写入选项
type BlockOptions struct {
	BlockType       uint8             // 块类型
	Compress        bool              // 是否压缩
	Encrypt         bool              // 是否加密
	Checksum        bool              // 是否计算校验和
	EncryptionKey   []byte            // 加密密钥（如果启用加密）
	CompressionType uint8             // 压缩类型
	MetadataTags    map[uint16][]byte // 块关联的元数据标签
	AppendToBlockID uint32            // 要附加到的块ID（如果使用链式存储）
	PriorityClass   uint8             // 优先级类别（用于缓存和存储管理）
	LifecyclePolicy uint8             // 生命周期策略
}

// IndexStatus 索引状态信息
type IndexStatus struct {
	TotalEntries    uint32    // 总条目数
	ValidEntries    uint32    // 有效条目数
	InvalidEntries  uint32    // 无效条目数
	LastVerified    time.Time // 最后验证时间
	RebuildRequired bool      // 是否需要重建
	IntegrityScore  float64   // 完整性评分（0-1）
	IndexLoadTime   uint32    // 加载时间（毫秒）
	DeferredUpdates uint32    // 延迟更新数
}

// MetadataQuery 元数据查询参数
type MetadataQuery struct {
	Conditions []MetadataCondition // 查询条件
	Operator   uint8               // 条件组合操作符（AND/OR）
	Limit      uint32              // 限制结果数量
	Offset     uint32              // 结果偏移
	SortBy     uint16              // 排序依据的标签
	SortOrder  uint8               // 排序顺序（升序/降序）
}

// MetadataCondition 元数据查询条件
type MetadataCondition struct {
	Tag      uint16 // 元数据标签
	Operator uint8  // 操作符（等于/不等于/大于/小于/包含）
	Value    []byte // 比较值
	Flags    uint8  // 附加标志
}

// QueryResult 查询结果
type QueryResult struct {
	Entries     []ResultEntry // 结果条目
	TotalCount  uint32        // 总匹配条数
	ReturnCount uint32        // 返回条数
	HasMore     bool          // 是否有更多结果
	QueryTime   uint32        // 查询用时（毫秒）
}

// ResultEntry 结果条目
type ResultEntry struct {
	BlockID      uint32            // 块ID
	MetadataName string            // 元数据名称
	MetadataID   uint16            // 元数据ID
	MetadataData []byte            // 元数据内容
	ExtraData    map[string][]byte // 额外数据
}

// BatchMetadataOperation 批量元数据操作
type BatchMetadataOperation struct {
	Operations      []MetadataOperation // 操作列表
	AtomicExec      bool                // 是否原子执行
	RollbackOnError bool                // 错误时是否回滚
}

// MetadataOperation 元数据操作
type MetadataOperation struct {
	Operation uint8  // 操作类型 (0=设置, 1=删除, 2=附加)
	Tag       uint16 // 元数据标签
	Value     []byte // 元数据值 (仅对设置和附加有效)
	Flags     uint8  // 操作标志
}
