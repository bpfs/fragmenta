package fragmenta

import (
	"encoding/binary"
	"io"
	"sync"
	"time"
)

// metadataManagerImpl 是MetadataManager接口的实现
type metadataManagerImpl struct {
	// 元数据存储
	metadata map[uint16][]byte

	// 索引相关
	tagIndices map[uint16][]uint32 // 标签到块ID的映射

	// 同步与状态
	mutex        sync.RWMutex
	isDirty      bool
	lastModified time.Time

	// 格式信息
	fragmentaHeader *FragmentaHeader

	// 文件操作
	file io.ReadWriteSeeker
}

// NewMetadataManager 创建一个元数据管理器
func NewMetadataManager(header *FragmentaHeader, file io.ReadWriteSeeker) MetadataManager {
	mgr := &metadataManagerImpl{
		metadata:        make(map[uint16][]byte),
		tagIndices:      make(map[uint16][]uint32),
		fragmentaHeader: header,
		lastModified:    time.Now(),
		file:            file,
	}

	// 如果文件不为nil，尝试加载元数据
	if file != nil {
		mgr.LoadMetadata()
	}

	return mgr
}

// SetFile 设置文件句柄
func (mm *metadataManagerImpl) SetFile(file io.ReadWriteSeeker) {
	mm.file = file
}

// LoadMetadata 从文件加载元数据
func (mm *metadataManagerImpl) LoadMetadata() error {
	if mm.file == nil {
		return ErrInvalidOperation
	}

	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	// 如果元数据区大小为0，说明没有元数据
	if mm.fragmentaHeader.MetadataSize == 0 {
		return nil
	}

	// 定位到元数据区
	_, err := mm.file.Seek(int64(mm.fragmentaHeader.MetadataOffset), io.SeekStart)
	if err != nil {
		logger.Error("定位到元数据区失败", "error", err)
		return err
	}

	// 读取元数据数量
	var count uint32
	err = binary.Read(mm.file, binary.BigEndian, &count)
	if err != nil {
		logger.Error("读取元数据数量失败", "error", err)
		return err
	}

	// 读取每个元数据项
	for i := uint32(0); i < count; i++ {
		var metaTag uint16
		var size uint16

		// 读取标签
		err = binary.Read(mm.file, binary.BigEndian, &metaTag)
		if err != nil {
			logger.Error("读取标签失败", "error", err)
			return err
		}

		// 读取大小
		err = binary.Read(mm.file, binary.BigEndian, &size)
		if err != nil {
			logger.Error("读取大小失败", "error", err)
			return err
		}

		// 读取元数据标志
		var flags uint8
		err = binary.Read(mm.file, binary.BigEndian, &flags)
		if err != nil {
			logger.Error("读取元数据标志失败", "error", err)
			return err
		}

		// 读取保留字段
		var reserved uint8
		err = binary.Read(mm.file, binary.BigEndian, &reserved)
		if err != nil {
			logger.Error("读取保留字段失败", "error", err)
			return err
		}

		// 读取元数据值
		metaData := make([]byte, size)
		_, err = mm.file.Read(metaData)
		if err != nil {
			logger.Error("读取元数据值失败", "error", err)
			return err
		}

		// 存储到内存
		mm.metadata[metaTag] = metaData
	}

	mm.isDirty = false
	return nil
}

// SetMetadata 设置元数据
func (mm *metadataManagerImpl) SetMetadata(tag uint16, data []byte) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.metadata[tag] = data
	mm.isDirty = true
	mm.lastModified = time.Now()

	// 如果是内置标签，需要特殊处理
	if tag == TagLastModified {
		// 更新最后修改时间
		mm.fragmentaHeader.LastModified = time.Now().UnixNano()
	} else if tag == TagCreateTime && mm.fragmentaHeader.Timestamp == 0 {
		// 如果是创建时间且头部时间戳未设置，则更新头部时间戳
		var timestamp int64
		if len(data) >= 8 {
			timestamp = DecodeInt64(data)
		} else {
			timestamp = time.Now().UnixNano()
		}
		mm.fragmentaHeader.Timestamp = timestamp
	}

	return nil
}

// GetMetadata 获取元数据
func (mm *metadataManagerImpl) GetMetadata(tag uint16) ([]byte, error) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	data, ok := mm.metadata[tag]
	if !ok {
		return nil, ErrMetadataNotFound
	}

	return data, nil
}

// DeleteMetadata 删除元数据
func (mm *metadataManagerImpl) DeleteMetadata(tag uint16) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	// 检查是否是受保护的元数据标签
	if tag == TagCreateTime || tag == TagVersion || tag == TagFragmentaType {
		return ErrProtectedMetadata
	}

	_, ok := mm.metadata[tag]
	if !ok {
		return ErrMetadataNotFound
	}

	delete(mm.metadata, tag)
	mm.isDirty = true
	mm.lastModified = time.Now()

	return nil
}

// ListMetadata 列出所有元数据
func (mm *metadataManagerImpl) ListMetadata() (map[uint16][]byte, error) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// 创建副本
	result := make(map[uint16][]byte)
	for tag, data := range mm.metadata {
		result[tag] = data
	}

	return result, nil
}

// BatchOperation 执行批量元数据操作
func (mm *metadataManagerImpl) BatchOperation(batch *BatchMetadataOperation) error {
	if batch == nil {
		return nil
	}

	// 如果需要原子执行，获取写锁
	if batch.AtomicExec {
		mm.mutex.Lock()
		defer mm.mutex.Unlock()
	}

	// 跟踪错误
	var lastError error

	// 执行操作
	for _, op := range batch.Operations {
		var err error

		switch op.Operation {
		case 0: // 设置
			if batch.AtomicExec {
				// 已经有锁，直接操作
				mm.metadata[op.Tag] = op.Value
				mm.isDirty = true
			} else {
				// 没有全局锁，调用方法获取锁
				err = mm.SetMetadata(op.Tag, op.Value)
			}
		case 1: // 删除
			if batch.AtomicExec {
				// 已经有锁，直接操作
				// 检查是否是受保护的元数据标签
				if op.Tag == TagCreateTime || op.Tag == TagVersion || op.Tag == TagFragmentaType {
					err = ErrProtectedMetadata
				} else {
					delete(mm.metadata, op.Tag)
					mm.isDirty = true
				}
			} else {
				// 没有全局锁，调用方法获取锁
				err = mm.DeleteMetadata(op.Tag)
			}
		case 2: // 附加
			if batch.AtomicExec {
				// 已经有锁，直接操作
				existing, ok := mm.metadata[op.Tag]
				if ok {
					mm.metadata[op.Tag] = append(existing, op.Value...)
				} else {
					mm.metadata[op.Tag] = op.Value
				}
				mm.isDirty = true
			} else {
				// 获取现有数据
				existing, gErr := mm.GetMetadata(op.Tag)
				if gErr != nil && gErr != ErrMetadataNotFound {
					err = gErr
				} else if gErr == ErrMetadataNotFound {
					// 不存在则创建
					err = mm.SetMetadata(op.Tag, op.Value)
				} else {
					// 附加数据
					newData := append(existing, op.Value...)
					err = mm.SetMetadata(op.Tag, newData)
				}
			}
		}

		// 错误处理
		if err != nil {
			lastError = err
			if batch.RollbackOnError {
				// 需要回滚，但简单实现暂不支持回滚
				logger.Error("批量操作回滚失败", "error", err)
				return err
			}
		}
	}

	// 如果有原子操作，更新状态
	if batch.AtomicExec {
		mm.lastModified = time.Now()
	}

	return lastError
}

// QueryMetadata 查询元数据
func (mm *metadataManagerImpl) QueryMetadata(query *MetadataQuery) (*QueryResult, error) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	if query == nil || len(query.Conditions) == 0 {
		return nil, ErrInvalidQuery
	}

	// 创建结果
	result := &QueryResult{
		Entries:     make([]ResultEntry, 0),
		TotalCount:  0,
		ReturnCount: 0,
		HasMore:     false,
		QueryTime:   0,
	}

	// 记录开始时间
	start := time.Now()

	// 简单实现：遍历所有元数据
	for tag, data := range mm.metadata {
		// 检查是否满足所有条件
		matches := true

		for _, condition := range query.Conditions {
			if !mm.matchCondition(tag, data, &condition) {
				matches = false
				break
			}
		}

		// 如果满足所有条件，添加到结果
		if matches {
			result.TotalCount++

			// 检查偏移和限制
			if result.TotalCount > uint32(query.Offset) &&
				(query.Limit == 0 || result.ReturnCount < query.Limit) {
				// 添加到结果
				entry := ResultEntry{
					MetadataID:   tag,
					MetadataData: data,
					ExtraData:    make(map[string][]byte),
				}

				result.Entries = append(result.Entries, entry)
				result.ReturnCount++
			}
		}
	}

	// 检查是否有更多结果
	result.HasMore = result.TotalCount > (uint32(query.Offset) + result.ReturnCount)

	// 计算查询时间（毫秒）
	result.QueryTime = uint32(time.Since(start).Milliseconds())

	return result, nil
}

// Flush 将元数据刷新到磁盘
func (mm *metadataManagerImpl) Flush() error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.isDirty || mm.file == nil {
		return nil
	}

	// 更新最后修改时间
	mm.lastModified = time.Now()

	// 将元数据更新到头部
	mm.fragmentaHeader.LastModified = mm.lastModified.UnixNano()

	// 计算元数据总大小
	var totalSize uint64 = 4 // 元数据数量占4字节
	for _, value := range mm.metadata {
		totalSize += 6 // 标签(2字节)+大小(2字节)+标志(1字节)+保留(1字节)
		totalSize += uint64(len(value))
	}

	// 更新元数据大小
	mm.fragmentaHeader.MetadataSize = totalSize

	// 定位到元数据区
	_, err := mm.file.Seek(int64(mm.fragmentaHeader.MetadataOffset), io.SeekStart)
	if err != nil {
		logger.Error("定位到元数据区失败", "error", err)
		return err
	}

	// 写入元数据数量
	count := uint32(len(mm.metadata))
	err = binary.Write(mm.file, binary.BigEndian, count)
	if err != nil {
		logger.Error("写入元数据数量失败", "error", err)
		return err
	}

	// 写入每个元数据项
	for metaTag, metaData := range mm.metadata {
		// 写入标签
		err = binary.Write(mm.file, binary.BigEndian, metaTag)
		if err != nil {
			logger.Error("写入标签失败", "error", err)
			return err
		}

		// 写入大小
		size := uint16(len(metaData))
		err = binary.Write(mm.file, binary.BigEndian, size)
		if err != nil {
			logger.Error("写入大小失败", "error", err)
			return err
		}

		// 写入标志
		var flags uint8 = 0
		err = binary.Write(mm.file, binary.BigEndian, flags)
		if err != nil {
			logger.Error("写入标志失败", "error", err)
			return err
		}

		// 写入保留字段
		var reserved uint8 = 0
		err = binary.Write(mm.file, binary.BigEndian, reserved)
		if err != nil {
			logger.Error("写入保留字段失败", "error", err)
			return err
		}

		// 写入数据
		_, err = mm.file.Write(metaData)
		if err != nil {
			logger.Error("写入数据失败", "error", err)
			return err
		}
	}

	// 重置标志
	mm.isDirty = false

	return nil
}

// 内部辅助方法

// matchCondition 检查元数据是否匹配条件
func (mm *metadataManagerImpl) matchCondition(tag uint16, data []byte, condition *MetadataCondition) bool {
	// 如果标签不匹配，直接返回false
	if tag != condition.Tag {
		return false
	}

	// 根据操作符检查值
	switch condition.Operator {
	case OpEquals:
		// 简单比较字节数组
		if len(data) != len(condition.Value) {
			return false
		}
		for i := 0; i < len(data); i++ {
			if data[i] != condition.Value[i] {
				return false
			}
		}
		return true

	case OpNotEquals:
		// 与Equals相反
		if len(data) != len(condition.Value) {
			return true
		}
		for i := 0; i < len(data); i++ {
			if data[i] != condition.Value[i] {
				return true
			}
		}
		return false

	case OpGreaterThan:
		// 比较数值
		if len(data) >= 8 && len(condition.Value) >= 8 {
			// 假设数据是int64编码的
			dataVal := DecodeInt64(data)
			condVal := DecodeInt64(condition.Value)
			return dataVal > condVal
		}
		// 简单比较第一个字节
		if len(data) > 0 && len(condition.Value) > 0 {
			return data[0] > condition.Value[0]
		}
		return false

	case OpLessThan:
		// 比较数值
		if len(data) >= 8 && len(condition.Value) >= 8 {
			// 假设数据是int64编码的
			dataVal := DecodeInt64(data)
			condVal := DecodeInt64(condition.Value)
			return dataVal < condVal
		}
		// 简单比较第一个字节
		if len(data) > 0 && len(condition.Value) > 0 {
			return data[0] < condition.Value[0]
		}
		return false

	case OpContains:
		// 检查子串
		if len(data) < len(condition.Value) {
			return false
		}
		for i := 0; i <= len(data)-len(condition.Value); i++ {
			matches := true
			for j := 0; j < len(condition.Value); j++ {
				if data[i+j] != condition.Value[j] {
					matches = false
					break
				}
			}
			if matches {
				return true
			}
		}
		return false

	default:
		return false
	}
}
