package index

import (
	"sync"
	"time"
)

// IndexStatistics 存储索引统计信息
type IndexStatistics struct {
	// 索引基本信息
	IndexName     string
	IndexedFields []string
	IndexType     string

	// 记录统计信息
	TotalRecords       int64
	AvgRecordsPerValue float64
	DistinctValues     int64

	// 内存使用情况
	MemoryUsage int64 // 字节数

	// 缓存统计
	CacheHitRate float64
	CacheSize    int

	// 查询性能统计
	QueryLatency map[string]float64 // 毫秒
	Selectivity  map[string]float64 // 0.0-1.0，1.0表示全表扫描

	// 上次更新时间
	LastUpdated time.Time

	// 互斥锁
	mutex sync.RWMutex
}

// NewIndexStatistics 创建新的索引统计信息
func NewIndexStatistics() *IndexStatistics {
	return &IndexStatistics{
		IndexName:     "default",
		IndexedFields: []string{},
		IndexType:     "default",
		TotalRecords:  0,
		MemoryUsage:   0,
		CacheHitRate:  0.0,
		CacheSize:     0,
		QueryLatency:  make(map[string]float64),
		Selectivity:   make(map[string]float64),
		LastUpdated:   time.Now(),
	}
}

// UpdateTotalRecords 更新总记录数
func (s *IndexStatistics) UpdateTotalRecords(count int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.TotalRecords = count
	s.LastUpdated = time.Now()
}

// UpdateMemoryUsage 更新内存使用情况
func (s *IndexStatistics) UpdateMemoryUsage(bytes int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.MemoryUsage = bytes
	s.LastUpdated = time.Now()
}

// UpdateCacheStats 更新缓存统计
func (s *IndexStatistics) UpdateCacheStats(hitRate float64, size int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.CacheHitRate = hitRate
	s.CacheSize = size
	s.LastUpdated = time.Now()
}

// RecordQueryLatency 记录查询延迟
func (s *IndexStatistics) RecordQueryLatency(queryType string, latencyMs float64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.QueryLatency[queryType] = latencyMs
	s.LastUpdated = time.Now()
}

// UpdateSelectivity 更新选择性
func (s *IndexStatistics) UpdateSelectivity(field string, selectivity float64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Selectivity[field] = selectivity
	s.LastUpdated = time.Now()
}

// GetSnapshot 获取统计信息快照
func (s *IndexStatistics) GetSnapshot() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 复制延迟统计
	latencyMap := make(map[string]float64)
	for k, v := range s.QueryLatency {
		latencyMap[k] = v
	}

	// 复制选择性统计
	selectivityMap := make(map[string]float64)
	for k, v := range s.Selectivity {
		selectivityMap[k] = v
	}

	return map[string]interface{}{
		"index_name":            s.IndexName,
		"indexed_fields":        s.IndexedFields,
		"index_type":            s.IndexType,
		"total_records":         s.TotalRecords,
		"avg_records_per_value": s.AvgRecordsPerValue,
		"distinct_values":       s.DistinctValues,
		"memory_usage":          s.MemoryUsage,
		"cache_hit_rate":        s.CacheHitRate,
		"cache_size":            s.CacheSize,
		"query_latency":         latencyMap,
		"selectivity":           selectivityMap,
		"last_updated":          s.LastUpdated.Format(time.RFC3339),
	}
}

// QueryStatistics 查询统计信息
type QueryStatistics struct {
	// 查询类型
	QueryType string

	// 执行次数
	ExecutionCount int

	// 平均执行时间
	AvgExecutionTime float64 // 毫秒

	// 最长执行时间
	MaxExecutionTime float64 // 毫秒

	// 最短执行时间
	MinExecutionTime float64 // 毫秒

	// 平均结果数
	AvgResultCount float64

	// 缓存命中率
	CacheHitRate float64

	// 上次执行时间
	LastExecutedAt time.Time

	// 互斥锁
	mutex sync.RWMutex
}

// NewQueryStatistics 创建新的查询统计信息
func NewQueryStatistics(queryType string) *QueryStatistics {
	return &QueryStatistics{
		QueryType:      queryType,
		ExecutionCount: 0,
		LastExecutedAt: time.Now(),
	}
}

// RecordExecution 记录查询执行
func (s *QueryStatistics) RecordExecution(executionTime float64, resultCount int, cacheHit bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 更新执行次数
	s.ExecutionCount++

	// 更新执行时间统计
	if s.ExecutionCount == 1 {
		s.AvgExecutionTime = executionTime
		s.MaxExecutionTime = executionTime
		s.MinExecutionTime = executionTime
		s.AvgResultCount = float64(resultCount)
	} else {
		// 更新平均执行时间
		s.AvgExecutionTime = (s.AvgExecutionTime*float64(s.ExecutionCount-1) + executionTime) / float64(s.ExecutionCount)

		// 更新最长执行时间
		if executionTime > s.MaxExecutionTime {
			s.MaxExecutionTime = executionTime
		}

		// 更新最短执行时间
		if executionTime < s.MinExecutionTime {
			s.MinExecutionTime = executionTime
		}

		// 更新平均结果数
		s.AvgResultCount = (s.AvgResultCount*float64(s.ExecutionCount-1) + float64(resultCount)) / float64(s.ExecutionCount)
	}

	// 更新缓存命中率
	if cacheHit {
		s.CacheHitRate = (s.CacheHitRate*float64(s.ExecutionCount-1) + 1.0) / float64(s.ExecutionCount)
	} else {
		s.CacheHitRate = (s.CacheHitRate*float64(s.ExecutionCount-1) + 0.0) / float64(s.ExecutionCount)
	}

	// 更新最后执行时间
	s.LastExecutedAt = time.Now()
}

// GetSnapshot 获取统计信息快照
func (s *QueryStatistics) GetSnapshot() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]interface{}{
		"query_type":         s.QueryType,
		"execution_count":    s.ExecutionCount,
		"avg_execution_time": s.AvgExecutionTime,
		"max_execution_time": s.MaxExecutionTime,
		"min_execution_time": s.MinExecutionTime,
		"avg_result_count":   s.AvgResultCount,
		"cache_hit_rate":     s.CacheHitRate,
		"last_executed_at":   s.LastExecutedAt.Format(time.RFC3339),
	}
}

// StatisticsManager 统计信息管理器
type StatisticsManager struct {
	// 索引统计
	IndexStats *IndexStatistics

	// 查询统计
	QueryStats map[string]*QueryStatistics

	// 互斥锁
	mutex sync.RWMutex
}

// NewStatisticsManager 创建新的统计信息管理器
func NewStatisticsManager() *StatisticsManager {
	return &StatisticsManager{
		IndexStats: NewIndexStatistics(),
		QueryStats: make(map[string]*QueryStatistics),
	}
}

// GetQueryStatistics 获取或创建查询统计信息
func (m *StatisticsManager) GetQueryStatistics(queryType string) *QueryStatistics {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	stats, ok := m.QueryStats[queryType]
	if !ok {
		stats = NewQueryStatistics(queryType)
		m.QueryStats[queryType] = stats
	}

	return stats
}

// GetAllStatistics 获取所有统计信息快照
func (m *StatisticsManager) GetAllStatistics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := map[string]interface{}{
		"index": m.IndexStats.GetSnapshot(),
	}

	queryStats := make(map[string]interface{})
	for queryType, stats := range m.QueryStats {
		queryStats[queryType] = stats.GetSnapshot()
	}
	result["queries"] = queryStats

	return result
}

// ResetStatistics 重置统计信息
func (m *StatisticsManager) ResetStatistics() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.IndexStats = NewIndexStatistics()
	m.QueryStats = make(map[string]*QueryStatistics)
}
