package index

import (
	"testing"
	"time"
)

// TestLRUCacheBasic 测试LRU缓存基本功能
func TestLRUCacheBasic(t *testing.T) {
	// 创建容量为5的缓存
	cache := NewExtendedLRUQueryCache(5)

	// 创建测试用的查询条件
	query1 := &Query{
		RootCondition: &QueryCondition{
			Field:     "name",
			FieldType: TypeString,
			Operator:  OpEqual,
			Value:     "test1",
		},
		Limit:  10,
		Offset: 0,
	}

	query2 := &Query{
		RootCondition: &QueryCondition{
			Field:     "age",
			FieldType: TypeInteger,
			Operator:  OpGreater,
			Value:     18,
		},
		Limit:  20,
		Offset: 0,
	}

	// 创建测试用的查询计划
	plan1 := &MockQueryPlan{
		Type:        FullScanPlan,
		Description: "测试计划1",
	}

	plan2 := &MockQueryPlan{
		Type:        IndexLookupPlan,
		Description: "测试计划2",
	}

	// 测试存储和获取
	cache.Put(query1, plan1)
	cache.Put(query2, plan2)

	// 验证缓存获取
	cachedPlan1 := cache.Get(query1)
	if cachedPlan1 == nil {
		t.Error("缓存未命中plan1")
	} else if cachedPlan1.GetType() != FullScanPlan {
		t.Errorf("缓存返回错误的计划类型: %v", cachedPlan1.GetType())
	}

	cachedPlan2 := cache.Get(query2)
	if cachedPlan2 == nil {
		t.Error("缓存未命中plan2")
	} else if cachedPlan2.GetType() != IndexLookupPlan {
		t.Errorf("缓存返回错误的计划类型: %v", cachedPlan2.GetType())
	}

	// 测试缓存结果
	result1 := &QueryResult{
		IDs:           []uint32{1, 2, 3},
		TotalCount:    3,
		ExecutionTime: time.Millisecond * 100,
	}

	result2 := &QueryResult{
		IDs:           []uint32{4, 5, 6, 7},
		TotalCount:    4,
		ExecutionTime: time.Millisecond * 200,
	}

	cache.PutResult(plan1, result1)
	cache.PutResult(plan2, result2)

	// 验证缓存获取结果
	cachedResult1, found1 := cache.GetResult(plan1)
	if !found1 {
		t.Error("缓存未命中result1")
	} else if len(cachedResult1.IDs) != 3 {
		t.Errorf("缓存返回错误的结果数量: %v", len(cachedResult1.IDs))
	}

	cachedResult2, found2 := cache.GetResult(plan2)
	if !found2 {
		t.Error("缓存未命中result2")
	} else if len(cachedResult2.IDs) != 4 {
		t.Errorf("缓存返回错误的结果数量: %v", len(cachedResult2.IDs))
	}

	// 测试缓存统计
	stats := cache.GetStats()
	if stats["hits"].(int) != 4 {
		t.Errorf("缓存命中统计错误: %v", stats["hits"])
	}
	if stats["misses"].(int) != 0 {
		t.Errorf("缓存未命中统计错误: %v", stats["misses"])
	}
}

// 模拟查询计划
type MockQueryPlan struct {
	Type        PlanType
	Description string
	Cost        PlanCost
	Children    []QueryPlan
}

// Execute 执行查询计划
func (p *MockQueryPlan) Execute() (*QueryResult, error) {
	return &QueryResult{
		IDs:           []uint32{1, 2, 3},
		TotalCount:    3,
		ExecutionTime: time.Millisecond * 10,
	}, nil
}

// GetCost 获取查询计划成本
func (p *MockQueryPlan) GetCost() PlanCost {
	return p.Cost
}

// GetType 获取查询计划类型
func (p *MockQueryPlan) GetType() PlanType {
	return p.Type
}

// GetChildren 获取子查询计划
func (p *MockQueryPlan) GetChildren() []QueryPlan {
	return p.Children
}

// GetDescription 获取查询计划描述
func (p *MockQueryPlan) GetDescription() string {
	return p.Description
}

// ToJSON 转换为JSON
func (p *MockQueryPlan) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"type":        string(p.Type),
		"description": p.Description,
	}
}

// TestLRUCacheEviction 测试LRU缓存驱逐策略
func TestLRUCacheEviction(t *testing.T) {
	// 创建容量为3的缓存
	cache := NewExtendedLRUQueryCache(3)

	// 创建6个测试查询和计划
	queries := make([]*Query, 6)
	plans := make([]QueryPlan, 6)

	for i := 0; i < 6; i++ {
		queries[i] = &Query{
			RootCondition: &QueryCondition{
				Field:     "test",
				FieldType: TypeString,
				Operator:  OpEqual,
				Value:     i,
			},
			Limit:  10,
			Offset: 0,
		}

		plans[i] = &MockQueryPlan{
			Type:        FullScanPlan,
			Description: "plan" + string(rune(i+'0')),
		}

		// 放入缓存
		cache.Put(queries[i], plans[i])
	}

	// 前三个应该被驱逐，后三个应该在缓存中
	for i := 0; i < 3; i++ {
		if cache.Get(queries[i]) != nil {
			t.Errorf("计划%d应该被驱逐但仍在缓存中", i)
		}
	}

	for i := 3; i < 6; i++ {
		if cache.Get(queries[i]) == nil {
			t.Errorf("计划%d应该在缓存中但被驱逐", i)
		}
	}

	// 测试LRU特性：访问计划4，然后添加新计划，计划3应该被驱逐
	cache.Get(queries[4])

	newQuery := &Query{
		RootCondition: &QueryCondition{
			Field:     "test",
			FieldType: TypeString,
			Operator:  OpEqual,
			Value:     100,
		},
		Limit:  10,
		Offset: 0,
	}

	newPlan := &MockQueryPlan{
		Type:        IndexLookupPlan,
		Description: "new plan",
	}

	cache.Put(newQuery, newPlan)

	// 计划3应该被驱逐
	if cache.Get(queries[3]) != nil {
		t.Error("计划3应该被驱逐，但仍在缓存中")
	}

	// 计划4应该仍在缓存中
	if cache.Get(queries[4]) == nil {
		t.Error("计划4应该仍在缓存中，但被驱逐")
	}

	// 计划5应该仍在缓存中
	if cache.Get(queries[5]) == nil {
		t.Error("计划5应该仍在缓存中，但被驱逐")
	}

	// 新计划应该在缓存中
	if cache.Get(newQuery) == nil {
		t.Error("新计划应该在缓存中，但被驱逐")
	}
}

// TestLRUCacheExpiry 测试LRU缓存过期
func TestLRUCacheExpiry(t *testing.T) {
	// 创建容量为5的缓存，使用自定义设置过期时间的方式
	cache := NewExtendedLRUQueryCache(5)

	// 手动设置字段
	cache.expiry = 1

	// 创建测试查询和计划
	query := &Query{
		RootCondition: &QueryCondition{
			Field:     "test",
			FieldType: TypeString,
			Operator:  OpEqual,
			Value:     "expiry",
		},
		Limit:  10,
		Offset: 0,
	}

	plan := &MockQueryPlan{
		Type:        FullScanPlan,
		Description: "expiry plan",
	}

	// 放入缓存
	cache.Put(query, plan)

	// 立即获取，应该命中
	if cache.Get(query) == nil {
		t.Error("缓存应该立即命中")
	}

	// 等待2秒，应该过期
	time.Sleep(time.Second * 2)

	// 再次获取，应该未命中
	if cache.Get(query) != nil {
		t.Error("缓存应该过期但仍命中")
	}

	// 检查统计
	stats := cache.GetStats()
	if stats["hits"].(int) != 1 {
		t.Errorf("缓存命中统计错误: %v", stats["hits"])
	}
	if stats["misses"].(int) != 1 {
		t.Errorf("缓存未命中统计错误: %v", stats["misses"])
	}
}

// TestLRUCacheClear 测试LRU缓存清除
func TestLRUCacheClear(t *testing.T) {
	// 创建容量为5的缓存
	cache := NewExtendedLRUQueryCache(5)

	// 创建测试查询和计划
	query := &Query{
		RootCondition: &QueryCondition{
			Field:     "test",
			FieldType: TypeString,
			Operator:  OpEqual,
			Value:     "clear",
		},
		Limit:  10,
		Offset: 0,
	}

	plan := &MockQueryPlan{
		Type:        FullScanPlan,
		Description: "clear plan",
	}

	// 放入缓存
	cache.Put(query, plan)

	// 清除缓存
	cache.Clear()

	// 获取，应该未命中
	if cache.Get(query) != nil {
		t.Error("缓存应该被清除但仍命中")
	}

	// 检查统计
	stats := cache.GetStats()
	if stats["hits"].(int) != 0 {
		t.Errorf("缓存命中统计错误: %v", stats["hits"])
	}
	if stats["misses"].(int) != 1 {
		t.Errorf("缓存未命中统计错误: %v", stats["misses"])
	}
}
