package fragmenta

import (
	"encoding/binary"
	"io"
	"os"
	"sync"
	"time"
)

// 创建实体类型的别名，解决循环引用问题
var (
	// IndexServices 存储索引服务
	IndexServices = make(map[string]interface{})
	// StorageServices 存储管理服务
	StorageServices = make(map[string]interface{})
)

// FragmentaImpl 是Fragmenta接口的具体实现
type FragmentaImpl struct {
	// 文件相关
	path         string
	file         *os.File
	header       FragmentaHeader
	isNew        bool
	isDirty      bool
	lastModified time.Time

	// 状态和锁
	isOpen     bool
	readOnly   bool
	writeMutex sync.RWMutex

	// 组件
	storageManager  interface{} // storage.StorageManager
	metadataManager MetadataManager
	blockManager    BlockManager
	indexManager    interface{} // index.IndexManager
	queryService    interface{} // *index.QueryService

	// 内部缓存
	metadataCache map[uint16][]byte
	blockCache    map[uint32][]byte
}

// 实现Fragmenta接口

// Close 关闭文件
func (f *FragmentaImpl) Close() error {
	if !f.isOpen {
		return nil
	}

	f.writeMutex.Lock()
	defer f.writeMutex.Unlock()

	// 如果有未提交的更改，先提交
	if f.isDirty {
		if err := f.Commit(); err != nil {
			logger.Error("关闭文件失败", "error", err)
			return err
		}
	}

	// 关闭文件
	err := f.file.Close()
	if err == nil {
		f.isOpen = false
	}
	return err
}

// Commit 提交更改
func (f *FragmentaImpl) Commit() error {
	f.writeMutex.Lock()
	defer f.writeMutex.Unlock()

	if !f.isDirty {
		return nil
	}

	if f.readOnly {
		return ErrReadOnly
	}

	// 更新最后修改时间
	f.header.LastModified = time.Now().UnixNano()

	// 刷新元数据到文件
	if err := f.flushMetadata(); err != nil {
		logger.Error("刷新元数据失败", "error", err)
		return err
	}

	// 刷新头部信息到文件
	if err := f.writeHeader(); err != nil {
		logger.Error("刷新头部信息失败", "error", err)
		return err
	}

	f.isDirty = false
	return nil
}

// GetHeader 获取文件头
func (f *FragmentaImpl) GetHeader() *FragmentaHeader {
	return &f.header
}

// SetMetadata 设置元数据
func (f *FragmentaImpl) SetMetadata(tag uint16, value []byte) error {
	if f.readOnly {
		return ErrReadOnly
	}

	err := f.metadataManager.SetMetadata(tag, value)
	if err != nil {
		logger.Error("设置元数据失败", "error", err)
		return err
	}

	f.isDirty = true
	return nil
}

// GetMetadata 获取元数据
func (f *FragmentaImpl) GetMetadata(tag uint16) ([]byte, error) {
	return f.metadataManager.GetMetadata(tag)
}

// DeleteMetadata 删除元数据
func (f *FragmentaImpl) DeleteMetadata(tag uint16) error {
	if f.readOnly {
		return ErrReadOnly
	}

	err := f.metadataManager.DeleteMetadata(tag)
	if err != nil {
		logger.Error("删除元数据失败", "error", err)
		return err
	}

	f.isDirty = true
	return nil
}

// BatchMetadataOp 批量元数据操作
func (f *FragmentaImpl) BatchMetadataOp(batch *BatchMetadataOperation) error {
	if f.readOnly {
		return ErrReadOnly
	}

	err := f.metadataManager.BatchOperation(batch)
	if err != nil {
		logger.Error("批量元数据操作失败", "error", err)
		return err
	}

	f.isDirty = true
	return nil
}

// ListMetadata 列出所有元数据
func (f *FragmentaImpl) ListMetadata() (map[uint16][]byte, error) {
	return f.metadataManager.ListMetadata()
}

// WriteBlock 写入数据块
func (f *FragmentaImpl) WriteBlock(data []byte, options *BlockOptions) (uint32, error) {
	if f.readOnly {
		return 0, ErrReadOnly
	}

	blockID, err := f.blockManager.WriteBlock(data, options)
	if err != nil {
		logger.Error("写入数据块失败", "error", err)
		return 0, err
	}

	f.isDirty = true
	f.header.BlockSize += uint64(len(data))
	return blockID, nil
}

// ReadBlock 读取数据块
func (f *FragmentaImpl) ReadBlock(blockID uint32) ([]byte, error) {
	return f.blockManager.ReadBlock(blockID)
}

// WriteFromReader 从Reader写入
func (f *FragmentaImpl) WriteFromReader(reader io.Reader, options *BlockOptions) error {
	if f.readOnly {
		return ErrReadOnly
	}

	// 读取所有数据到内存
	data, err := io.ReadAll(reader)
	if err != nil {
		logger.Error("读取数据失败", "error", err)
		return err
	}

	// 写入数据块
	_, err = f.WriteBlock(data, options)
	if err != nil {
		logger.Error("写入数据块失败", "error", err)
		return err
	}

	return nil
}

// ReadToWriter 读取到Writer
func (f *FragmentaImpl) ReadToWriter(writer io.Writer) error {
	// 读取所有块并写入writer
	// 简化实现：假设只有一个块，ID为1
	data, err := f.ReadBlock(1)
	if err != nil {
		logger.Error("读取数据失败", "error", err)
		return err
	}

	_, err = writer.Write(data)
	if err != nil {
		logger.Error("写入数据失败", "error", err)
		return err
	}

	return nil
}

// QueryByTag 通过标签查询
func (f *FragmentaImpl) QueryByTag(tag uint16, value []byte) ([]interface{}, error) {
	// 简单实现：通过元数据查询
	query := &MetadataQuery{
		Conditions: []MetadataCondition{
			{
				Tag:      tag,
				Operator: OpEquals,
				Value:    value,
			},
		},
		Operator: LogicAnd,
		Limit:    100,
	}

	result, err := f.QueryMetadata(query)
	if err != nil {
		logger.Error("查询元数据失败", "error", err)
		return nil, err
	}

	// 转换结果
	entries := make([]interface{}, len(result.Entries))
	for i, entry := range result.Entries {
		entries[i] = entry
	}

	return entries, nil
}

// QueryMetadata 复杂元数据查询
func (f *FragmentaImpl) QueryMetadata(query *MetadataQuery) (*QueryResult, error) {
	// 直接调用元数据管理器的查询功能
	return f.metadataManager.QueryMetadata(query)
}

// VerifyIndices 验证索引
func (f *FragmentaImpl) VerifyIndices() (*IndexStatus, error) {
	// 暂时返回一个基本的状态
	return &IndexStatus{
		TotalEntries:    0,
		ValidEntries:    0,
		InvalidEntries:  0,
		LastVerified:    time.Now(),
		RebuildRequired: false,
		IntegrityScore:  1.0,
		IndexLoadTime:   0,
		DeferredUpdates: 0,
	}, nil
}

// RebuildIndices 重建索引
func (f *FragmentaImpl) RebuildIndices() error {
	// 暂时返回nil，后续实现
	return nil
}

// StartQueryService 启动查询服务
func (f *FragmentaImpl) StartQueryService() error {
	// 暂时返回nil，后续实现
	return nil
}

// ConvertToDirectoryMode 转换为目录模式
func (f *FragmentaImpl) ConvertToDirectoryMode() error {
	if f.readOnly {
		return ErrReadOnly
	}

	// 只有容器模式才能转换为目录模式
	if f.header.StorageMode != ContainerMode {
		return ErrInvalidOperation
	}

	// 暂时返回nil，后续实现
	return nil
}

// ConvertToContainerMode 转换为容器模式
func (f *FragmentaImpl) ConvertToContainerMode() error {
	if f.readOnly {
		return ErrReadOnly
	}

	// 只有目录模式才能转换为容器模式
	if f.header.StorageMode != DirectoryMode {
		return ErrInvalidOperation
	}

	// 暂时返回nil，后续实现
	return nil
}

// OptimizeStorage 优化存储
func (f *FragmentaImpl) OptimizeStorage() error {
	if f.readOnly {
		return ErrReadOnly
	}

	// 暂时返回nil，后续实现
	return nil
}

// 内部辅助方法
func (f *FragmentaImpl) initializeHeader() {
	f.header = FragmentaHeader{
		Magic:          MagicNumber,
		Version:        CurrentVersion,
		Flags:          0,
		Timestamp:      time.Now().UnixNano(),
		LastModified:   time.Now().UnixNano(),
		StorageMode:    ContainerMode,
		Reserved1:      0,
		Reserved2:      0,
		MetadataOffset: 256, // 紧跟在头部之后
		MetadataSize:   0,
		BlockOffset:    0,
		BlockSize:      0,
		IndexOffset:    0,
		IndexSize:      0,
		TotalSize:      256, // 初始只有头部
	}
}

func (f *FragmentaImpl) writeHeader() error {
	// 定位到文件开头
	_, err := f.file.Seek(0, io.SeekStart)
	if err != nil {
		logger.Error("定位到文件开头失败", "error", err)
		return err
	}

	// 写入魔数
	err = binary.Write(f.file, binary.BigEndian, f.header.Magic)
	if err != nil {
		logger.Error("写入魔数失败", "error", err)
		return err
	}

	// 写入版本
	err = binary.Write(f.file, binary.BigEndian, f.header.Version)
	if err != nil {
		logger.Error("写入版本失败", "error", err)
		return err
	}

	// 写入标志
	err = binary.Write(f.file, binary.BigEndian, f.header.Flags)
	if err != nil {
		logger.Error("写入标志失败", "error", err)
		return err
	}

	// 写入时间戳
	err = binary.Write(f.file, binary.BigEndian, f.header.Timestamp)
	if err != nil {
		logger.Error("写入时间戳失败", "error", err)
		return err
	}

	// 写入最后修改时间
	err = binary.Write(f.file, binary.BigEndian, f.header.LastModified)
	if err != nil {
		logger.Error("写入最后修改时间失败", "error", err)
		return err
	}

	// 写入存储模式
	err = binary.Write(f.file, binary.BigEndian, f.header.StorageMode)
	if err != nil {
		logger.Error("写入存储模式失败", "error", err)
		return err
	}

	// 写入保留字段
	err = binary.Write(f.file, binary.BigEndian, f.header.Reserved1)
	if err != nil {
		logger.Error("写入保留字段失败", "error", err)
		return err
	}

	err = binary.Write(f.file, binary.BigEndian, f.header.Reserved2)
	if err != nil {
		logger.Error("写入保留字段失败", "error", err)
		return err
	}

	// 写入各区域偏移和大小
	err = binary.Write(f.file, binary.BigEndian, f.header.MetadataOffset)
	if err != nil {
		logger.Error("写入元数据偏移失败", "error", err)
		return err
	}

	err = binary.Write(f.file, binary.BigEndian, f.header.MetadataSize)
	if err != nil {
		logger.Error("写入元数据大小失败", "error", err)
		return err
	}

	err = binary.Write(f.file, binary.BigEndian, f.header.BlockOffset)
	if err != nil {
		logger.Error("写入块偏移失败", "error", err)
		return err
	}

	err = binary.Write(f.file, binary.BigEndian, f.header.BlockSize)
	if err != nil {
		logger.Error("写入块大小失败", "error", err)
		return err
	}

	err = binary.Write(f.file, binary.BigEndian, f.header.IndexOffset)
	if err != nil {
		logger.Error("写入索引偏移失败", "error", err)
		return err
	}

	err = binary.Write(f.file, binary.BigEndian, f.header.IndexSize)
	if err != nil {
		logger.Error("写入索引大小失败", "error", err)
		return err
	}

	err = binary.Write(f.file, binary.BigEndian, f.header.TotalSize)
	if err != nil {
		logger.Error("写入总大小失败", "error", err)
		return err
	}

	// 写入用户定义ID
	_, err = f.file.Write(f.header.UserDefinedID[:])
	if err != nil {
		logger.Error("写入用户定义ID失败", "error", err)
		return err
	}

	// 写入校验和
	_, err = f.file.Write(f.header.CheckSum[:])
	if err != nil {
		logger.Error("写入校验和失败", "error", err)
		return err
	}

	return nil
}

func (f *FragmentaImpl) readHeader() error {
	// 定位到文件开头
	_, err := f.file.Seek(0, io.SeekStart)
	if err != nil {
		logger.Error("定位到文件开头失败", "error", err)
		return err
	}

	// 读取魔数
	err = binary.Read(f.file, binary.BigEndian, &f.header.Magic)
	if err != nil {
		logger.Error("读取魔数失败", "error", err)
		return err
	}

	// 验证魔数
	if f.header.Magic != MagicNumber {
		logger.Error("验证魔数失败", "error", err)
		return ErrInvalidFragmenta
	}

	// 读取版本
	err = binary.Read(f.file, binary.BigEndian, &f.header.Version)
	if err != nil {
		logger.Error("读取版本失败", "error", err)
		return err
	}

	// 验证版本
	if f.header.Version < MinSupportedVersion || f.header.Version > CurrentVersion {
		logger.Error("验证版本失败", "error", err)
		return ErrUnsupportedVersion
	}

	// 读取标志
	err = binary.Read(f.file, binary.BigEndian, &f.header.Flags)
	if err != nil {
		logger.Error("读取标志失败", "error", err)
		return err
	}

	// 读取时间戳
	err = binary.Read(f.file, binary.BigEndian, &f.header.Timestamp)
	if err != nil {
		logger.Error("读取时间戳失败", "error", err)
		return err
	}

	// 读取最后修改时间
	err = binary.Read(f.file, binary.BigEndian, &f.header.LastModified)
	if err != nil {
		logger.Error("读取最后修改时间失败", "error", err)
		return err
	}

	// 读取存储模式
	err = binary.Read(f.file, binary.BigEndian, &f.header.StorageMode)
	if err != nil {
		logger.Error("读取存储模式失败", "error", err)
		return err
	}

	// 读取保留字段
	err = binary.Read(f.file, binary.BigEndian, &f.header.Reserved1)
	if err != nil {
		logger.Error("读取保留字段失败", "error", err)
		return err
	}

	err = binary.Read(f.file, binary.BigEndian, &f.header.Reserved2)
	if err != nil {
		logger.Error("读取保留字段失败", "error", err)
		return err
	}

	// 读取各区域偏移和大小
	err = binary.Read(f.file, binary.BigEndian, &f.header.MetadataOffset)
	if err != nil {
		logger.Error("读取元数据偏移失败", "error", err)
		return err
	}

	err = binary.Read(f.file, binary.BigEndian, &f.header.MetadataSize)
	if err != nil {
		logger.Error("读取元数据大小失败", "error", err)
		return err
	}

	err = binary.Read(f.file, binary.BigEndian, &f.header.BlockOffset)
	if err != nil {
		logger.Error("读取块偏移失败", "error", err)
		return err
	}

	err = binary.Read(f.file, binary.BigEndian, &f.header.BlockSize)
	if err != nil {
		logger.Error("读取块大小失败", "error", err)
		return err
	}

	err = binary.Read(f.file, binary.BigEndian, &f.header.IndexOffset)
	if err != nil {
		logger.Error("读取索引偏移失败", "error", err)
		return err
	}

	err = binary.Read(f.file, binary.BigEndian, &f.header.IndexSize)
	if err != nil {
		logger.Error("读取索引大小失败", "error", err)
		return err
	}

	err = binary.Read(f.file, binary.BigEndian, &f.header.TotalSize)
	if err != nil {
		logger.Error("读取总大小失败", "error", err)
		return err
	}

	// 读取用户定义ID
	_, err = f.file.Read(f.header.UserDefinedID[:])
	if err != nil {
		logger.Error("读取用户定义ID失败", "error", err)
		return err
	}

	// 读取校验和
	_, err = f.file.Read(f.header.CheckSum[:])
	if err != nil {
		logger.Error("读取校验和失败", "error", err)
		return err
	}

	return nil
}

func (f *FragmentaImpl) validateHeader() error {
	if f.header.Magic != MagicNumber {
		return ErrInvalidFragmenta
	}

	if f.header.Version > CurrentVersion {
		return ErrUnsupportedVersion
	}

	return nil
}

// initializeComponents 初始化组件
func (f *FragmentaImpl) initializeComponents() error {
	// 初始化元数据管理器
	f.metadataManager = NewMetadataManager(&f.header, f.file)

	// 初始化块管理器
	f.blockManager = NewBlockManager(f.file, &f.header)

	// 设置初始元数据
	if f.isNew {
		f.metadataManager.SetMetadata(TagCreateTime, EncodeInt64(time.Now().UnixNano()))
		f.metadataManager.SetMetadata(TagVersion, EncodeInt64(int64(CurrentVersion)))
		f.metadataManager.SetMetadata(TagFragmentaType, []byte("FragDB"))
	}

	return nil
}

// flushMetadata 刷新元数据
func (f *FragmentaImpl) flushMetadata() error {
	// 暂时只调用元数据管理器的刷新方法
	return f.metadataManager.Flush()
}

// 工厂方法实现

// NewFragmenta 创建新的格式文件
func NewFragmenta(path string, options *FragmentaOptions) (Fragmenta, error) {
	if options == nil {
		options = &FragmentaOptions{
			StorageMode:       ContainerMode,
			BlockSize:         DefaultBlockSize,
			IndexUpdateMode:   IndexUpdateRealtime,
			MaxIndexCacheSize: DefaultIndexCacheSize,
		}
	}

	// 创建文件
	file, err := os.Create(path)
	if err != nil {
		logger.Error("创建文件失败", "error", err)
		return nil, err
	}

	// 创建FragmentaImpl实例
	fragmenta := &FragmentaImpl{
		path:          path,
		file:          file,
		isNew:         true,
		isDirty:       true,
		isOpen:        true,
		readOnly:      false,
		metadataCache: make(map[uint16][]byte),
		blockCache:    make(map[uint32][]byte),
		lastModified:  time.Now(),
	}

	// 初始化头部
	fragmenta.initializeHeader()

	// 设置存储模式
	fragmenta.header.StorageMode = options.StorageMode

	// 写入头部
	err = fragmenta.writeHeader()
	if err != nil {
		file.Close()
		os.Remove(path)
		logger.Error("写入头部失败", "error", err)
		return nil, err
	}

	// 初始化组件
	err = fragmenta.initializeComponents()
	if err != nil {
		file.Close()
		os.Remove(path)
		logger.Error("初始化组件失败", "error", err)
		return nil, err
	}

	return fragmenta, nil
}

// NewFragmentaFromExisting 打开现有格式文件
func NewFragmentaFromExisting(path string) (Fragmenta, error) {
	// 打开文件
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		// 尝试以只读方式打开
		file, err = os.Open(path)
		if err != nil {
			logger.Error("打开文件失败", "error", err)
			return nil, err
		}
	}

	// 创建FragmentaImpl实例
	fragmenta := &FragmentaImpl{
		path:          path,
		file:          file,
		isNew:         false,
		isDirty:       false,
		isOpen:        true,
		readOnly:      false,
		metadataCache: make(map[uint16][]byte),
		blockCache:    make(map[uint32][]byte),
	}

	// 读取头部
	err = fragmenta.readHeader()
	if err != nil {
		file.Close()
		logger.Error("读取头部失败", "error", err)
		return nil, err
	}

	// 验证头部
	err = fragmenta.validateHeader()
	if err != nil {
		file.Close()
		logger.Error("验证头部失败", "error", err)
		return nil, err
	}

	// 检查是否只读
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		logger.Error("获取文件信息失败", "error", err)
		return nil, err
	}

	if fileInfo.Mode().Perm()&0200 == 0 {
		fragmenta.readOnly = true
	}

	// 初始化组件
	err = fragmenta.initializeComponents()
	if err != nil {
		file.Close()
		logger.Error("初始化组件失败", "error", err)
		return nil, err
	}

	// 记录最后修改时间
	fragmenta.lastModified = time.Unix(0, fragmenta.header.LastModified)

	return fragmenta, nil
}

// NewStorage 初始化存储
func NewStorage(rootPath string, options *StorageOptions) (Fragmenta, error) {
	if options == nil {
		options = &StorageOptions{
			DefaultMode:          ContainerMode,
			AutoConvertThreshold: AutoConvertThreshold,
			BlockSize:            DefaultBlockSize,
			InlineThreshold:      InlineBlockThreshold,
			DedupEnabled:         true,
			CacheSize:            DefaultIndexCacheSize,
			CachePolicy:          "LRU",
		}
	}

	// 检查是否存在现有存储
	_, err := os.Stat(rootPath)
	if os.IsNotExist(err) {
		// 创建新存储
		fragmentaOptions := &FragmentaOptions{
			StorageMode: options.DefaultMode,
			BlockSize:   options.BlockSize,
		}
		return NewFragmenta(rootPath, fragmentaOptions)
	}

	// 打开现有存储
	return NewFragmentaFromExisting(rootPath)
}
