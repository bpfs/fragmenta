// package fragmenta 提供FragDB格式的数据结构和操作接口
package fragmenta

import (
	"encoding/binary"
	"errors"
	"math"
)

// ===== 错误常量 =====

var (
	// ErrInvalidFragmenta 无效的格式
	ErrInvalidFragmenta = errors.New("invalid FragDB fragmenta")
	// ErrUnsupportedVersion 不支持的版本
	ErrUnsupportedVersion = errors.New("unsupported FragDB version")
	// ErrInvalidOperation 无效的操作
	ErrInvalidOperation = errors.New("invalid operation")
	// ErrMetadataNotFound 元数据未找到
	ErrMetadataNotFound = errors.New("metadata not found")
	// ErrBlockNotFound 数据块未找到
	ErrBlockNotFound = errors.New("block not found")
	// ErrProtectedMetadata 受保护的元数据不可修改
	ErrProtectedMetadata = errors.New("protected metadata cannot be modified")
	// ErrInvalidQuery 无效的查询
	ErrInvalidQuery = errors.New("invalid query")
	// ErrInvalidArgument 无效的参数
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrStorageLimitExceeded 存储限制超出
	ErrStorageLimitExceeded = errors.New("storage limit exceeded")
	// ErrReadOnly 只读模式
	ErrReadOnly = errors.New("operation not allowed in read-only mode")
	// ErrIndexCorruption 索引损坏
	ErrIndexCorruption = errors.New("index corruption detected")
)

// ===== 魔数和版本常量 =====

const (
	// MagicNumber FragDB格式魔数
	MagicNumber uint32 = 0x44654653 // "DeFS"

	// CurrentVersion 当前格式版本
	CurrentVersion uint16 = 0x0100 // 1.0

	// MinSupportedVersion 最小支持版本
	MinSupportedVersion uint16 = 0x0100 // 1.0
)

// ===== 存储模式常量 =====

const (
	// ContainerMode 容器模式
	ContainerMode uint8 = 1

	// DirectoryMode 目录模式
	DirectoryMode uint8 = 2

	// HybridMode 混合模式
	HybridMode uint8 = 3

	// AutoConvertThreshold 自动转换阈值（字节）
	AutoConvertThreshold uint64 = 1024 * 1024 * 10 // 10MB
)

// ===== 元数据标签常量 =====

const (
	// 系统元数据标签 (0x0000-0x00FF)

	// TagVersion 版本信息
	TagVersion uint16 = 0x0001

	// TagCreateTime 创建时间
	TagCreateTime uint16 = 0x0002

	// TagLastModified 最后修改时间
	TagLastModified uint16 = 0x0003

	// TagTitle 标题
	TagTitle uint16 = 0x0004

	// TagDescription 描述
	TagDescription uint16 = 0x0005

	// TagAuthor 作者
	TagAuthor uint16 = 0x0006

	// TagContentType 内容类型
	TagContentType uint16 = 0x0007

	// TagContentSize 内容大小
	TagContentSize uint16 = 0x0008

	// TagFragmentaType 格式类型
	TagFragmentaType uint16 = 0x0009

	// TagFlags 标志
	TagFlags uint16 = 0x000A

	// 应用元数据标签 (0x0100-0x0FFF)

	// TagApp1 应用1
	TagApp1 uint16 = 0x0100

	// TagApp2 应用2
	TagApp2 uint16 = 0x0101
)

// UserTag 创建用户自定义标签
func UserTag(id uint16) uint16 {
	return 0x1000 + (id & 0x0FFF)
}

// IsUserTag 检查是否是用户标签
func IsUserTag(tag uint16) bool {
	return tag >= 0x1000
}

// IsSystemTag 检查是否是系统标签
func IsSystemTag(tag uint16) bool {
	return tag < 0x0100
}

// IsAppTag 检查是否是应用标签
func IsAppTag(tag uint16) bool {
	return tag >= 0x0100 && tag < 0x1000
}

// ===== 文件标志常量 =====

const (
	// FlagCompressed 压缩标志
	FlagCompressed uint16 = 0x0001

	// FlagEncrypted 加密标志
	FlagEncrypted uint16 = 0x0002

	// FlagReadOnly 只读标志
	FlagReadOnly uint16 = 0x0004

	// FlagIndexed 已索引标志
	FlagIndexed uint16 = 0x0008

	// FlagHasDelta 包含差异数据
	FlagHasDelta uint16 = 0x0010

	// FlagTempFile 临时文件
	FlagTempFile uint16 = 0x0020
)

// ===== 块类型常量 =====

const (
	// NormalBlockType 普通数据块
	NormalBlockType uint8 = 0x00

	// MetadataBlockType 元数据块
	MetadataBlockType uint8 = 0x01

	// IndexBlockType 索引块
	IndexBlockType uint8 = 0x02

	// DeltaBlockType 差异块
	DeltaBlockType uint8 = 0x03

	// XORBlockType 异或块
	XORBlockType uint8 = 0x04

	// CompressionBlockType 压缩块
	CompressionBlockType uint8 = 0x05

	// EncryptedBlockType 加密块
	EncryptedBlockType uint8 = 0x06

	// IndirectBlockType 间接块
	IndirectBlockType uint8 = 0x07

	// SystemBlockType 系统块
	SystemBlockType uint8 = 0xFF
)

// ===== 索引更新模式常量 =====

const (
	// IndexUpdateRealtime 实时更新索引
	IndexUpdateRealtime uint8 = 0x00

	// IndexUpdateBatch 批量更新索引
	IndexUpdateBatch uint8 = 0x01

	// IndexUpdateManual 手动更新索引
	IndexUpdateManual uint8 = 0x02
)

// ===== 查询操作符常量 =====

const (
	// OpEquals 等于
	OpEquals uint8 = 0x00

	// OpNotEquals 不等于
	OpNotEquals uint8 = 0x01

	// OpGreaterThan 大于
	OpGreaterThan uint8 = 0x02

	// OpLessThan 小于
	OpLessThan uint8 = 0x03

	// OpContains 包含
	OpContains uint8 = 0x04
)

// ===== 查询逻辑操作符常量 =====

const (
	// LogicAnd 逻辑与
	LogicAnd uint8 = 0x00

	// LogicOr 逻辑或
	LogicOr uint8 = 0x01
)

// ===== 排序顺序常量 =====

const (
	// SortAscending 升序
	SortAscending uint8 = 0x00

	// SortDescending 降序
	SortDescending uint8 = 0x01
)

// ===== 默认值常量 =====

const (
	// DefaultBlockSize 默认块大小
	DefaultBlockSize uint32 = 4096

	// InlineBlockThreshold 内联块阈值
	InlineBlockThreshold uint32 = 512

	// DefaultIndexCacheSize 默认索引缓存大小
	DefaultIndexCacheSize uint32 = 1024 * 1024 // 1MB
)

// ===== 编码解码工具函数 =====

// EncodeInt64 将int64编码为字节数组
func EncodeInt64(val int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(val))
	return buf
}

// DecodeInt64 从字节数组解码int64
func DecodeInt64(data []byte) int64 {
	if len(data) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(data))
}

// EncodeFloat64 将float64编码为字节数组
func EncodeFloat64(val float64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, math.Float64bits(val))
	return buf
}

// DecodeFloat64 从字节数组解码float64
func DecodeFloat64(data []byte) float64 {
	if len(data) < 8 {
		return 0
	}
	return math.Float64frombits(binary.BigEndian.Uint64(data))
}

// 接口定义将在其他文件中实现
