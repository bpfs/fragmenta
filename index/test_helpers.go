package index

// 此文件包含测试所需的共享辅助代码，包括模拟实现

// MockIndexManager 模拟索引管理器实现
type MockIndexManager struct {
	// 标签到ID的映射
	tagToIDs map[uint32][]uint32
	// 所有ID的列表
	allIDs []uint32
	// ID计数器
	idCounter uint32
	// 分片ID到标签映射的映射
	shardTagToIDs map[int]map[uint32][]uint32
	// 标签名称到标签ID的映射
	tagNameToID map[string]uint32
}

// NewMockIndexManager 创建模拟索引管理器
func NewMockIndexManager() *MockIndexManager {
	return &MockIndexManager{
		tagToIDs:      make(map[uint32][]uint32),
		allIDs:        make([]uint32, 0),
		shardTagToIDs: make(map[int]map[uint32][]uint32),
		tagNameToID:   make(map[string]uint32),
	}
}

// AddIndex 添加索引
func (m *MockIndexManager) AddIndex(tag uint32, id uint32) error {
	if _, ok := m.tagToIDs[tag]; !ok {
		m.tagToIDs[tag] = make([]uint32, 0)
	}
	m.tagToIDs[tag] = append(m.tagToIDs[tag], id)

	// 如果ID不在allIDs中，则添加
	found := false
	for _, existingID := range m.allIDs {
		if existingID == id {
			found = true
			break
		}
	}
	if !found {
		m.allIDs = append(m.allIDs, id)
	}

	return nil
}

// RemoveIndex 移除索引
func (m *MockIndexManager) RemoveIndex(tag uint32, id uint32) error {
	if ids, ok := m.tagToIDs[tag]; ok {
		newIDs := make([]uint32, 0)
		for _, existingID := range ids {
			if existingID != id {
				newIDs = append(newIDs, existingID)
			}
		}
		m.tagToIDs[tag] = newIDs
	}
	return nil
}

// FindByKey 根据键查找
func (m *MockIndexManager) FindByKey(tag uint32) ([]uint32, error) {
	if ids, ok := m.tagToIDs[tag]; ok {
		return ids, nil
	}
	return []uint32{}, nil
}

// FindByPattern 根据模式查找
func (m *MockIndexManager) FindByPattern(pattern string) (map[uint32][]uint32, error) {
	// 简单实现：返回所有标签和ID
	return m.tagToIDs, nil
}

// UpdateIndices 更新索引
func (m *MockIndexManager) UpdateIndices() error {
	return nil
}

// GetStatus 获取索引状态
func (m *MockIndexManager) GetStatus() *IndexStatus {
	return &IndexStatus{
		TotalItems:   len(m.allIDs),
		IndexedItems: len(m.allIDs),
		MemoryUsage:  int64(len(m.allIDs) * 8), // 假设每个ID占用8字节
	}
}

// LoadIndex 加载索引
func (m *MockIndexManager) LoadIndex(path string) error {
	return nil
}

// SaveIndex 保存索引
func (m *MockIndexManager) SaveIndex(path string) error {
	return nil
}

// IndexMetadata 索引元数据
func (m *MockIndexManager) IndexMetadata(id uint32, tags []uint32) error {
	for _, tag := range tags {
		m.AddIndex(tag, id)
	}
	return nil
}

// FindByTag 根据标签查找
func (m *MockIndexManager) FindByTag(tag uint32) ([]uint32, error) {
	if ids, ok := m.tagToIDs[tag]; ok {
		return ids, nil
	}
	return []uint32{}, nil
}

// AsyncAddIndex 异步添加索引
func (m *MockIndexManager) AsyncAddIndex(tag uint32, id uint32) error {
	return m.AddIndex(tag, id)
}

// AsyncRemoveIndex 异步移除索引
func (m *MockIndexManager) AsyncRemoveIndex(tag uint32, id uint32) error {
	return m.RemoveIndex(tag, id)
}

// BatchAddIndices 批量添加索引
func (m *MockIndexManager) BatchAddIndices(tags []uint32, ids []uint32) error {
	for i, tag := range tags {
		if i < len(ids) {
			m.AddIndex(tag, ids[i])
		}
	}
	return nil
}

// BatchRemoveIndices 批量移除索引
func (m *MockIndexManager) BatchRemoveIndices(tags []uint32, ids []uint32) error {
	for i, tag := range tags {
		if i < len(ids) {
			m.RemoveIndex(tag, ids[i])
		}
	}
	return nil
}

// GetIndexMetadata 获取索引元数据
func (m *MockIndexManager) GetIndexMetadata() *IndexMetadata {
	return &IndexMetadata{
		ShardCount: 1,
	}
}

// FindByTagInShard 在分片中根据标签查找
func (m *MockIndexManager) FindByTagInShard(tag uint32, shardID int) ([]uint32, error) {
	if shardMap, ok := m.shardTagToIDs[shardID]; ok {
		if ids, ok := shardMap[tag]; ok {
			return ids, nil
		}
	}
	return []uint32{}, nil
}

// OptimizeIndex 优化索引
func (m *MockIndexManager) OptimizeIndex() error {
	return nil
}

// GetPendingTaskCount 获取挂起任务数
func (m *MockIndexManager) GetPendingTaskCount() int {
	return 0
}

// GetPrefixTree 获取前缀树
func (m *MockIndexManager) GetPrefixTree(tag uint32) (*PrefixNode, error) {
	return nil, nil
}

// FindByPrefix 根据前缀查找
func (m *MockIndexManager) FindByPrefix(tag uint32, prefix string) ([]uint32, error) {
	return m.FindByTag(tag)
}

// FindByRange 根据范围查找
func (m *MockIndexManager) FindByRange(tag uint32, start, end uint32) ([]uint32, error) {
	ids, err := m.FindByTag(tag)
	if err != nil {
		return nil, err
	}

	result := make([]uint32, 0)
	for _, id := range ids {
		if id >= start && id <= end {
			result = append(result, id)
		}
	}
	return result, nil
}

// FindCompound 复合查询
func (m *MockIndexManager) FindCompound(conditions []IndexQueryCondition) ([]uint32, error) {
	if len(conditions) == 0 {
		return m.allIDs, nil
	}

	// 对于每个条件获取ID列表
	resultSets := make([][]uint32, 0, len(conditions))
	for _, condition := range conditions {
		var ids []uint32
		var err error

		// 支持特殊类型转换
		if condition.Tag == 1000 && condition.Operation == "eq" {
			// 这是type字段的查询
			if intValue, ok := condition.Value.(int); ok {
				// 查找type=value
				tagID := uint32(1000 + intValue - 1) // type值对应的实际标签ID
				ids, err = m.FindByTag(tagID)
			}
		} else if condition.Tag == 2000 && condition.Operation == "eq" {
			// 这是category字段的查询
			if intValue, ok := condition.Value.(int); ok {
				// 查找category=value
				tagID := uint32(2000 + intValue) // category值对应的实际标签ID
				ids, err = m.FindByTag(tagID)
			}
		} else if condition.Operation == "eq" || condition.Operation == "equals" {
			ids, err = m.FindByTag(condition.Tag)
		}

		if err != nil {
			return nil, err
		}

		if ids != nil {
			resultSets = append(resultSets, ids)
		}
	}

	if len(resultSets) == 0 {
		return []uint32{}, nil
	}

	// 如果只有一个结果集，直接返回
	if len(resultSets) == 1 {
		return resultSets[0], nil
	}

	// 取所有条件结果的交集
	return m.intersection(resultSets), nil
}

// union 计算多个集合的并集
func (m *MockIndexManager) union(sets ...[]uint32) []uint32 {
	if len(sets) == 0 {
		return []uint32{}
	}
	if len(sets) == 1 {
		return sets[0]
	}

	// 使用map记录所有ID
	idMap := make(map[uint32]bool)
	for _, set := range sets {
		for _, id := range set {
			idMap[id] = true
		}
	}

	// 转换回切片
	result := make([]uint32, 0, len(idMap))
	for id := range idMap {
		result = append(result, id)
	}
	return result
}

// intersection 计算多个集合的交集
func (m *MockIndexManager) intersection(sets [][]uint32) []uint32 {
	if len(sets) == 0 {
		return []uint32{}
	}
	if len(sets) == 1 {
		return sets[0]
	}

	// 使用第一个集合作为基准
	result := make([]uint32, 0)
	for _, id := range sets[0] {
		found := true
		// 检查ID是否在所有其他集合中存在
		for i := 1; i < len(sets); i++ {
			if !m.contains(sets[i], id) {
				found = false
				break
			}
		}
		if found {
			result = append(result, id)
		}
	}
	return result
}

// contains 检查ID是否在集合中
func (m *MockIndexManager) contains(set []uint32, id uint32) bool {
	for _, val := range set {
		if val == id {
			return true
		}
	}
	return false
}

// GetAllIDs 获取所有ID
func (m *MockIndexManager) GetAllIDs() ([]uint32, error) {
	return m.allIDs, nil
}

// MockMetadataProvider 模拟元数据提供器实现
type MockMetadataProvider struct {
	metadata map[uint32]map[string]interface{}
}

// NewMockMetadataProvider 创建模拟元数据提供器
func NewMockMetadataProvider() *MockMetadataProvider {
	return &MockMetadataProvider{
		metadata: make(map[uint32]map[string]interface{}),
	}
}

// GetMetadataForID 获取指定ID的元数据
func (m *MockMetadataProvider) GetMetadataForID(id uint32) (map[string]interface{}, error) {
	if meta, ok := m.metadata[id]; ok {
		return meta, nil
	}
	return make(map[string]interface{}), nil
}

// GetAllIDs 获取所有ID
func (m *MockMetadataProvider) GetAllIDs() ([]uint32, error) {
	ids := make([]uint32, 0, len(m.metadata))
	for id := range m.metadata {
		ids = append(ids, id)
	}
	return ids, nil
}

// AddMetadata 添加元数据
func (m *MockMetadataProvider) AddMetadata(id uint32, metadata map[string]interface{}) {
	m.metadata[id] = metadata
}

// 创建测试用的模拟索引管理器
func createTestMockIndexManager() *MockIndexManager {
	mockManager := NewMockIndexManager()

	// 设置标签名称到标签ID的映射
	mockManager.tagNameToID["type"] = 1000
	mockManager.tagNameToID["category"] = 2000
	mockManager.tagNameToID["size"] = 3000

	// 添加type标签数据 - 用于测试查询执行
	// type=1 的文件（总共3个）
	mockManager.AddIndex(1000, 101) // type=1, ID=101
	mockManager.AddIndex(1000, 102) // type=1, ID=102
	mockManager.AddIndex(1000, 103) // type=1, ID=103

	// type=2 的文件（总共2个）
	mockManager.AddIndex(1001, 104) // type=2, ID=104
	mockManager.AddIndex(1001, 105) // type=2, ID=105

	// 添加category标签数据
	mockManager.AddIndex(2010, 101) // category=10, ID=101（同时也是type=1）

	// 将所有ID添加到allIDs列表中
	mockManager.allIDs = []uint32{101, 102, 103, 104, 105}

	return mockManager
}
