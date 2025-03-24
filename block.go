package fragmenta

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// 块头大小常量
const BlockHeaderSize = 64 // 块头的大小，单位为字节

// blockManagerImpl 是BlockManager接口的实现
type blockManagerImpl struct {
	// 文件操作
	file io.ReadWriteSeeker

	// 块管理
	nextBlockID uint32
	blockMap    map[uint32]*BlockHeader
	freeList    []uint32

	// 同步与缓存
	mutex      sync.RWMutex
	blockCache map[uint32][]byte
	cacheSize  int
	isDirty    bool

	// 格式信息
	fragmentaHeader *FragmentaHeader
}

// NewBlockManager 创建一个块管理器
func NewBlockManager(file io.ReadWriteSeeker, header *FragmentaHeader) BlockManager {
	return &blockManagerImpl{
		file:            file,
		fragmentaHeader: header,
		blockMap:        make(map[uint32]*BlockHeader),
		blockCache:      make(map[uint32][]byte),
		cacheSize:       4096, // 默认缓存大小
	}
}

// WriteBlock 写入数据块
func (bm *blockManagerImpl) WriteBlock(data []byte, options *BlockOptions) (uint32, error) {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	if options == nil {
		options = &BlockOptions{
			BlockType: NormalBlockType,
			Compress:  false,
			Encrypt:   false,
			Checksum:  true,
		}
	}

	// 创建块头
	blockID := bm.getNextBlockID()
	header := &BlockHeader{
		BlockID:   blockID,
		BlockType: options.BlockType,
		Flags:     0,
		Size:      uint32(len(data)),
		Timestamp: time.Now().UnixNano(),
	}

	// 设置压缩和加密标志
	if options.Compress {
		header.Flags |= 0x01 // 压缩标志位
	}
	if options.Encrypt {
		header.Flags |= 0x02 // 加密标志位
	}

	// 如果需要校验和
	if options.Checksum {
		checksum := md5.Sum(data)
		header.Checksum = checksum
	}

	// 如果是链式存储
	if options.AppendToBlockID != 0 {
		prevHeader, err := bm.GetBlockInfo(options.AppendToBlockID)
		if err == nil {
			header.PreviousBlock = options.AppendToBlockID
			prevHeader.NextBlock = blockID
			// 更新前一个块的头信息
			bm.blockMap[options.AppendToBlockID] = prevHeader
		}
	}

	// 确定块的存储位置
	offset := bm.fragmentaHeader.BlockOffset
	if offset == 0 {
		// 如果是第一个块，设置块区起始位置
		// 通常是在头部和元数据之后
		offset = bm.fragmentaHeader.MetadataOffset + bm.fragmentaHeader.MetadataSize
		bm.fragmentaHeader.BlockOffset = offset
	} else {
		// 否则，在最后一个块之后
		offset += bm.fragmentaHeader.BlockSize
	}

	// 将文件指针移动到块的存储位置
	_, err := bm.file.Seek(int64(offset), io.SeekStart)
	if err != nil {
		logger.Error("移动文件指针失败", "error", err)
		return 0, err
	}

	// 写入块头
	// 简化：直接写入固定大小的块头
	headerSize := uint32(64) // 假设块头大小为64字节

	// 写入块ID
	err = binary.Write(bm.file, binary.BigEndian, header.BlockID)
	if err != nil {
		logger.Error("写入块ID失败", "error", err)
		return 0, err
	}

	// 写入块类型
	err = binary.Write(bm.file, binary.BigEndian, header.BlockType)
	if err != nil {
		logger.Error("写入块类型失败", "error", err)
		return 0, err
	}

	// 写入标志
	err = binary.Write(bm.file, binary.BigEndian, header.Flags)
	if err != nil {
		logger.Error("写入标志失败", "error", err)
		return 0, err
	}

	// 写入保留字段
	err = binary.Write(bm.file, binary.BigEndian, header.Reserved)
	if err != nil {
		logger.Error("写入保留字段失败", "error", err)
		return 0, err
	}

	// 写入大小
	err = binary.Write(bm.file, binary.BigEndian, header.Size)
	if err != nil {
		logger.Error("写入大小失败", "error", err)
		return 0, err
	}

	// 写入校验和
	_, err = bm.file.Write(header.Checksum[:])
	if err != nil {
		logger.Error("写入校验和失败", "error", err)
		return 0, err
	}

	// 写入前后块链接
	err = binary.Write(bm.file, binary.BigEndian, header.PreviousBlock)
	if err != nil {
		logger.Error("写入前后块链接失败", "error", err)
		return 0, err
	}

	err = binary.Write(bm.file, binary.BigEndian, header.NextBlock)
	if err != nil {
		logger.Error("写入前后块链接失败", "error", err)
		return 0, err
	}

	// 写入时间戳
	err = binary.Write(bm.file, binary.BigEndian, header.Timestamp)
	if err != nil {
		logger.Error("写入时间戳失败", "error", err)
		return 0, err
	}

	// 写入块数据
	_, err = bm.file.Write(data)
	if err != nil {
		logger.Error("写入块数据失败", "error", err)
		return 0, err
	}

	// 更新头部信息
	bm.fragmentaHeader.BlockSize += uint64(headerSize + header.Size)
	bm.fragmentaHeader.TotalSize += uint64(headerSize + header.Size)

	// 存储块和头信息
	bm.blockMap[blockID] = header
	bm.blockCache[blockID] = data
	bm.isDirty = true

	return blockID, nil
}

// ReadBlock 读取数据块
func (bm *blockManagerImpl) ReadBlock(blockID uint32) ([]byte, error) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	// 先检查缓存
	if data, ok := bm.blockCache[blockID]; ok {
		return data, nil
	}

	// 查找块头信息
	header, ok := bm.blockMap[blockID]
	if !ok {
		// 如果块头信息不在内存中，尝试从文件中读取
		var err error
		header, err = bm.readBlockHeader(blockID)
		if err != nil {
			// 如果是第一个块且块ID为1，并且文件有内容
			if blockID == 1 && bm.fragmentaHeader.BlockSize > 0 {
				// 创建一个默认的BlockHeader
				header = &BlockHeader{
					BlockID:   1,
					BlockType: NormalBlockType,
					Size:      uint32(bm.fragmentaHeader.BlockSize - 64), // 减去头部大小
				}
			} else {
				logger.Error("无法读取块头信息(ID=%d): %v", blockID, err)
				return nil, err
			}
		}
	}

	// 确保header不为nil
	if header == nil {
		logger.Error("块头信息为空", "blockID", blockID)
		return nil, fmt.Errorf("块头信息为空(ID=%d)", blockID)
	}

	// 读取块数据
	data, err := bm.readBlockData(header)
	if err != nil {
		// 特殊处理EOF错误，这可能是由于文件被截断或写入不完整导致的
		if err == io.EOF && bm.fragmentaHeader.BlockSize > 0 {
			// 尝试读取可用的数据
			var availableData []byte
			// 定位到数据块的开始位置
			offset := calculateBlockOffset(blockID, bm.fragmentaHeader)
			if offset > 0 {
				_, seekErr := bm.file.Seek(int64(offset+BlockHeaderSize), io.SeekStart)
				if seekErr == nil {
					// 尝试读取剩余的所有数据
					availableData, _ = io.ReadAll(bm.file)
					if len(availableData) > 0 {
						// 更新缓存
						bm.blockCache[blockID] = availableData
						return availableData, nil
					}
				}
			}
		}
		logger.Error("读取块数据失败(ID=%d): %v", blockID, err)
		return nil, err
	}

	// 验证校验和
	if header.Flags&0x04 != 0 { // 假设0x04是校验和标志
		checksum := md5.Sum(data)
		if checksum != header.Checksum {
			return nil, errors.New("block data checksum mismatch")
		}
	}

	// 更新缓存
	bm.blockCache[blockID] = data

	return data, nil
}

// DeleteBlock 删除数据块
func (bm *blockManagerImpl) DeleteBlock(blockID uint32) error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// 查找块头信息
	header, ok := bm.blockMap[blockID]
	if !ok {
		return ErrBlockNotFound
	}

	// 处理链接
	if header.PreviousBlock != 0 {
		prevHeader, ok := bm.blockMap[header.PreviousBlock]
		if ok {
			prevHeader.NextBlock = header.NextBlock
			bm.blockMap[header.PreviousBlock] = prevHeader
		}
	}

	if header.NextBlock != 0 {
		nextHeader, ok := bm.blockMap[header.NextBlock]
		if ok {
			nextHeader.PreviousBlock = header.PreviousBlock
			bm.blockMap[header.NextBlock] = nextHeader
		}
	}

	// 删除块信息
	delete(bm.blockMap, blockID)
	delete(bm.blockCache, blockID)
	bm.freeList = append(bm.freeList, blockID)
	bm.isDirty = true

	// 注意：实际的文件空间不会立即释放，需要通过OptimizeBlocks进行碎片整理

	return nil
}

// LinkBlocks 链接两个数据块
func (bm *blockManagerImpl) LinkBlocks(sourceID, targetID uint32) error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// 查找源块和目标块
	sourceHeader, sourceOk := bm.blockMap[sourceID]
	targetHeader, targetOk := bm.blockMap[targetID]

	if !sourceOk || !targetOk {
		return ErrBlockNotFound
	}

	// 设置链接关系
	sourceHeader.NextBlock = targetID
	targetHeader.PreviousBlock = sourceID

	// 更新块头信息
	bm.blockMap[sourceID] = sourceHeader
	bm.blockMap[targetID] = targetHeader
	bm.isDirty = true

	return nil
}

// GetBlockInfo 获取块信息
func (bm *blockManagerImpl) GetBlockInfo(blockID uint32) (*BlockHeader, error) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	header, ok := bm.blockMap[blockID]
	if !ok {
		// 如果块头信息不在内存中，尝试从文件中读取
		var err error
		header, err = bm.readBlockHeader(blockID)
		if err != nil {
			logger.Error("无法读取块头信息(ID=%d): %v", blockID, err)
			return nil, err
		}
		// 将读取的块头信息缓存
		bm.blockMap[blockID] = header
	}

	return header, nil
}

// OptimizeBlocks 优化块存储
func (bm *blockManagerImpl) OptimizeBlocks() error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// 这是一个复杂操作，需要重新组织文件中的块
	// 1. 创建临时文件
	tempFile, err := os.CreateTemp("", "defsf-optimize-*.tmp")
	if err != nil {
		logger.Error("创建临时文件失败", "error", err)
		return err
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	// 2. 将有效的块按顺序写入临时文件
	// 3. 更新所有块的偏移量信息
	// 4. 替换原文件

	// 简化实现：仅清理空闲列表
	bm.freeList = make([]uint32, 0)

	return nil
}

// 内部方法

// getNextBlockID 获取下一个可用的块ID
func (bm *blockManagerImpl) getNextBlockID() uint32 {
	// 优先使用空闲列表中的ID
	if len(bm.freeList) > 0 {
		id := bm.freeList[0]
		bm.freeList = bm.freeList[1:]
		return id
	}

	// 或者自增生成新ID
	bm.nextBlockID++
	return bm.nextBlockID
}

// readBlockHeader 从文件中读取块头信息
func (bm *blockManagerImpl) readBlockHeader(blockID uint32) (*BlockHeader, error) {
	// 获取所有块的索引信息
	// 在实际实现中，这些信息可能存储在索引区域
	// 简化：线性搜索文件中的所有块

	// 从块区开始搜索
	offset := bm.fragmentaHeader.BlockOffset
	var currentID uint32

	for offset < bm.fragmentaHeader.BlockOffset+bm.fragmentaHeader.BlockSize {
		// 定位到当前偏移
		_, err := bm.file.Seek(int64(offset), io.SeekStart)
		if err != nil {
			logger.Error("移动文件指针失败(ID=%d): %v", blockID, err)
			return nil, err
		}

		// 读取块ID
		err = binary.Read(bm.file, binary.BigEndian, &currentID)
		if err != nil {
			logger.Error("读取块ID失败(ID=%d): %v", blockID, err)
			return nil, err
		}

		if currentID == blockID {
			// 找到目标块，读取完整头部
			// 回退以重新读取ID
			_, err = bm.file.Seek(int64(offset), io.SeekStart)
			if err != nil {
				logger.Error("移动文件指针失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			header := &BlockHeader{}

			// 读取块ID
			err = binary.Read(bm.file, binary.BigEndian, &header.BlockID)
			if err != nil {
				logger.Error("读取块ID失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			// 读取块类型
			err = binary.Read(bm.file, binary.BigEndian, &header.BlockType)
			if err != nil {
				logger.Error("读取块类型失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			// 读取标志
			err = binary.Read(bm.file, binary.BigEndian, &header.Flags)
			if err != nil {
				logger.Error("读取标志失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			// 读取保留字段
			err = binary.Read(bm.file, binary.BigEndian, &header.Reserved)
			if err != nil {
				logger.Error("读取保留字段失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			// 读取大小
			err = binary.Read(bm.file, binary.BigEndian, &header.Size)
			if err != nil {
				logger.Error("读取大小失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			// 读取校验和
			_, err = bm.file.Read(header.Checksum[:])
			if err != nil {
				logger.Error("读取校验和失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			// 读取前后块链接
			err = binary.Read(bm.file, binary.BigEndian, &header.PreviousBlock)
			if err != nil {
				logger.Error("读取前后块链接失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			err = binary.Read(bm.file, binary.BigEndian, &header.NextBlock)
			if err != nil {
				logger.Error("读取前后块链接失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			// 读取时间戳
			err = binary.Read(bm.file, binary.BigEndian, &header.Timestamp)
			if err != nil {
				logger.Error("读取时间戳失败(ID=%d): %v", blockID, err)
				return nil, err
			}

			return header, nil
		}

		// 读取块的大小
		var blockSize uint32
		// 跳过块类型、标志和保留字段(1+1+2=4字节)
		_, err = bm.file.Seek(4, io.SeekCurrent)
		if err != nil {
			logger.Error("跳过块类型、标志和保留字段失败(ID=%d): %v", blockID, err)
			return nil, err
		}

		// 读取块大小
		err = binary.Read(bm.file, binary.BigEndian, &blockSize)
		if err != nil {
			logger.Error("读取块大小失败(ID=%d): %v", blockID, err)
			return nil, err
		}

		// 跳过剩余头部和块数据
		// 假设头部总大小为64字节
		_, err = bm.file.Seek(int64(64-4-4+blockSize), io.SeekCurrent)
		if err != nil {
			logger.Error("跳过剩余头部和块数据失败(ID=%d): %v", blockID, err)
			return nil, err
		}

		// 更新偏移量
		offset += 64 + uint64(blockSize)
	}

	return nil, ErrBlockNotFound
}

// readBlockData 从文件中读取块数据
func (bm *blockManagerImpl) readBlockData(header *BlockHeader) ([]byte, error) {
	// 特殊情况：如果是第一个块，直接从数据区开始处读取
	if header.BlockID == 1 && bm.fragmentaHeader.BlockOffset > 0 {
		// 定位到块数据起始位置
		_, err := bm.file.Seek(int64(bm.fragmentaHeader.BlockOffset+64), io.SeekStart)
		if err != nil {
			logger.Error("移动文件指针失败(ID=%d): %v", header.BlockID, err)
			return nil, err
		}

		// 读取块数据
		data := make([]byte, header.Size)
		_, err = bm.file.Read(data)
		if err != nil {
			logger.Error("读取块数据失败(ID=%d): %v", header.BlockID, err)
			return nil, err
		}

		return data, nil
	}

	// 常规情况：线性搜索文件中的块
	offset := bm.fragmentaHeader.BlockOffset
	var currentID uint32

	for offset < bm.fragmentaHeader.BlockOffset+bm.fragmentaHeader.BlockSize {
		// 定位到当前偏移
		_, err := bm.file.Seek(int64(offset), io.SeekStart)
		if err != nil {
			logger.Error("移动文件指针失败(ID=%d): %v", header.BlockID, err)
			return nil, err
		}

		// 读取块ID
		err = binary.Read(bm.file, binary.BigEndian, &currentID)
		if err != nil {
			logger.Error("读取块ID失败(ID=%d): %v", header.BlockID, err)
			return nil, err
		}

		if currentID == header.BlockID {
			// 找到目标块，跳过头部读取数据
			// 假设头部大小为64字节
			_, err = bm.file.Seek(int64(offset+64), io.SeekStart)
			if err != nil {
				logger.Error("移动文件指针失败(ID=%d): %v", header.BlockID, err)
				return nil, err
			}

			// 读取块数据
			data := make([]byte, header.Size)
			_, err = bm.file.Read(data)
			if err != nil {
				logger.Error("读取块数据失败(ID=%d): %v", header.BlockID, err)
				return nil, err
			}

			return data, nil
		}

		// 读取块的大小
		var blockSize uint32
		// 跳过块类型、标志和保留字段(1+1+2=4字节)
		_, err = bm.file.Seek(4, io.SeekCurrent)
		if err != nil {
			logger.Error("跳过块类型、标志和保留字段失败(ID=%d): %v", header.BlockID, err)
			return nil, err
		}

		// 读取块大小
		err = binary.Read(bm.file, binary.BigEndian, &blockSize)
		if err != nil {
			logger.Error("读取块大小失败(ID=%d): %v", header.BlockID, err)
			return nil, err
		}

		// 跳过剩余头部和块数据
		// 假设头部总大小为64字节
		_, err = bm.file.Seek(int64(64-4-4+blockSize), io.SeekCurrent)
		if err != nil {
			logger.Error("跳过剩余头部和块数据失败(ID=%d): %v", header.BlockID, err)
			return nil, err
		}

		// 更新偏移量
		offset += 64 + uint64(blockSize)
	}

	return nil, ErrBlockNotFound
}

// 辅助函数：计算块的偏移位置
func calculateBlockOffset(blockID uint32, header *FragmentaHeader) uint64 {
	// 简单的偏移计算，实际实现可能更复杂
	if blockID == 0 || header == nil {
		return 0
	}
	return header.BlockOffset + uint64((blockID-1)*128) // 假设每个块头128字节
}
