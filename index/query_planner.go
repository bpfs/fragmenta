package index

import (
	"fmt"
	"sort"
	"time"
)

// 查询计划类型
type PlanType string

const (
	// 全表扫描
	FullScanPlan PlanType = "FULL_SCAN"
	// 索引查询
	IndexLookupPlan PlanType = "INDEX_LOOKUP"
	// 范围扫描
	RangeScanPlan PlanType = "RANGE_SCAN"
	// 联合索引查询
	CompoundIndexPlan PlanType = "COMPOUND_INDEX"
	// 并行执行
	ParallelPlan PlanType = "PARALLEL"
	// 排序
	SortPlan PlanType = "SORT"
	// 聚合
	AggregatePlan PlanType = "AGGREGATE"
	// 联合查询
	JoinPlan PlanType = "JOIN"
	// 空值快速路径
	NullPathPlan PlanType = "NULL_PATH"
	// 缓存结果
	CachedPlan PlanType = "CACHED"
)

// PlanCost 查询计划成本
type PlanCost struct {
	// 估计CPU成本
	CPUCost float64
	// 估计内存成本
	MemoryCost float64
	// 估计IO成本
	IOCost float64
	// 估计执行时间
	EstimatedTime time.Duration
	// 估计结果数量
	EstimatedRows int
	// 综合成本(加权总和)
	TotalCost float64
}

// QueryPlan 查询计划接口
type QueryPlan interface {
	// Execute 执行查询计划
	Execute() (*QueryResult, error)
	// GetCost 获取查询计划成本
	GetCost() PlanCost
	// GetType 获取查询计划类型
	GetType() PlanType
	// GetChildren 获取子查询计划
	GetChildren() []QueryPlan
	// GetDescription 获取查询计划描述
	GetDescription() string
	// ToJSON 转换为JSON便于可视化
	ToJSON() map[string]interface{}
}

// BasePlan 基础查询计划结构
type BasePlan struct {
	// 计划类型
	Type PlanType
	// 计划描述
	Description string
	// 成本信息
	Cost PlanCost
	// 子计划
	Children []QueryPlan
	// 条件信息
	Condition *QueryCondition
	// 结果集大小限制
	Limit int
	// 结果集偏移
	Offset int
}

// Execute 执行基础查询计划，子类需要覆盖此方法
func (p *BasePlan) Execute() (*QueryResult, error) {
	// 基础计划通常不直接执行，返回空结果
	return &QueryResult{
		IDs:           []uint32{},
		TotalCount:    0,
		ExecutionTime: 0,
	}, nil
}

// GetCost 获取查询计划成本
func (p *BasePlan) GetCost() PlanCost {
	return p.Cost
}

// GetType 获取查询计划类型
func (p *BasePlan) GetType() PlanType {
	return p.Type
}

// GetChildren 获取子查询计划
func (p *BasePlan) GetChildren() []QueryPlan {
	return p.Children
}

// GetDescription 获取查询计划描述
func (p *BasePlan) GetDescription() string {
	return p.Description
}

// ToJSON 转换为JSON
func (p *BasePlan) ToJSON() map[string]interface{} {
	result := map[string]interface{}{
		"type":        string(p.Type),
		"description": p.Description,
		"cost": map[string]interface{}{
			"cpu":            p.Cost.CPUCost,
			"memory":         p.Cost.MemoryCost,
			"io":             p.Cost.IOCost,
			"estimated_time": p.Cost.EstimatedTime.String(),
			"estimated_rows": p.Cost.EstimatedRows,
			"total":          p.Cost.TotalCost,
		},
	}

	if len(p.Children) > 0 {
		children := make([]map[string]interface{}, len(p.Children))
		for i, child := range p.Children {
			children[i] = child.ToJSON()
		}
		result["children"] = children
	}

	return result
}

// QueryPlanner 查询计划生成器接口
type QueryPlanner interface {
	// GeneratePlan 生成查询计划
	GeneratePlan(query *Query) (QueryPlan, error)
	// OptimizePlan 优化查询计划
	OptimizePlan(plan QueryPlan) (QueryPlan, error)
	// EstimateCost 估算查询计划成本
	EstimateCost(plan QueryPlan) (PlanCost, error)
	// GetStatistics 获取索引统计信息
	GetStatistics() *IndexStatistics
}

// DefaultQueryPlanner 默认查询计划生成器
type DefaultQueryPlanner struct {
	// 索引管理器
	indexManager IndexManager
	// 元数据提供器
	metadataProvider MetadataProvider
	// 索引统计信息
	statistics *IndexStatistics
	// 缓存系统
	queryCache QueryCache
	// 配置
	config *PlannerConfig
}

// PlannerConfig 计划生成器配置
type PlannerConfig struct {
	// 启用并行执行
	EnableParallel bool
	// 并行度（线程数）
	ParallelDegree int
	// 启用缓存
	EnableCache bool
	// 缓存大小
	CacheSize int
	// 是否对子查询启用缓存
	CacheSubqueries bool
	// 启用成本优化
	EnableCostOptimization bool
	// CPU成本权重
	CPUWeight float64
	// 内存成本权重
	MemoryWeight float64
	// IO成本权重
	IOWeight float64
	// 启用统计信息收集
	EnableStatistics bool
	// 统计信息收集间隔（秒）
	StatisticsInterval int
}

// NewQueryPlanner 创建查询计划生成器
func NewQueryPlanner(
	indexManager IndexManager,
	metadataProvider MetadataProvider,
	config *PlannerConfig,
) (*DefaultQueryPlanner, error) {
	if config == nil {
		config = &PlannerConfig{
			EnableParallel:         true,
			ParallelDegree:         4,
			EnableCache:            true,
			CacheSize:              1000,
			CacheSubqueries:        true,
			EnableCostOptimization: true,
			CPUWeight:              1.0,
			MemoryWeight:           0.5,
			IOWeight:               2.0,
			EnableStatistics:       true,
			StatisticsInterval:     60,
		}
	}

	planner := &DefaultQueryPlanner{
		indexManager:     indexManager,
		metadataProvider: metadataProvider,
		statistics:       &IndexStatistics{},
		config:           config,
	}

	// 初始化缓存
	if config.EnableCache {
		planner.queryCache = NewExtendedLRUQueryCache(config.CacheSize)
	}

	// 初始化统计信息
	if config.EnableStatistics {
		err := planner.initStatistics()
		if err != nil {
			return nil, fmt.Errorf("初始化统计信息失败: %w", err)
		}
	}

	return planner, nil
}

// 初始化统计信息
func (p *DefaultQueryPlanner) initStatistics() error {
	status := p.indexManager.GetStatus()
	p.statistics = NewIndexStatistics()
	p.statistics.UpdateTotalRecords(int64(status.IndexedItems))
	p.statistics.UpdateMemoryUsage(status.MemoryUsage)
	p.statistics.UpdateCacheStats(0.0, p.config.CacheSize)

	// TODO: 收集更详细的统计信息
	// 这里可以添加对各种标签的统计分析

	return nil
}

// GeneratePlan 生成查询计划
func (p *DefaultQueryPlanner) GeneratePlan(query *Query) (QueryPlan, error) {
	// 1. 检查缓存
	if p.config.EnableCache {
		if plan := p.queryCache.Get(query); plan != nil {
			// 返回缓存的查询计划
			return &CachedQueryPlan{
				BasePlan: BasePlan{
					Type:        CachedPlan,
					Description: "从缓存中获取结果",
					Cost: PlanCost{
						CPUCost:       0.1,
						MemoryCost:    0.1,
						IOCost:        0,
						EstimatedTime: time.Microsecond * 100,
						EstimatedRows: 0, // 这个值需要从缓存中更新
						TotalCost:     0.1,
					},
				},
				OriginalPlan: plan,
				Cache:        p.queryCache,
			}, nil
		}
	}

	// 2. 根据查询条件生成候选计划
	candidatePlans, err := p.generateCandidatePlans(query)
	if err != nil {
		return nil, fmt.Errorf("生成候选计划失败: %w", err)
	}

	// 3. 选择最佳计划
	bestPlan := p.selectBestPlan(candidatePlans)

	// 4. 优化计划
	if p.config.EnableCostOptimization {
		bestPlan, err = p.OptimizePlan(bestPlan)
		if err != nil {
			return nil, fmt.Errorf("优化查询计划失败: %w", err)
		}
	}

	// 5. 缓存计划（可选）
	if p.config.EnableCache {
		p.queryCache.Put(query, bestPlan)
	}

	return bestPlan, nil
}

// generateCandidatePlans 生成候选查询计划
func (p *DefaultQueryPlanner) generateCandidatePlans(query *Query) ([]QueryPlan, error) {
	if query == nil || query.RootCondition == nil {
		// 如果没有查询条件，返回全表扫描计划
		return []QueryPlan{p.createFullScanPlan()}, nil
	}

	// 根据不同条件类型生成不同计划
	switch query.RootCondition.Operator {
	case OpAnd, OpOr:
		return p.generateLogicalPlan(query)
	case OpEqual, OpNotEqual, OpIn, OpNotIn:
		return p.generateLookupPlan(query)
	case OpGreater, OpGreaterEqual, OpLess, OpLessEqual, OpBetween:
		return p.generateRangePlan(query)
	case OpContains, OpStartsWith, OpEndsWith, OpMatches:
		return p.generateTextSearchPlan(query)
	default:
		// 对于其他操作符，返回全表扫描计划
		return []QueryPlan{p.createFullScanPlan()}, nil
	}
}

// createFullScanPlan 创建全表扫描计划
func (p *DefaultQueryPlanner) createFullScanPlan() QueryPlan {
	status := p.indexManager.GetStatus()
	totalRecords := int(status.IndexedItems)
	if totalRecords <= 0 {
		totalRecords = 1000 // 默认估算值
	}

	return &FullScanQueryPlan{
		BasePlan: BasePlan{
			Type:        FullScanPlan,
			Description: "全表扫描",
			Cost: PlanCost{
				CPUCost:       float64(totalRecords) * 0.001,
				MemoryCost:    float64(totalRecords) * 0.01,
				IOCost:        float64(totalRecords) * 0.1,
				EstimatedTime: time.Duration(totalRecords) * time.Microsecond,
				EstimatedRows: totalRecords,
				TotalCost:     float64(totalRecords) * 0.1,
			},
		},
		IndexManager: p.indexManager,
	}
}

// 更多的代码将在后续实现...

// FullScanQueryPlan 全表扫描查询计划
type FullScanQueryPlan struct {
	BasePlan
	IndexManager IndexManager
}

// Execute 执行全表扫描查询
func (p *FullScanQueryPlan) Execute() (*QueryResult, error) {
	// 实现全表扫描逻辑
	startTime := time.Now()

	// 使用通配符获取所有记录
	tagToIDsMap, err := p.IndexManager.FindByPattern("*")
	if err != nil {
		return nil, fmt.Errorf("获取所有记录失败: %w", err)
	}

	// 收集所有唯一ID
	idMap := make(map[uint32]struct{})
	for _, ids := range tagToIDsMap {
		for _, id := range ids {
			idMap[id] = struct{}{}
		}
	}

	// 转换为切片
	allIDs := make([]uint32, 0, len(idMap))
	for id := range idMap {
		allIDs = append(allIDs, id)
	}

	// 处理分页
	totalCount := len(allIDs)
	if p.Limit > 0 && p.Offset >= 0 && p.Offset < totalCount {
		end := p.Offset + p.Limit
		if end > totalCount {
			end = totalCount
		}
		allIDs = allIDs[p.Offset:end]
	}

	return &QueryResult{
		IDs:           allIDs,
		TotalCount:    totalCount,
		ExecutionTime: time.Since(startTime),
	}, nil
}

// CachedQueryPlan 缓存查询计划
type CachedQueryPlan struct {
	BasePlan
	OriginalPlan QueryPlan
	Cache        QueryCache
}

// Execute 执行缓存查询计划
func (p *CachedQueryPlan) Execute() (*QueryResult, error) {
	// 从缓存中获取结果
	// 假设Cache接口中有GetResult方法
	result, found := p.Cache.GetResult(p.OriginalPlan)
	if found {
		return result, nil
	}

	// 如果缓存中没有结果，则执行原始计划
	result, err := p.OriginalPlan.Execute()
	if err != nil {
		return nil, err
	}

	// 将结果存入缓存
	p.Cache.PutResult(p.OriginalPlan, result)

	return result, nil
}

// QueryCache 查询缓存接口
type QueryCache interface {
	// 获取查询计划
	Get(query *Query) QueryPlan
	// 存储查询计划
	Put(query *Query, plan QueryPlan)
	// 获取查询结果
	GetResult(plan QueryPlan) (*QueryResult, bool)
	// 存储查询结果
	PutResult(plan QueryPlan, result *QueryResult)
	// 清除缓存
	Clear()
}

// OptimizePlan 优化查询计划
func (p *DefaultQueryPlanner) OptimizePlan(plan QueryPlan) (QueryPlan, error) {
	// 具体优化逻辑将在后续实现
	return plan, nil
}

// EstimateCost 估算查询计划成本
func (p *DefaultQueryPlanner) EstimateCost(plan QueryPlan) (PlanCost, error) {
	// 具体成本估算逻辑将在后续实现
	return plan.GetCost(), nil
}

// GetStatistics 获取索引统计信息
func (p *DefaultQueryPlanner) GetStatistics() *IndexStatistics {
	return p.statistics
}

// selectBestPlan 选择最佳查询计划
func (p *DefaultQueryPlanner) selectBestPlan(plans []QueryPlan) QueryPlan {
	if len(plans) == 0 {
		return p.createFullScanPlan()
	}
	if len(plans) == 1 {
		return plans[0]
	}

	// 按成本排序
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].GetCost().TotalCost < plans[j].GetCost().TotalCost
	})

	// 返回成本最低的计划
	return plans[0]
}

// generateLogicalPlan 生成逻辑操作查询计划
func (p *DefaultQueryPlanner) generateLogicalPlan(query *Query) ([]QueryPlan, error) {
	// 逻辑查询计划生成将在后续实现
	return []QueryPlan{p.createFullScanPlan()}, nil
}

// generateLookupPlan 生成查找操作查询计划
func (p *DefaultQueryPlanner) generateLookupPlan(query *Query) ([]QueryPlan, error) {
	// 查找操作查询计划生成将在后续实现
	return []QueryPlan{p.createFullScanPlan()}, nil
}

// generateRangePlan 生成范围操作查询计划
func (p *DefaultQueryPlanner) generateRangePlan(query *Query) ([]QueryPlan, error) {
	// 范围操作查询计划生成将在后续实现
	return []QueryPlan{p.createFullScanPlan()}, nil
}

// generateTextSearchPlan 生成文本搜索查询计划
func (p *DefaultQueryPlanner) generateTextSearchPlan(query *Query) ([]QueryPlan, error) {
	// 文本搜索查询计划生成将在后续实现
	return []QueryPlan{p.createFullScanPlan()}, nil
}
