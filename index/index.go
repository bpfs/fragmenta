// package index 提供DeFSF格式的索引功能实现
package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

// 错误定义
var (
	ErrIndexNotFound  = errors.New("index not found")
	ErrInvalidTag     = errors.New("invalid tag")
	ErrInvalidPattern = errors.New("invalid pattern")
	ErrIndexCorrupted = errors.New("index corrupted")
	ErrIndexLocked    = errors.New("index locked by another process")
)

// IndexManagerImpl 索引管理器实现
type IndexManagerImpl struct {
	// 配置
	config *IndexConfig

	// 元数据索引
	metadataIndices map[uint32][]uint32

	// 内容索引
	contentIndices map[string][]uint32

	// 同步
	mutex sync.RWMutex

	// 状态
	lastUpdateTime time.Time
	isUpdating     bool
	progress       int
	indexedCount   int
	lastError      string

	// 前缀树相关字段
	prefixTrees    map[uint32]*PrefixNode // 前缀树索引
	prefixTreeLock sync.RWMutex           // 前缀树读写锁
}

// NewIndexManager 创建索引管理器
func NewIndexManager(config *IndexConfig) (*IndexManagerImpl, error) {
	if config == nil {
		config = &IndexConfig{
			IndexPath:    "",
			AutoSave:     true,
			AutoRebuild:  false,
			MaxCacheSize: 10 * 1024 * 1024, // 10MB
		}
	}

	im := &IndexManagerImpl{
		config:          config,
		metadataIndices: make(map[uint32][]uint32),
		contentIndices:  make(map[string][]uint32),
		lastUpdateTime:  time.Now(),
		isUpdating:      false,
		progress:        0,
		indexedCount:    0,
		lastError:       "",
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

	return im, nil
}

// AddIndex 添加索引
func (im *IndexManagerImpl) AddIndex(tag uint32, id uint32) error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 初始化map
	if im.metadataIndices == nil {
		im.metadataIndices = make(map[uint32][]uint32)
	}
	if im.prefixTrees == nil {
		im.prefixTrees = make(map[uint32]*PrefixNode)
	}

	// 更新索引映射
	im.metadataIndices[tag] = append(im.metadataIndices[tag], id)
	im.indexedCount++

	// 更新前缀树
	if err := im.updatePrefixTree(tag, id, OpAdd); err != nil {
		return fmt.Errorf("更新前缀树失败: %v", err)
	}

	im.lastUpdateTime = time.Now()
	return nil
}

// RemoveIndex 移除索引
func (im *IndexManagerImpl) RemoveIndex(tag uint32, id uint32) error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 初始化map
	if im.metadataIndices == nil {
		im.metadataIndices = make(map[uint32][]uint32)
	}
	if im.prefixTrees == nil {
		im.prefixTrees = make(map[uint32]*PrefixNode)
	}

	// 从索引映射中移除
	if ids, exists := im.metadataIndices[tag]; exists {
		for i, storedID := range ids {
			if storedID == id {
				im.metadataIndices[tag] = append(ids[:i], ids[i+1:]...)
				im.indexedCount--
				break
			}
		}
	}

	// 更新前缀树
	if err := im.updatePrefixTree(tag, id, OpRemove); err != nil {
		return fmt.Errorf("更新前缀树失败: %v", err)
	}

	im.lastUpdateTime = time.Now()
	return nil
}

// FindByKey 根据键查找
func (im *IndexManagerImpl) FindByKey(tag uint32) ([]uint32, error) {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	// 检查标签是否存在
	if ids, ok := im.metadataIndices[tag]; ok {
		// 返回副本
		result := make([]uint32, len(ids))
		copy(result, ids)
		return result, nil
	}

	return nil, ErrIndexNotFound
}

// FindByPattern 根据模式查找
func (im *IndexManagerImpl) FindByPattern(pattern string) (map[uint32][]uint32, error) {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	// 实际实现应支持正则表达式或通配符
	// 此处简化为返回所有索引
	result := make(map[uint32][]uint32)

	for tag, ids := range im.metadataIndices {
		// 创建副本
		result[tag] = make([]uint32, len(ids))
		copy(result[tag], ids)
	}

	return result, nil
}

// UpdateIndices 更新索引
func (im *IndexManagerImpl) UpdateIndices() error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 更新状态
	im.isUpdating = true
	im.progress = 0
	im.lastError = ""

	// 实际实现应执行索引更新操作
	// 此处简化为仅更新状态

	// 更新完成
	im.isUpdating = false
	im.progress = 100
	im.lastUpdateTime = time.Now()

	// 如果启用自动保存，则保存索引
	if im.config.AutoSave && im.config.IndexPath != "" {
		return im.SaveIndex(im.config.IndexPath)
	}

	return nil
}

// GetStatus 获取索引状态
func (im *IndexManagerImpl) GetStatus() *IndexStatus {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	return &IndexStatus{
		TotalItems:     len(im.metadataIndices) + len(im.contentIndices),
		IndexedItems:   im.indexedCount,
		LastUpdateTime: im.lastUpdateTime,
		IsUpdating:     im.isUpdating,
		Progress:       im.progress,
		Error:          im.lastError,
	}
}

// LoadIndex 加载索引
func (im *IndexManagerImpl) LoadIndex(path string) error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		logger.Error("打开索引文件失败", "error", err)
		return err
	}
	defer file.Close()

	// 读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		logger.Error("读取索引文件失败", "error", err)
		return err
	}

	// 解析JSON
	var indices struct {
		MetadataIndices map[uint32][]uint32 `json:"metadata_indices"`
		ContentIndices  map[string][]uint32 `json:"content_indices"`
		LastUpdateTime  time.Time           `json:"last_update_time"`
	}

	err = json.Unmarshal(data, &indices)
	if err != nil {
		logger.Error("解析索引文件失败", "error", err)
		return err
	}

	// 更新索引
	im.metadataIndices = indices.MetadataIndices
	im.contentIndices = indices.ContentIndices
	im.lastUpdateTime = indices.LastUpdateTime
	im.indexedCount = 0

	// 计算索引数量
	for _, ids := range im.metadataIndices {
		im.indexedCount += len(ids)
	}

	return nil
}

// SaveIndex 保存索引
func (im *IndexManagerImpl) SaveIndex(path string) error {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	// 创建索引数据
	indices := struct {
		MetadataIndices map[uint32][]uint32 `json:"metadata_indices"`
		ContentIndices  map[string][]uint32 `json:"content_indices"`
		LastUpdateTime  time.Time           `json:"last_update_time"`
	}{
		MetadataIndices: im.metadataIndices,
		ContentIndices:  im.contentIndices,
		LastUpdateTime:  im.lastUpdateTime,
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(indices, "", "  ")
	if err != nil {
		logger.Error("序列化索引文件失败", "error", err)
		return err
	}

	// 写入文件
	return os.WriteFile(path, data, 0644)
}

// IndexMetadata 索引元数据
func (im *IndexManagerImpl) IndexMetadata(id uint32, tags []uint32) error {
	for _, tag := range tags {
		err := im.AddIndex(tag, id)
		if err != nil {
			logger.Error("索引元数据失败", "error", err)
			return err
		}
	}

	return nil
}

// FindByTag 根据标签查找
func (im *IndexManagerImpl) FindByTag(tag uint32) ([]uint32, error) {
	return im.FindByKey(tag)
}

// AsyncAddIndex 异步添加索引
func (im *IndexManagerImpl) AsyncAddIndex(tag uint32, id uint32) error {
	// 由于当前实现是同步的，直接调用同步方法
	return im.AddIndex(tag, id)
}

// AsyncRemoveIndex 异步移除索引
func (im *IndexManagerImpl) AsyncRemoveIndex(tag uint32, id uint32) error {
	// 由于当前实现是同步的，直接调用同步方法
	return im.RemoveIndex(tag, id)
}

// BatchAddIndices 批量添加索引
func (im *IndexManagerImpl) BatchAddIndices(tags []uint32, ids []uint32) error {
	if len(tags) != len(ids) {
		return fmt.Errorf("标签和ID数组长度不匹配")
	}

	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 批量添加
	for i := 0; i < len(tags); i++ {
		tag := tags[i]
		id := ids[i]

		// 检查标签是否存在
		if _, ok := im.metadataIndices[tag]; !ok {
			im.metadataIndices[tag] = make([]uint32, 0)
		}

		// 检查ID是否已存在
		exists := false
		for _, existingID := range im.metadataIndices[tag] {
			if existingID == id {
				exists = true
				break
			}
		}

		// 如果ID不存在，则添加
		if !exists {
			im.metadataIndices[tag] = append(im.metadataIndices[tag], id)
			im.indexedCount++
		}
	}

	// 更新状态
	im.lastUpdateTime = time.Now()

	// 如果启用自动保存，则保存索引
	if im.config.AutoSave && im.config.IndexPath != "" {
		return im.SaveIndex(im.config.IndexPath)
	}

	return nil
}

// BatchRemoveIndices 批量移除索引
func (im *IndexManagerImpl) BatchRemoveIndices(tags []uint32, ids []uint32) error {
	if len(tags) != len(ids) {
		return fmt.Errorf("标签和ID数组长度不匹配")
	}

	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 创建要删除的(tag,id)对的映射，用于快速查找
	removeMap := make(map[uint32]map[uint32]bool)
	for i := 0; i < len(tags); i++ {
		tag := tags[i]
		id := ids[i]

		if _, ok := removeMap[tag]; !ok {
			removeMap[tag] = make(map[uint32]bool)
		}
		removeMap[tag][id] = true
	}

	// 批量处理删除
	for tag, idMap := range removeMap {
		if ids, ok := im.metadataIndices[tag]; ok {
			newIds := make([]uint32, 0, len(ids))
			for _, id := range ids {
				if !idMap[id] {
					newIds = append(newIds, id)
				} else {
					im.indexedCount--
				}
			}

			// 更新或删除标签
			if len(newIds) > 0 {
				im.metadataIndices[tag] = newIds
			} else {
				delete(im.metadataIndices, tag)
			}
		}
	}

	// 更新状态
	im.lastUpdateTime = time.Now()

	// 如果启用自动保存，则保存索引
	if im.config.AutoSave && im.config.IndexPath != "" {
		return im.SaveIndex(im.config.IndexPath)
	}

	return nil
}

// GetIndexMetadata 获取索引元数据
func (im *IndexManagerImpl) GetIndexMetadata() *IndexMetadata {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	return &IndexMetadata{
		Version:    "1.0",
		CreatedAt:  time.Time{}, // 没有创建时间记录
		ModifiedAt: im.lastUpdateTime,
		ItemCount:  im.indexedCount,
		ShardCount: 1, // 基本实现不分片
	}
}

// FindByTagInShard 按分片查找标签
func (im *IndexManagerImpl) FindByTagInShard(tag uint32, shardID int) ([]uint32, error) {
	// 基本实现不支持分片，忽略shardID参数
	return im.FindByTag(tag)
}

// OptimizeIndex 优化索引
func (im *IndexManagerImpl) OptimizeIndex() error {
	// 基本实现不需要特殊优化
	return nil
}

// GetPendingTaskCount 获取待处理任务数
func (im *IndexManagerImpl) GetPendingTaskCount() int {
	// 基本实现是同步的，没有待处理任务
	return 0
}

// GetPrefixTree 获取指定标签的前缀树
func (im *IndexManagerImpl) GetPrefixTree(tag uint32) (*PrefixNode, error) {
	im.prefixTreeLock.RLock()
	defer im.prefixTreeLock.RUnlock()

	if im.prefixTrees == nil {
		im.prefixTrees = make(map[uint32]*PrefixNode)
	}

	tree, exists := im.prefixTrees[tag]
	if !exists {
		tree = &PrefixNode{
			Prefix:   "",
			Count:    0,
			Children: make(map[string]*PrefixNode),
			IDs:      make([]uint32, 0),
		}
		im.prefixTrees[tag] = tree
	}

	return tree, nil
}

// FindByPrefix 根据前缀查找
func (im *IndexManagerImpl) FindByPrefix(tag uint32, prefix string) ([]uint32, error) {
	im.prefixTreeLock.RLock()
	defer im.prefixTreeLock.RUnlock()

	tree, exists := im.prefixTrees[tag]
	if !exists {
		return nil, nil
	}

	// 如果前缀为空，返回所有ID
	if prefix == "" {
		return im.collectAllIDs(tree), nil
	}

	// 遍历前缀树查找匹配的节点
	current := tree
	for _, char := range prefix {
		next, exists := current.Children[string(char)]
		if !exists {
			return nil, nil
		}
		current = next
	}

	// 收集所有匹配的ID
	return im.collectAllIDs(current), nil
}

// collectAllIDs 收集节点及其所有子节点的ID
func (im *IndexManagerImpl) collectAllIDs(node *PrefixNode) []uint32 {
	ids := make([]uint32, 0)

	// 添加当前节点的ID
	ids = append(ids, node.IDs...)

	// 递归添加子节点的ID
	for _, child := range node.Children {
		ids = append(ids, im.collectAllIDs(child)...)
	}

	return ids
}

// updatePrefixTree 更新前缀树
func (im *IndexManagerImpl) updatePrefixTree(tag uint32, id uint32, operation UpdateOperation) error {
	im.prefixTreeLock.Lock()
	defer im.prefixTreeLock.Unlock()

	if im.prefixTrees == nil {
		im.prefixTrees = make(map[uint32]*PrefixNode)
	}

	tree, exists := im.prefixTrees[tag]
	if !exists {
		tree = &PrefixNode{
			Prefix:   "",
			Count:    0,
			Children: make(map[string]*PrefixNode),
			IDs:      make([]uint32, 0),
		}
		im.prefixTrees[tag] = tree
	}

	// 将ID转换为字符串用于构建前缀树
	idStr := fmt.Sprintf("%d", id)

	// 根据操作类型更新前缀树
	switch operation {
	case OpAdd:
		im.addToPrefixTree(tree, idStr, id)
	case OpRemove:
		im.removeFromPrefixTree(tree, idStr, id)
	}

	return nil
}

// addToPrefixTree 添加ID到前缀树
func (im *IndexManagerImpl) addToPrefixTree(node *PrefixNode, idStr string, id uint32) {
	current := node
	for i, char := range idStr {
		prefix := string(char)
		next, exists := current.Children[prefix]
		if !exists {
			next = &PrefixNode{
				Prefix:   prefix,
				Count:    1,
				Children: make(map[string]*PrefixNode),
				IDs:      make([]uint32, 0),
			}
			current.Children[prefix] = next
		} else {
			next.Count++
		}
		current = next

		// 如果是最后一个字符，添加ID
		if i == len(idStr)-1 {
			current.IDs = append(current.IDs, id)
		}
	}
}

// removeFromPrefixTree 从前缀树中移除ID
func (im *IndexManagerImpl) removeFromPrefixTree(node *PrefixNode, idStr string, id uint32) {
	current := node
	for i, char := range idStr {
		prefix := string(char)
		next, exists := current.Children[prefix]
		if !exists {
			return
		}
		next.Count--
		current = next

		// 如果是最后一个字符，移除ID
		if i == len(idStr)-1 {
			for j, storedID := range current.IDs {
				if storedID == id {
					current.IDs = append(current.IDs[:j], current.IDs[j+1:]...)
					break
				}
			}
		}

		// 如果节点没有子节点且没有ID，删除该节点
		if next.Count == 0 && len(next.Children) == 0 && len(next.IDs) == 0 {
			delete(current.Children, prefix)
		}
	}
}

// FindByRange 范围搜索
func (im *IndexManagerImpl) FindByRange(tag uint32, start, end uint32) ([]uint32, error) {
	// 获取标签对应的所有ID
	ids, err := im.FindByKey(tag)
	if err != nil {
		return nil, err
	}

	// 对ID列表进行排序（如果未排序）
	if !sort.SliceIsSorted(ids, func(i, j int) bool { return ids[i] < ids[j] }) {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	}

	// 使用二分查找找到起始位置
	startIdx := sort.Search(len(ids), func(i int) bool {
		return ids[i] >= start
	})

	// 使用二分查找找到结束位置
	endIdx := sort.Search(len(ids), func(i int) bool {
		return ids[i] > end
	})

	// 如果没有找到符合范围的ID
	if startIdx == len(ids) {
		return nil, ErrIndexNotFound
	}

	// 提取范围内的ID
	result := make([]uint32, endIdx-startIdx)
	copy(result, ids[startIdx:endIdx])

	if len(result) == 0 {
		return nil, ErrIndexNotFound
	}

	return result, nil
}

// FindCompound 复合查询
func (im *IndexManagerImpl) FindCompound(conditions []IndexQueryCondition) ([]uint32, error) {
	if len(conditions) == 0 {
		return nil, fmt.Errorf("没有提供查询条件")
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
			return nil, fmt.Errorf("前缀值必须是字符串类型")
		}
		result, err = im.FindByPrefix(firstCondition.Tag, prefix)
	case "range": // 范围
		rangeValues, ok := firstCondition.Value.([]uint32)
		if !ok || len(rangeValues) != 2 {
			return nil, fmt.Errorf("范围值必须是包含两个uint32的数组")
		}
		result, err = im.FindByRange(firstCondition.Tag, rangeValues[0], rangeValues[1])
	default:
		return nil, fmt.Errorf("不支持的操作类型: %s", firstCondition.Operation)
	}

	if err != nil && err != ErrIndexNotFound {
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
				return nil, fmt.Errorf("前缀值必须是字符串类型")
			}
			conditionResult, err = im.FindByPrefix(condition.Tag, prefix)
		case "range": // 范围
			rangeValues, ok := condition.Value.([]uint32)
			if !ok || len(rangeValues) != 2 {
				return nil, fmt.Errorf("范围值必须是包含两个uint32的数组")
			}
			conditionResult, err = im.FindByRange(condition.Tag, rangeValues[0], rangeValues[1])
		default:
			return nil, fmt.Errorf("不支持的操作类型: %s", condition.Operation)
		}

		if err != nil && err != ErrIndexNotFound {
			return nil, err
		}

		// 计算交集
		result = im.intersectIDs(result, conditionResult)
	}

	return result, nil
}

// intersectIDs 计算两个ID列表的交集
func (im *IndexManagerImpl) intersectIDs(a, b []uint32) []uint32 {
	if len(a) == 0 || len(b) == 0 {
		return []uint32{}
	}

	// 使用map优化查找
	bMap := make(map[uint32]bool, len(b))
	for _, id := range b {
		bMap[id] = true
	}

	result := make([]uint32, 0)
	for _, id := range a {
		if bMap[id] {
			result = append(result, id)
		}
	}

	return result
}
