// package index 提供优化版索引功能实现
package index

import (
	"container/heap"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 优先级队列项
// 内部使用的UpdateTask定义，与外部接口区分
type updateTaskInternal struct {
	Tag       uint32
	ID        uint32
	Operation UpdateOperation
	Priority  int
	Index     int
	Timestamp time.Time
}

// 优化索引的操作类型常量与types.go保持一致
// const (
//	AddIndexOp    UpdateOperation = 0
//	RemoveIndexOp UpdateOperation = 1
// )

// OptimizedIndexManager 优化版索引管理器
type OptimizedIndexManager struct {
	// 基本配置
	config *IndexConfig

	// 索引元数据
	metadata IndexMetadata

	// 索引分片数据 - 外层map是分片ID，内层map是标签到ID列表的映射
	shards []map[uint32][]uint32

	// 内容索引 - 分片形式存储
	contentShards []map[string][]uint32

	// 前缀树索引 - 加速前缀查询
	prefixTrees map[uint32]*PrefixNode

	// 异步更新队列
	updateQueue priorityQueue
	queueMutex  sync.Mutex

	// 线程池相关
	workerPool   chan struct{}
	workerWg     sync.WaitGroup
	stopWorkers  chan struct{}
	updateTicker *time.Ticker

	// 批量操作缓冲区
	batchBuffer      map[uint32]map[UpdateOperation][]uint32
	batchBufferSize  int
	batchBufferMutex sync.Mutex

	// 状态信息
	statusMutex      sync.RWMutex
	lastUpdateTime   time.Time
	isUpdating       bool
	progress         int32
	indexedCount     int32
	pendingCount     int32
	activeWorkers    int32
	compressionRatio float64
	memoryUsage      int64
	lastError        string
	shardStatus      []ShardStatus

	// 分片级锁 - 避免全局锁竞争
	shardMutexes []sync.RWMutex
}

// 优先级队列
type priorityQueue []*updateTaskInternal

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].Priority < pq[j].Priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*updateTaskInternal)
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	*pq = old[0 : n-1]
	return item
}

// NewOptimizedIndexManager 创建优化版索引管理器
func NewOptimizedIndexManager(config *IndexConfig) (*OptimizedIndexManager, error) {
	if config == nil {
		config = &IndexConfig{
			IndexPath:      "",
			AutoSave:       true,
			AutoRebuild:    false,
			MaxCacheSize:   10 * 1024 * 1024, // 10MB
			AsyncUpdate:    true,
			MaxWorkers:     runtime.NumCPU(),
			NumShards:      16,
			UpdateInterval: 1000, // 默认1秒
			BatchThreshold: 1000, // 默认1000个操作
		}
	}

	// 确保分片数量至少为1
	if config.NumShards < 1 {
		config.NumShards = 1
	}

	// 创建管理器对象
	im := &OptimizedIndexManager{
		config:         config,
		shards:         make([]map[uint32][]uint32, config.NumShards),
		contentShards:  make([]map[string][]uint32, config.NumShards),
		prefixTrees:    make(map[uint32]*PrefixNode),
		updateQueue:    make(priorityQueue, 0),
		workerPool:     make(chan struct{}, config.MaxWorkers),
		stopWorkers:    make(chan struct{}),
		batchBuffer:    make(map[uint32]map[UpdateOperation][]uint32),
		lastUpdateTime: time.Now(),
		isUpdating:     false,
		progress:       0,
		indexedCount:   0,
		pendingCount:   0,
		activeWorkers:  0,
		shardMutexes:   make([]sync.RWMutex, config.NumShards),
		shardStatus:    make([]ShardStatus, config.NumShards),
		metadata: IndexMetadata{
			Version:    "1.0",
			CreatedAt:  time.Now(),
			ModifiedAt: time.Now(),
			ShardCount: config.NumShards,
		},
	}

	// 初始化堆
	heap.Init(&im.updateQueue)

	// 初始化分片
	for i := 0; i < config.NumShards; i++ {
		im.shards[i] = make(map[uint32][]uint32)
		im.contentShards[i] = make(map[string][]uint32)
		im.shardStatus[i] = ShardStatus{
			ShardID:    i,
			ItemCount:  0,
			Available:  true,
			ReadCount:  0,
			WriteCount: 0,
			LastAccess: time.Now(),
		}
	}

	// 如果索引文件存在，则加载
	if config.IndexPath != "" {
		_, err := os.Stat(config.IndexPath)
		if err == nil {
			err = im.LoadIndex(config.IndexPath)
			if err != nil {
				logger.Error("加载索引失败", "error", err)
				return nil, err
			}
		}
	}

	// 如果开启异步更新，启动工作线程
	if config.AsyncUpdate {
		im.startWorkers()
	}

	return im, nil
}

// updateMemoryUsage 更新内存使用统计
func (im *OptimizedIndexManager) updateMemoryUsage() {
	// 实际实现可能需要更复杂的计算
	im.memoryUsage = int64(len(im.shards) * 1024 * 1024) // 简化估算
}

// getShardID 获取分片ID
func (im *OptimizedIndexManager) getShardID(id uint32) int {
	return int(id % uint32(len(im.shards)))
}

// startWorkers 启动工作线程
func (im *OptimizedIndexManager) startWorkers() {
	// 停止现有线程
	if im.updateTicker != nil {
		close(im.stopWorkers)
		im.updateTicker.Stop()
		im.workerWg.Wait()
	}

	// 创建新的停止通道
	im.stopWorkers = make(chan struct{})

	// 创建批处理定时器，确保间隔至少为1ms
	interval := im.config.UpdateInterval
	if interval <= 0 {
		interval = 100 // 默认100ms
	}
	im.updateTicker = time.NewTicker(time.Duration(interval) * time.Millisecond)

	// 启动批处理线程
	im.workerWg.Add(1)
	go func() {
		defer im.workerWg.Done()
		for {
			select {
			case <-im.updateTicker.C:
				// 处理批量缓冲区
				im.processBatchBuffer()
				// 处理更新队列
				im.processUpdateQueue()
			case <-im.stopWorkers:
				logger.Info("工作线程停止")
				return
			}
		}
	}()

	logger.Info("启动工作线程成功")
}

// processBatchBuffer 处理批量缓冲区
func (im *OptimizedIndexManager) processBatchBuffer() {
	im.batchBufferMutex.Lock()
	defer im.batchBufferMutex.Unlock()

	if len(im.batchBuffer) == 0 {
		return
	}

	logger.Debug("处理批量缓冲区", "size", im.batchBufferSize)

	// 遍历所有标签
	for tag, operations := range im.batchBuffer {
		// 遍历所有操作类型
		for op, ids := range operations {
			if len(ids) == 0 {
				continue
			}

			var err error
			switch op {
			case OpAdd:
				err = im.batchAddIndicesInternal(tag, ids)
			case OpRemove:
				err = im.batchRemoveIndicesInternal(tag, ids)
			}

			if err != nil {
				logger.Error("批量处理失败", "tag", tag, "operation", op, "error", err)
				im.lastError = err.Error()
			}
		}
	}

	// 清空缓冲区
	im.batchBuffer = make(map[uint32]map[UpdateOperation][]uint32)
	im.batchBufferSize = 0
}

// processUpdateQueue 处理更新队列
func (im *OptimizedIndexManager) processUpdateQueue() {
	for im.updateQueue.Len() > 0 {
		// 达到最大工作线程数时等待
		if atomic.LoadInt32(&im.activeWorkers) >= int32(im.config.MaxWorkers) {
			break
		}

		// 获取优先级最高的任务
		im.queueMutex.Lock()
		if im.updateQueue.Len() == 0 {
			im.queueMutex.Unlock()
			break
		}
		task := heap.Pop(&im.updateQueue).(*updateTaskInternal)
		im.queueMutex.Unlock()

		// 更新计数
		atomic.AddInt32(&im.pendingCount, -1)
		atomic.AddInt32(&im.activeWorkers, 1)

		// 处理任务
		im.workerPool <- struct{}{}
		go func(t *updateTaskInternal) {
			var err error
			switch t.Operation {
			case OpAdd:
				err = im.addIndexInternal(t.Tag, t.ID)
			case OpRemove:
				err = im.removeIndexInternal(t.Tag, t.ID)
			}

			if err != nil {
				logger.Error("处理任务失败", "tag", t.Tag, "id", t.ID, "operation", t.Operation, "error", err)
				im.lastError = err.Error()
			}

			// 更新计数
			atomic.AddInt32(&im.activeWorkers, -1)
			<-im.workerPool
		}(task)
	}
}

// addToUpdateQueue 添加到更新队列
func (im *OptimizedIndexManager) addToUpdateQueue(op UpdateOperation, tag, id uint32, priority int) {
	im.queueMutex.Lock()
	defer im.queueMutex.Unlock()

	// 创建任务
	task := &updateTaskInternal{
		Tag:       tag,
		ID:        id,
		Operation: op,
		Priority:  priority,
		Timestamp: time.Now(),
	}

	// 添加到队列
	heap.Push(&im.updateQueue, task)

	// 更新计数
	atomic.AddInt32(&im.pendingCount, 1)
}

// addToBatchBuffer 添加到批量缓冲区
func (im *OptimizedIndexManager) addToBatchBuffer(op UpdateOperation, tag, id uint32) {
	im.batchBufferMutex.Lock()
	defer im.batchBufferMutex.Unlock()

	// 确保标签存在
	if _, ok := im.batchBuffer[tag]; !ok {
		im.batchBuffer[tag] = make(map[UpdateOperation][]uint32)
	}

	// 确保操作类型存在
	if _, ok := im.batchBuffer[tag][op]; !ok {
		im.batchBuffer[tag][op] = make([]uint32, 0)
	}

	// 添加到缓冲区
	im.batchBuffer[tag][op] = append(im.batchBuffer[tag][op], id)
	im.batchBufferSize++

	// 如果达到阈值，立即处理
	if im.batchBufferSize >= im.config.BatchThreshold {
		go im.processBatchBuffer()
	}
}

// 内部添加索引实现（直接操作，不经过异步队列或批处理）
func (im *OptimizedIndexManager) addIndexInternal(tag uint32, id uint32) error {
	// 确定分片ID
	shardID := im.getShardID(id)

	// 获取分片锁
	im.shardMutexes[shardID].Lock()
	defer im.shardMutexes[shardID].Unlock()

	// 更新分片访问时间
	im.shardStatus[shardID].LastAccess = time.Now()
	atomic.AddInt64(&im.shardStatus[shardID].WriteCount, 1)

	// 检查标签是否存在
	if _, ok := im.shards[shardID][tag]; !ok {
		im.shards[shardID][tag] = make([]uint32, 0)
	}

	// 检查ID是否已存在
	for _, existingID := range im.shards[shardID][tag] {
		if existingID == id {
			return nil // 已存在，无需添加
		}
	}

	// 添加索引
	im.shards[shardID][tag] = append(im.shards[shardID][tag], id)

	// 更新状态
	atomic.AddInt32(&im.indexedCount, 1)
	atomic.AddInt32(&im.shardStatus[shardID].ItemCount, 1)

	// 更新前缀树（如果启用）
	if im.config.EnablePrefixCompression {
		im.updatePrefixTree(tag, id, true)
	}

	return nil
}

// 内部移除索引实现（直接操作，不经过异步队列或批处理）
func (im *OptimizedIndexManager) removeIndexInternal(tag uint32, id uint32) error {
	// 确定分片ID
	shardID := im.getShardID(id)

	// 获取分片锁
	im.shardMutexes[shardID].Lock()
	defer im.shardMutexes[shardID].Unlock()

	// 更新分片访问时间
	im.shardStatus[shardID].LastAccess = time.Now()
	atomic.AddInt64(&im.shardStatus[shardID].WriteCount, 1)

	// 检查标签是否存在
	if _, ok := im.shards[shardID][tag]; !ok {
		return ErrIndexNotFound
	}

	// 查找并移除ID
	ids := im.shards[shardID][tag]
	found := false
	for i, existingID := range ids {
		if existingID == id {
			// 移除元素
			im.shards[shardID][tag] = append(ids[:i], ids[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return ErrIndexNotFound
	}

	// 更新状态
	atomic.AddInt32(&im.indexedCount, -1)
	atomic.AddInt32(&im.shardStatus[shardID].ItemCount, -1)

	// 更新前缀树（如果启用）
	if im.config.EnablePrefixCompression {
		im.updatePrefixTree(tag, id, false)
	}

	return nil
}

// 批量添加索引（内部实现）
func (im *OptimizedIndexManager) batchAddIndicesInternal(tag uint32, ids []uint32) error {
	// 按分片分组ID
	shardGroups := make(map[int][]uint32)
	for _, id := range ids {
		shardID := im.getShardID(id)
		if _, ok := shardGroups[shardID]; !ok {
			shardGroups[shardID] = make([]uint32, 0)
		}
		shardGroups[shardID] = append(shardGroups[shardID], id)
	}

	// 对每个分片进行批量添加
	for shardID, shardIDs := range shardGroups {
		// 获取分片锁
		im.shardMutexes[shardID].Lock()

		// 更新分片访问时间
		im.shardStatus[shardID].LastAccess = time.Now()
		atomic.AddInt64(&im.shardStatus[shardID].WriteCount, 1)

		// 检查标签是否存在
		if _, ok := im.shards[shardID][tag]; !ok {
			im.shards[shardID][tag] = make([]uint32, 0, len(shardIDs))
		}

		// 创建现有ID的映射，用于快速查找
		existingIDs := make(map[uint32]bool)
		for _, id := range im.shards[shardID][tag] {
			existingIDs[id] = true
		}

		// 添加不存在的ID
		addedCount := 0
		for _, id := range shardIDs {
			if !existingIDs[id] {
				im.shards[shardID][tag] = append(im.shards[shardID][tag], id)
				existingIDs[id] = true
				addedCount++

				// 更新前缀树（如果启用）
				if im.config.EnablePrefixCompression {
					im.updatePrefixTree(tag, id, true)
				}
			}
		}

		// 更新状态
		if addedCount > 0 {
			atomic.AddInt32(&im.indexedCount, int32(addedCount))
			atomic.AddInt32(&im.shardStatus[shardID].ItemCount, int32(addedCount))
		}

		im.shardMutexes[shardID].Unlock()
	}

	return nil
}

// 批量移除索引（内部实现）
func (im *OptimizedIndexManager) batchRemoveIndicesInternal(tag uint32, ids []uint32) error {
	// 按分片分组ID
	shardGroups := make(map[int][]uint32)
	for _, id := range ids {
		shardID := im.getShardID(id)
		if _, ok := shardGroups[shardID]; !ok {
			shardGroups[shardID] = make([]uint32, 0)
		}
		shardGroups[shardID] = append(shardGroups[shardID], id)
	}

	// 对每个分片进行批量移除
	for shardID, shardIDs := range shardGroups {
		// 获取分片锁
		im.shardMutexes[shardID].Lock()

		// 更新分片访问时间
		im.shardStatus[shardID].LastAccess = time.Now()
		atomic.AddInt64(&im.shardStatus[shardID].WriteCount, 1)

		// 检查标签是否存在
		if _, ok := im.shards[shardID][tag]; !ok {
			im.shardMutexes[shardID].Unlock()
			continue // 跳过不存在的标签
		}

		// 创建要移除的ID的映射，用于快速查找
		removeIDs := make(map[uint32]bool)
		for _, id := range shardIDs {
			removeIDs[id] = true
		}

		// 筛选出要保留的ID
		originalLength := len(im.shards[shardID][tag])
		newIDs := make([]uint32, 0, originalLength)
		for _, id := range im.shards[shardID][tag] {
			if !removeIDs[id] {
				newIDs = append(newIDs, id)
			} else {
				// 更新前缀树（如果启用）
				if im.config.EnablePrefixCompression {
					im.updatePrefixTree(tag, id, false)
				}
			}
		}

		// 更新分片中的ID列表
		im.shards[shardID][tag] = newIDs

		// 更新状态
		removedCount := originalLength - len(newIDs)
		if removedCount > 0 {
			atomic.AddInt32(&im.indexedCount, -int32(removedCount))
			atomic.AddInt32(&im.shardStatus[shardID].ItemCount, -int32(removedCount))
		}

		im.shardMutexes[shardID].Unlock()
	}

	return nil
}

// AddIndex 添加索引
func (im *OptimizedIndexManager) AddIndex(tag uint32, id uint32) error {
	if im.config.AsyncUpdate {
		// 异步模式：添加到批处理缓冲区
		im.addToBatchBuffer(OpAdd, tag, id)
		return nil
	} else {
		// 同步模式：直接添加
		return im.addIndexInternal(tag, id)
	}
}

// RemoveIndex 移除索引
func (im *OptimizedIndexManager) RemoveIndex(tag uint32, id uint32) error {
	if im.config.AsyncUpdate {
		// 异步模式：添加到批处理缓冲区
		im.addToBatchBuffer(OpRemove, tag, id)
		return nil
	} else {
		// 同步模式：直接移除
		return im.removeIndexInternal(tag, id)
	}
}

// AsyncAddIndex 异步添加索引
func (im *OptimizedIndexManager) AsyncAddIndex(tag uint32, id uint32) error {
	im.addToUpdateQueue(OpAdd, tag, id, 0) // 默认优先级为0（最高）
	return nil
}

// AsyncRemoveIndex 异步移除索引
func (im *OptimizedIndexManager) AsyncRemoveIndex(tag uint32, id uint32) error {
	im.addToUpdateQueue(OpRemove, tag, id, 0) // 默认优先级为0（最高）
	return nil
}

// BatchAddIndices 批量添加索引
func (im *OptimizedIndexManager) BatchAddIndices(tags []uint32, ids []uint32) error {
	if len(tags) != len(ids) {
		return fmt.Errorf("tags and ids length mismatch")
	}

	// 按标签分组ID
	tagGroups := make(map[uint32][]uint32)
	for i, tag := range tags {
		if _, ok := tagGroups[tag]; !ok {
			tagGroups[tag] = make([]uint32, 0)
		}
		tagGroups[tag] = append(tagGroups[tag], ids[i])
	}

	// 对每个标签进行批量添加
	for tag, tagIDs := range tagGroups {
		if im.config.AsyncUpdate {
			// 异步模式：添加到批处理缓冲区
			for _, id := range tagIDs {
				im.addToBatchBuffer(OpAdd, tag, id)
			}
		} else {
			// 同步模式：直接批量添加
			if err := im.batchAddIndicesInternal(tag, tagIDs); err != nil {
				return err
			}
		}
	}

	return nil
}

// BatchRemoveIndices 批量移除索引
func (im *OptimizedIndexManager) BatchRemoveIndices(tags []uint32, ids []uint32) error {
	if len(tags) != len(ids) {
		return fmt.Errorf("tags and ids length mismatch")
	}

	// 按标签分组ID
	tagGroups := make(map[uint32][]uint32)
	for i, tag := range tags {
		if _, ok := tagGroups[tag]; !ok {
			tagGroups[tag] = make([]uint32, 0)
		}
		tagGroups[tag] = append(tagGroups[tag], ids[i])
	}

	// 对每个标签进行批量移除
	for tag, tagIDs := range tagGroups {
		if im.config.AsyncUpdate {
			// 异步模式：添加到批处理缓冲区
			for _, id := range tagIDs {
				im.addToBatchBuffer(OpRemove, tag, id)
			}
		} else {
			// 同步模式：直接批量移除
			if err := im.batchRemoveIndicesInternal(tag, tagIDs); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetPendingTaskCount 获取待处理更新任务数
func (im *OptimizedIndexManager) GetPendingTaskCount() int {
	return int(atomic.LoadInt32(&im.pendingCount))
}

// 更新前缀树
func (im *OptimizedIndexManager) updatePrefixTree(tag uint32, id uint32, isAdd bool) {
	// 暂时只是占位，后续会实现
	// TODO: 实现前缀树更新逻辑
}

// SaveIndex 保存索引到文件
func (im *OptimizedIndexManager) SaveIndex(path string) error {
	// 获取状态锁
	im.statusMutex.RLock()
	defer im.statusMutex.RUnlock()

	// 准备要保存的数据
	type IndexData struct {
		Metadata       IndexMetadata         `json:"metadata"`
		Shards         []map[uint32][]uint32 `json:"shards"`
		ContentShards  []map[string][]uint32 `json:"content_shards"`
		LastUpdateTime time.Time             `json:"last_update_time"`
		Checksum       string                `json:"checksum"`
	}

	data := IndexData{
		Metadata:       im.metadata,
		Shards:         im.shards,
		ContentShards:  im.contentShards,
		LastUpdateTime: im.lastUpdateTime,
	}

	// 计算校验和
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(jsonData)
	data.Checksum = hex.EncodeToString(hash[:])

	// 序列化为缩进格式的JSON
	jsonData, err = json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(path, jsonData, 0644)
}

// LoadIndex 从文件加载索引
func (im *OptimizedIndexManager) LoadIndex(path string) error {
	// 获取状态锁
	im.statusMutex.Lock()
	defer im.statusMutex.Unlock()

	// 读取文件
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// 解析JSON数据
	type IndexData struct {
		Metadata       IndexMetadata         `json:"metadata"`
		Shards         []map[uint32][]uint32 `json:"shards"`
		ContentShards  []map[string][]uint32 `json:"content_shards"`
		LastUpdateTime time.Time             `json:"last_update_time"`
		Checksum       string                `json:"checksum"`
	}

	var data IndexData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	// 验证校验和
	checksum := data.Checksum
	data.Checksum = ""
	jsonWithoutChecksum, err := json.Marshal(data)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(jsonWithoutChecksum)
	calculatedChecksum := hex.EncodeToString(hash[:])

	if checksum != calculatedChecksum {
		return ErrIndexCorrupted
	}

	// 更新索引数据
	im.metadata = data.Metadata
	im.shards = data.Shards
	im.contentShards = data.ContentShards
	im.lastUpdateTime = data.LastUpdateTime

	// 更新统计信息
	var totalCount int32
	for shardID, tagMap := range im.shards {
		var shardCount int32
		for _, ids := range tagMap {
			shardCount += int32(len(ids))
		}
		im.shardStatus[shardID].ItemCount = shardCount
		totalCount += shardCount
	}
	im.indexedCount = totalCount

	// 重建前缀树
	if im.config.EnablePrefixCompression {
		im.rebuildPrefixTrees()
	}

	return nil
}

// 重建前缀树
func (im *OptimizedIndexManager) rebuildPrefixTrees() {
	// 清空旧的前缀树
	im.prefixTrees = make(map[uint32]*PrefixNode)

	// 对每个分片的每个标签重建前缀树
	for _, tagMap := range im.shards {
		for tag, ids := range tagMap {
			// 对每个ID添加到前缀树
			for _, id := range ids {
				im.updatePrefixTree(tag, id, true)
			}
		}
	}
}

// FindByKey 根据键查找
func (im *OptimizedIndexManager) FindByKey(tag uint32) ([]uint32, error) {
	// 创建结果切片
	var result []uint32

	// 遍历所有分片
	for shardID := range im.shards {
		// 获取分片读锁
		im.shardMutexes[shardID].RLock()

		// 如果标签存在于当前分片
		if ids, ok := im.shards[shardID][tag]; ok {
			// 更新分片访问统计
			im.shardStatus[shardID].LastAccess = time.Now()
			atomic.AddInt64(&im.shardStatus[shardID].ReadCount, 1)

			// 追加ID到结果
			if result == nil {
				result = make([]uint32, 0, len(ids))
			}
			result = append(result, ids...)
		}

		// 释放分片读锁
		im.shardMutexes[shardID].RUnlock()
	}

	if result == nil {
		return nil, ErrIndexNotFound
	}

	return result, nil
}

// FindByTag 根据标签查找
func (im *OptimizedIndexManager) FindByTag(tag uint32) ([]uint32, error) {
	return im.FindByKey(tag)
}

// FindByTagInShard 按分片获取索引
func (im *OptimizedIndexManager) FindByTagInShard(tag uint32, shardID int) ([]uint32, error) {
	// 验证分片ID
	if shardID < 0 || shardID >= len(im.shards) {
		return nil, fmt.Errorf("invalid shard ID: %d", shardID)
	}

	// 获取分片读锁
	im.shardMutexes[shardID].RLock()
	defer im.shardMutexes[shardID].RUnlock()

	// 更新分片访问统计
	im.shardStatus[shardID].LastAccess = time.Now()
	atomic.AddInt64(&im.shardStatus[shardID].ReadCount, 1)

	// 如果标签存在于当前分片
	if ids, ok := im.shards[shardID][tag]; ok {
		// 创建结果副本
		result := make([]uint32, len(ids))
		copy(result, ids)
		return result, nil
	}

	return nil, ErrIndexNotFound
}

// FindByPattern 根据模式查找
func (im *OptimizedIndexManager) FindByPattern(pattern string) (map[uint32][]uint32, error) {
	// 创建结果映射
	result := make(map[uint32][]uint32)

	// 遍历所有分片
	for shardID := range im.shards {
		// 获取分片读锁
		im.shardMutexes[shardID].RLock()

		// 更新分片访问统计
		im.shardStatus[shardID].LastAccess = time.Now()
		atomic.AddInt64(&im.shardStatus[shardID].ReadCount, 1)

		// 对每个标签进行模式匹配
		for tag, ids := range im.shards[shardID] {
			tagStr := strconv.FormatUint(uint64(tag), 10)
			if strings.Contains(tagStr, pattern) {
				// 如果标签不在结果中，初始化
				if _, ok := result[tag]; !ok {
					result[tag] = make([]uint32, 0)
				}
				// 追加ID到结果
				result[tag] = append(result[tag], ids...)
			}
		}

		// 释放分片读锁
		im.shardMutexes[shardID].RUnlock()
	}

	if len(result) == 0 {
		return nil, ErrIndexNotFound
	}

	return result, nil
}

// UpdateIndices 更新索引
func (im *OptimizedIndexManager) UpdateIndices() error {
	// 设置更新状态
	im.statusMutex.Lock()
	if im.isUpdating {
		im.statusMutex.Unlock()
		return fmt.Errorf("index is already updating")
	}
	im.isUpdating = true
	im.progress = 0
	im.statusMutex.Unlock()

	// 确保在函数返回时清除更新状态
	defer func() {
		im.statusMutex.Lock()
		im.isUpdating = false
		im.progress = 100
		im.lastUpdateTime = time.Now()
		im.statusMutex.Unlock()
	}()

	// 遍历所有分片执行更新
	for shardID := range im.shards {
		// 更新进度
		im.statusMutex.Lock()
		im.progress = int32((shardID * 100) / len(im.shards))
		im.statusMutex.Unlock()

		// 获取分片写锁
		im.shardMutexes[shardID].Lock()

		// 执行分片优化
		im.optimizeShard(shardID)

		// 释放分片写锁
		im.shardMutexes[shardID].Unlock()
	}

	// 更新内存使用情况
	im.updateMemoryUsage()

	// 如果启用自动保存，则保存索引
	if im.config.AutoSave && im.config.IndexPath != "" {
		return im.SaveIndex(im.config.IndexPath)
	}

	return nil
}

// 优化单个分片
func (im *OptimizedIndexManager) optimizeShard(shardID int) {
	// 对每个标签的ID列表进行排序和去重
	for tag, ids := range im.shards[shardID] {
		if len(ids) > 1 {
			// 排序
			sort.Slice(ids, func(i, j int) bool {
				return ids[i] < ids[j]
			})

			// 去重
			j := 0
			for i := 1; i < len(ids); i++ {
				if ids[j] != ids[i] {
					j++
					ids[j] = ids[i]
				}
			}

			// 更新ID列表
			im.shards[shardID][tag] = ids[:j+1]
		}
	}

	// 更新分片状态
	im.shardStatus[shardID].ItemCount = 0
	for _, ids := range im.shards[shardID] {
		im.shardStatus[shardID].ItemCount += int32(len(ids))
	}
}

// GetStatus 获取索引状态
func (im *OptimizedIndexManager) GetStatus() *IndexStatus {
	im.statusMutex.RLock()
	defer im.statusMutex.RUnlock()

	// 计算总项目数
	totalItems := 0
	for _, shard := range im.shardStatus {
		totalItems += int(shard.ItemCount)
	}

	return &IndexStatus{
		TotalItems:       totalItems,
		IndexedItems:     int(im.indexedCount),
		LastUpdateTime:   im.lastUpdateTime,
		IsUpdating:       im.isUpdating,
		Progress:         int(im.progress),
		Error:            im.lastError,
		PendingUpdates:   int(im.pendingCount),
		ActiveWorkers:    int(im.activeWorkers),
		CompressionRatio: im.compressionRatio,
		MemoryUsage:      im.memoryUsage,
		ShardStatus:      im.shardStatus,
	}
}

// GetIndexMetadata 获取索引元数据
func (im *OptimizedIndexManager) GetIndexMetadata() *IndexMetadata {
	im.statusMutex.RLock()
	defer im.statusMutex.RUnlock()

	// 更新元数据中的项目数量
	im.metadata.ItemCount = int(im.indexedCount)
	im.metadata.ModifiedAt = im.lastUpdateTime

	// 创建副本
	metadata := im.metadata
	return &metadata
}

// IndexMetadata 索引元数据
func (im *OptimizedIndexManager) IndexMetadata(id uint32, tags []uint32) error {
	for _, tag := range tags {
		err := im.AddIndex(tag, id)
		if err != nil {
			return err
		}
	}
	return nil
}

// FindByPrefix 前缀搜索
func (im *OptimizedIndexManager) FindByPrefix(tag uint32, prefix string) ([]uint32, error) {
	// 如果启用了前缀压缩，使用前缀树进行搜索
	if im.config.EnablePrefixCompression {
		// 获取前缀树
		root, err := im.GetPrefixTree(tag)
		if err != nil {
			return nil, err
		}

		// 实现前缀树搜索逻辑
		result := im.searchPrefixTree(root, prefix)
		if len(result) == 0 {
			// 返回空切片而不是错误
			return []uint32{}, nil
		}
		return result, nil
	}

	// 如果没有启用前缀压缩，使用常规查找后过滤
	ids, err := im.FindByTag(tag)
	if err != nil {
		return nil, err
	}

	// 过滤符合前缀的ID
	result := make([]uint32, 0)
	for _, id := range ids {
		idStr := strconv.FormatUint(uint64(id), 10)
		if strings.HasPrefix(idStr, prefix) {
			result = append(result, id)
		}
	}

	// 返回空切片而不是错误
	return result, nil
}

// searchPrefixTree 在前缀树中查找匹配给定前缀的所有ID
func (im *OptimizedIndexManager) searchPrefixTree(root *PrefixNode, prefix string) []uint32 {
	if root == nil {
		return []uint32{}
	}

	// 如果前缀为空，返回当前节点的所有ID
	if prefix == "" {
		return im.collectAllIDs(root)
	}

	// 遍历前缀的每个字符
	currentNode := root
	for i, char := range prefix {
		childNode, ok := currentNode.Children[string(char)]
		if !ok {
			// 如果没有匹配的子节点，返回空集
			return []uint32{}
		}
		currentNode = childNode

		// 如果已经匹配到前缀的最后一个字符
		if i == len(prefix)-1 {
			// 收集此节点及其所有子节点的ID
			return im.collectAllIDs(currentNode)
		}
	}

	return []uint32{}
}

// collectAllIDs 收集节点及其所有子节点的ID
func (im *OptimizedIndexManager) collectAllIDs(node *PrefixNode) []uint32 {
	if node == nil {
		return []uint32{}
	}

	// 使用map去重
	uniqueIDs := make(map[uint32]struct{})

	// 添加当前节点的ID
	for _, id := range node.IDs {
		uniqueIDs[id] = struct{}{}
	}

	// 递归添加所有子节点的ID
	for _, child := range node.Children {
		childIDs := im.collectAllIDs(child)
		for _, id := range childIDs {
			uniqueIDs[id] = struct{}{}
		}
	}

	// 转换为切片
	result := make([]uint32, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		result = append(result, id)
	}

	return result
}

// GetPrefixTree 获取前缀树
func (im *OptimizedIndexManager) GetPrefixTree(tag uint32) (*PrefixNode, error) {
	// 如果不启用前缀压缩，返回错误
	if !im.config.EnablePrefixCompression {
		return nil, fmt.Errorf("prefix compression not enabled")
	}

	// 检查前缀树是否存在
	if prefixTree, ok := im.prefixTrees[tag]; ok {
		return prefixTree, nil
	}

	// 创建新的前缀树根节点
	root := &PrefixNode{
		Prefix:   "",
		Count:    0,
		Children: make(map[string]*PrefixNode),
	}

	// 将标签的所有ID添加到前缀树
	ids, err := im.FindByTag(tag)
	if err != nil {
		return nil, err
	}

	// 添加ID到前缀树
	for _, id := range ids {
		// 暂时只创建节点不做实际处理
		idStr := strconv.FormatUint(uint64(id), 10)
		_ = idStr // 避免未使用变量警告
	}

	// 保存前缀树
	im.prefixTrees[tag] = root

	return root, nil
}

// FindByRange 范围搜索
func (im *OptimizedIndexManager) FindByRange(tag uint32, start, end uint32) ([]uint32, error) {
	// 获取标签的所有ID
	ids, err := im.FindByTag(tag)
	if err != nil {
		return nil, err
	}

	// 过滤在范围内的ID
	result := make([]uint32, 0)
	for _, id := range ids {
		if id >= start && id <= end {
			result = append(result, id)
		}
	}

	if len(result) == 0 {
		return nil, ErrIndexNotFound
	}

	return result, nil
}

// FindCompound 复合查询
func (im *OptimizedIndexManager) FindCompound(conditions []IndexQueryCondition) ([]uint32, error) {
	if len(conditions) == 0 {
		return nil, fmt.Errorf("no conditions provided")
	}

	// 执行第一个条件获取初始结果集
	var result []uint32
	var err error

	firstCondition := conditions[0]
	switch firstCondition.Operation {
	case "eq": // 等于
		result, err = im.FindByTag(firstCondition.Tag)
	case "prefix": // 前缀
		prefix, ok := firstCondition.Value.(string)
		if !ok {
			return nil, fmt.Errorf("prefix value must be string")
		}
		result, err = im.FindByPrefix(firstCondition.Tag, prefix)
	case "range": // 范围
		rangeValues, ok := firstCondition.Value.([]uint32)
		if !ok || len(rangeValues) != 2 {
			return nil, fmt.Errorf("range value must be array of two uint32")
		}
		result, err = im.FindByRange(firstCondition.Tag, rangeValues[0], rangeValues[1])
	default:
		return nil, fmt.Errorf("unsupported operation: %s", firstCondition.Operation)
	}

	if err != nil {
		return nil, err
	}

	// 处理剩余条件
	for i := 1; i < len(conditions); i++ {
		condition := conditions[i]
		var conditionResult []uint32

		// 获取条件的结果集
		switch condition.Operation {
		case "eq": // 等于
			conditionResult, err = im.FindByTag(condition.Tag)
		case "prefix": // 前缀
			prefix, ok := condition.Value.(string)
			if !ok {
				return nil, fmt.Errorf("prefix value must be string")
			}
			conditionResult, err = im.FindByPrefix(condition.Tag, prefix)
		case "range": // 范围
			rangeValues, ok := condition.Value.([]uint32)
			if !ok || len(rangeValues) != 2 {
				return nil, fmt.Errorf("range value must be array of two uint32")
			}
			conditionResult, err = im.FindByRange(condition.Tag, rangeValues[0], rangeValues[1])
		default:
			return nil, fmt.Errorf("unsupported operation: %s", condition.Operation)
		}

		if err != nil && err != ErrIndexNotFound {
			return nil, err
		}

		// 计算交集
		result = im.intersection(result, conditionResult)
		if len(result) == 0 {
			return nil, ErrIndexNotFound
		}
	}

	return result, nil
}

// 计算两个ID列表的交集
func (im *OptimizedIndexManager) intersection(a, b []uint32) []uint32 {
	// 创建b的映射，用于快速查找
	bMap := make(map[uint32]bool)
	for _, id := range b {
		bMap[id] = true
	}

	// 找出既在a中又在b中的ID
	result := make([]uint32, 0)
	for _, id := range a {
		if bMap[id] {
			result = append(result, id)
		}
	}

	return result
}

// OptimizeIndex 优化索引以提高性能
func (im *OptimizedIndexManager) OptimizeIndex() error {
	// 设置优化状态
	im.statusMutex.Lock()
	if im.isUpdating {
		im.statusMutex.Unlock()
		return fmt.Errorf("index is already updating")
	}
	im.isUpdating = true
	im.progress = 0
	im.statusMutex.Unlock()

	// 确保在函数返回时清除优化状态
	defer func() {
		im.statusMutex.Lock()
		im.isUpdating = false
		im.progress = 100
		im.lastUpdateTime = time.Now()
		im.statusMutex.Unlock()
	}()

	// 1. 优化每个分片
	totalShards := len(im.shards)
	for shardID := range im.shards {
		// 更新进度
		im.statusMutex.Lock()
		im.progress = int32((shardID * 50) / totalShards) // 前50%的进度用于分片优化
		im.statusMutex.Unlock()

		// 获取分片写锁
		im.shardMutexes[shardID].Lock()
		im.optimizeShard(shardID)
		im.shardMutexes[shardID].Unlock()
	}

	// 2. 如果启用前缀压缩，重建前缀树
	if im.config.EnablePrefixCompression {
		im.statusMutex.Lock()
		im.progress = 50 // 50%的进度
		im.statusMutex.Unlock()

		im.rebuildPrefixTrees()
	}

	// 3. 计算压缩率
	im.statusMutex.Lock()
	im.progress = 75 // 75%的进度
	im.statusMutex.Unlock()

	// 计算大小优化率
	beforeSize := im.memoryUsage
	im.updateMemoryUsage()
	afterSize := im.memoryUsage

	if beforeSize > 0 {
		im.compressionRatio = float64(beforeSize-afterSize) / float64(beforeSize) * 100
	}

	// 4. 如果启用自动保存，则保存索引
	if im.config.AutoSave && im.config.IndexPath != "" {
		return im.SaveIndex(im.config.IndexPath)
	}

	return nil
}

// CompressIndex 压缩索引以减少内存占用
func (im *OptimizedIndexManager) CompressIndex(level int) error {
	// 设置优化状态
	im.statusMutex.Lock()
	if im.isUpdating {
		im.statusMutex.Unlock()
		return fmt.Errorf("index is already updating")
	}
	im.isUpdating = true
	im.progress = 0
	im.statusMutex.Unlock()

	// 确保在函数返回时清除优化状态
	defer func() {
		im.statusMutex.Lock()
		im.isUpdating = false
		im.progress = 100
		im.lastUpdateTime = time.Now()
		im.statusMutex.Unlock()
	}()

	// 根据压缩级别执行不同的压缩操作
	switch level {
	case 0: // 不压缩
		return nil

	case 1: // 轻度压缩：去重和排序
		for shardID := range im.shards {
			im.shardMutexes[shardID].Lock()
			im.deduplicateAndSortShard(shardID)
			im.shardMutexes[shardID].Unlock()
		}

	case 2: // 中度压缩：添加前缀树索引
		for shardID := range im.shards {
			im.shardMutexes[shardID].Lock()
			im.deduplicateAndSortShard(shardID)
			im.shardMutexes[shardID].Unlock()
		}

		return im.BuildPrefixIndex()

	case 3: // 高度压缩：所有压缩技术
		for shardID := range im.shards {
			im.shardMutexes[shardID].Lock()
			im.deduplicateAndSortShard(shardID)
			im.shardMutexes[shardID].Unlock()
		}

		if err := im.BuildPrefixIndex(); err != nil {
			return err
		}

		// 应用增量压缩
		im.applyDeltaCompression()
	}

	// 更新内存使用情况
	im.updateMemoryUsage()

	return nil
}

// deduplicateAndSortShard 对分片中的ID列表进行去重和排序
func (im *OptimizedIndexManager) deduplicateAndSortShard(shardID int) {
	for tag, ids := range im.shards[shardID] {
		if len(ids) <= 1 {
			continue
		}

		// 排序
		sort.Slice(ids, func(i, j int) bool {
			return ids[i] < ids[j]
		})

		// 去重
		j := 0
		for i := 1; i < len(ids); i++ {
			if ids[j] != ids[i] {
				j++
				ids[j] = ids[i]
			}
		}

		// 更新ID列表
		if j+1 < len(ids) {
			im.shards[shardID][tag] = ids[:j+1]
		}
	}
}

// applyDeltaCompression 应用增量压缩
func (im *OptimizedIndexManager) applyDeltaCompression() {
	// 实现增量压缩算法
	// 这里是简化实现，实际应用中可能需要更复杂的算法
	for shardID := range im.shards {
		im.shardMutexes[shardID].Lock()
		for tag, ids := range im.shards[shardID] {
			if len(ids) < 3 {
				continue
			}

			// 这里可以实现实际的增量压缩算法
			// 例如，将相邻ID之间的差值存储，而不是存储完整的ID

			// 简化:仅作为示例
			im.shards[shardID][tag] = ids
		}
		im.shardMutexes[shardID].Unlock()
	}
}

// BuildPrefixIndex 构建前缀索引以加速前缀查询
func (im *OptimizedIndexManager) BuildPrefixIndex() error {
	// 清空现有前缀树
	im.prefixTrees = make(map[uint32]*PrefixNode)

	// 收集所有标签
	tags := make(map[uint32]struct{})
	for _, shard := range im.shards {
		for tag := range shard {
			tags[tag] = struct{}{}
		}
	}

	// 为每个标签构建前缀树
	for tag := range tags {
		// 创建根节点
		root := &PrefixNode{
			Children: make(map[string]*PrefixNode),
			IDs:      nil,
			Count:    0,
			Depth:    0,
		}

		// 收集标签对应的所有ID
		var allIDs []uint32
		for shardID := range im.shards {
			if ids, ok := im.shards[shardID][tag]; ok {
				allIDs = append(allIDs, ids...)
			}
		}

		// 填充前缀树
		maxDepth := 8 // 默认最大深度
		for _, id := range allIDs {
			// 将ID转换为字符串用于前缀匹配
			idStr := strconv.FormatUint(uint64(id), 10)
			im.addToPrefixTree(root, idStr, id, maxDepth)
		}

		// 存储前缀树
		im.prefixTrees[tag] = root
	}

	return nil
}

// addToPrefixTree 将ID添加到前缀树
func (im *OptimizedIndexManager) addToPrefixTree(node *PrefixNode, prefix string, id uint32, maxDepth int) {
	// 添加ID到当前节点
	node.IDs = append(node.IDs, id)
	node.Count++

	// 如果已达到最大深度或前缀为空，则停止
	if len(prefix) == 0 || node.Depth >= maxDepth {
		return
	}

	// 取第一个字符作为前缀
	char := prefix[0:1]
	remaining := prefix[1:]

	// 如果子节点不存在，创建新节点
	if _, ok := node.Children[char]; !ok {
		node.Children[char] = &PrefixNode{
			Children: make(map[string]*PrefixNode),
			IDs:      nil,
			Count:    0,
			Depth:    node.Depth + 1,
		}
	}

	// 递归添加到子节点
	im.addToPrefixTree(node.Children[char], remaining, id, maxDepth)
}

// RebalanceShards 重新平衡分片以优化负载分布
func (im *OptimizedIndexManager) RebalanceShards() error {
	// 设置优化状态
	im.statusMutex.Lock()
	if im.isUpdating {
		im.statusMutex.Unlock()
		return fmt.Errorf("index is already updating")
	}
	im.isUpdating = true
	im.progress = 0
	im.statusMutex.Unlock()

	// 确保在函数返回时清除优化状态
	defer func() {
		im.statusMutex.Lock()
		im.isUpdating = false
		im.progress = 100
		im.lastUpdateTime = time.Now()
		im.statusMutex.Unlock()
	}()

	// 计算每个分片的项目数
	counts := make([]int, len(im.shards))
	for shardID := range im.shards {
		count := 0
		for _, ids := range im.shards[shardID] {
			count += len(ids)
		}
		counts[shardID] = count

		// 更新分片状态
		im.shardStatus[shardID].ItemCount = int32(count)
	}

	// 计算平均项目数
	total := 0
	for _, count := range counts {
		total += count
	}
	avg := float64(total) / float64(len(im.shards))

	// 找出负载过重和过轻的分片
	threshold := 0.2 // 20%差异允许
	var overloaded, underloaded []int

	for shardID, count := range counts {
		ratio := float64(count) / avg
		if ratio > 1+threshold {
			// 过载分片
			overloaded = append(overloaded, shardID)
		} else if ratio < 1-threshold {
			// 负载不足的分片
			underloaded = append(underloaded, shardID)
		}
	}

	// 如果没有不平衡的分片，直接返回
	if len(overloaded) == 0 || len(underloaded) == 0 {
		return nil
	}

	// 平衡分片
	return im.balanceShards(overloaded, underloaded, counts, avg)
}

// balanceShards 平衡分片
func (im *OptimizedIndexManager) balanceShards(overloaded, underloaded []int, counts []int, avg float64) error {
	// 对过载分片按负载从高到低排序
	sort.Slice(overloaded, func(i, j int) bool {
		return counts[overloaded[i]] > counts[overloaded[j]]
	})

	// 对负载不足的分片按负载从低到高排序
	sort.Slice(underloaded, func(i, j int) bool {
		return counts[underloaded[i]] < counts[underloaded[j]]
	})

	// 移动数据
	for _, shardID := range overloaded {
		// 确保该分片仍然过载
		currentCount := 0
		for _, ids := range im.shards[shardID] {
			currentCount += len(ids)
		}

		if float64(currentCount) <= avg*1.1 {
			continue // 已经足够平衡
		}

		// 获取该分片的写锁
		im.shardMutexes[shardID].Lock()

		// 计算需要移动的数据量
		toMove := int(float64(currentCount) - avg)
		moved := 0

		// 移动数据到负载不足的分片
		for _, targetShardID := range underloaded {
			// 检查目标分片是否仍然负载不足
			targetCount := 0
			for _, ids := range im.shards[targetShardID] {
				targetCount += len(ids)
			}

			if float64(targetCount) >= avg*0.9 {
				continue // 已经足够平衡
			}

			// 获取目标分片的写锁
			im.shardMutexes[targetShardID].Lock()

			// 移动数据
			neededCount := int(avg) - targetCount
			if neededCount > toMove-moved {
				neededCount = toMove - moved
			}

			// 实际移动数据
			movedCount := im.moveData(shardID, targetShardID, neededCount)
			moved += movedCount

			// 释放目标分片的锁
			im.shardMutexes[targetShardID].Unlock()

			// 更新计数
			counts[targetShardID] += movedCount
			counts[shardID] -= movedCount

			// 如果已经移动了足够的数据，则跳出循环
			if moved >= toMove {
				break
			}
		}

		// 释放该分片的锁
		im.shardMutexes[shardID].Unlock()
	}

	return nil
}

// moveData 将数据从一个分片移动到另一个分片
func (im *OptimizedIndexManager) moveData(sourceShardID, targetShardID, count int) int {
	moved := 0

	// 遍历源分片中的所有标签
	for tag, ids := range im.shards[sourceShardID] {
		// 如果已经移动了足够的数据，则跳出循环
		if moved >= count {
			break
		}

		// 计算该标签需要移动的ID数量
		numToMove := min(len(ids), count-moved)
		if numToMove <= 0 {
			continue
		}

		// 获取要移动的ID
		movedIDs := ids[:numToMove]

		// 更新源分片
		im.shards[sourceShardID][tag] = ids[numToMove:]

		// 更新目标分片
		if _, ok := im.shards[targetShardID][tag]; !ok {
			im.shards[targetShardID][tag] = make([]uint32, 0, numToMove)
		}
		im.shards[targetShardID][tag] = append(im.shards[targetShardID][tag], movedIDs...)

		// 更新移动计数
		moved += numToMove
	}

	return moved
}

// MeasureQueryPerformance 测量查询性能
func (im *OptimizedIndexManager) MeasureQueryPerformance(benchmarkQueries []interface{}) (float64, error) {
	// 执行查询基准测试并返回性能改进百分比
	// 这是一个简化实现，实际应用中可能需要更复杂的算法

	// 设置优化状态
	im.statusMutex.Lock()
	if im.isUpdating {
		im.statusMutex.Unlock()
		return 0, fmt.Errorf("index is already updating")
	}
	im.isUpdating = true
	im.progress = 0
	im.statusMutex.Unlock()

	// 确保在函数返回时清除优化状态
	defer func() {
		im.statusMutex.Lock()
		im.isUpdating = false
		im.progress = 100
		im.lastUpdateTime = time.Now()
		im.statusMutex.Unlock()
	}()

	// 测量优化前查询性能
	beforeTime := im.runQueryBenchmark(benchmarkQueries)

	// 优化索引
	if err := im.OptimizeIndex(); err != nil {
		return 0, err
	}

	// 测量优化后查询性能
	afterTime := im.runQueryBenchmark(benchmarkQueries)

	// 计算性能改进百分比
	if beforeTime <= 0 {
		return 0, nil
	}

	improvement := (beforeTime - afterTime) / beforeTime * 100
	return improvement, nil
}

// runQueryBenchmark 运行查询基准测试
func (im *OptimizedIndexManager) runQueryBenchmark(benchmarkQueries []interface{}) float64 {
	if len(benchmarkQueries) == 0 {
		return 0
	}

	startTime := time.Now()

	for _, query := range benchmarkQueries {
		switch q := query.(type) {
		case uint32:
			// 标签查询
			_, _ = im.FindByTag(q)

		case string:
			// 模式查询
			_, _ = im.FindByPattern(q)

		case []IndexQueryCondition:
			// 复合查询
			if len(q) > 0 {
				// 简化处理：只取第一个条件作为标签查询
				tag := q[0].Tag
				_, _ = im.FindByTag(tag)
			}
		}
	}

	// 返回平均查询时间(毫秒)
	return float64(time.Since(startTime).Milliseconds()) / float64(len(benchmarkQueries))
}

// AsyncOptimizeIndex 异步优化索引
func (im *OptimizedIndexManager) AsyncOptimizeIndex() <-chan error {
	resultCh := make(chan error, 1)

	// 确保不会同时运行多个优化
	im.statusMutex.Lock()
	if im.isUpdating {
		im.statusMutex.Unlock()
		resultCh <- fmt.Errorf("index is already updating")
		close(resultCh)
		return resultCh
	}
	im.isUpdating = true
	im.progress = 0
	im.statusMutex.Unlock()

	// 在后台协程中运行优化
	go func() {
		defer func() {
			im.statusMutex.Lock()
			im.isUpdating = false
			im.progress = 100
			im.lastUpdateTime = time.Now()
			im.statusMutex.Unlock()

			close(resultCh)
		}()

		// 执行优化并发送结果
		err := im.OptimizeIndex()
		resultCh <- err
	}()

	return resultCh
}

// GetOptimizationStats 获取索引优化统计信息
func (im *OptimizedIndexManager) GetOptimizationStats() *OptimizationStats {
	return &OptimizationStats{
		SizeBefore:                  im.memoryUsage,
		SizeAfter:                   im.memoryUsage,
		ExecutionTime:               time.Since(im.lastUpdateTime),
		OptimizedItems:              int(im.indexedCount),
		CompressionRatio:            im.compressionRatio,
		PrefixTreeNodes:             im.countPrefixTreeNodes(),
		PrefixTreeDepth:             im.calculatePrefixTreeDepth(),
		MemoryImprovement:           0, // 实际应用中计算
		QueryPerformanceImprovement: 0, // 实际应用中计算
	}
}

// countPrefixTreeNodes 计算前缀树节点数
func (im *OptimizedIndexManager) countPrefixTreeNodes() int {
	count := 0
	for _, root := range im.prefixTrees {
		if root != nil {
			count += im.countNodesRecursive(root)
		}
	}
	return count
}

// countNodesRecursive 递归计算节点数
func (im *OptimizedIndexManager) countNodesRecursive(node *PrefixNode) int {
	if node == nil {
		return 0
	}

	count := 1 // 当前节点
	for _, child := range node.Children {
		count += im.countNodesRecursive(child)
	}

	return count
}

// calculatePrefixTreeDepth 计算前缀树深度
func (im *OptimizedIndexManager) calculatePrefixTreeDepth() int {
	maxDepth := 0
	for _, root := range im.prefixTrees {
		if root != nil {
			depth := im.calculateDepthRecursive(root)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}
	return maxDepth
}

// calculateDepthRecursive 递归计算深度
func (im *OptimizedIndexManager) calculateDepthRecursive(node *PrefixNode) int {
	if node == nil || len(node.Children) == 0 {
		return 0
	}

	maxChildDepth := 0
	for _, child := range node.Children {
		childDepth := im.calculateDepthRecursive(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return maxChildDepth + 1
}

// min返回两个整数中较小的一个
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
