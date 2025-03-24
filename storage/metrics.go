package storage

import (
	"sort"
	"sync"
	"time"
)

// HybridPerformanceMetrics 混合存储性能指标
type HybridPerformanceMetrics struct {
	// 读取延迟
	ReadLatency []time.Duration
	// 平均读取延迟
	AvgReadLatency time.Duration
	// 最小读取延迟
	MinReadLatency time.Duration
	// 最大读取延迟
	MaxReadLatency time.Duration
	// 读取次数
	ReadCount int64

	// 写入延迟
	WriteLatency []time.Duration
	// 平均写入延迟
	AvgWriteLatency time.Duration
	// 最小写入延迟
	MinWriteLatency time.Duration
	// 最大写入延迟
	MaxWriteLatency time.Duration
	// 写入次数
	WriteCount int64

	// 缓存命中率
	CacheHitCount  int64
	CacheMissCount int64

	// 策略命中次数
	StrategyHitCount int64
	// 策略总决策次数
	StrategyTotalCount int64

	// 最近N次操作的延迟统计（滑动窗口）
	LastNCount int

	// 互斥锁
	mutex sync.RWMutex
}

// NewHybridPerformanceMetrics 创建新的性能指标
func NewHybridPerformanceMetrics(lastNCount int) *HybridPerformanceMetrics {
	if lastNCount <= 0 {
		lastNCount = 100 // 默认记录最近100次操作
	}

	return &HybridPerformanceMetrics{
		ReadLatency:        make([]time.Duration, 0, lastNCount),
		WriteLatency:       make([]time.Duration, 0, lastNCount),
		MinReadLatency:     time.Hour, // 初始化为一个较大的值
		MinWriteLatency:    time.Hour,
		LastNCount:         lastNCount,
		StrategyHitCount:   0,
		StrategyTotalCount: 0,
	}
}

// RecordReadLatency 记录读取延迟
func (m *HybridPerformanceMetrics) RecordReadLatency(latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.ReadCount++

	// 更新最小/最大延迟
	if latency < m.MinReadLatency {
		m.MinReadLatency = latency
	}
	if latency > m.MaxReadLatency {
		m.MaxReadLatency = latency
	}

	// 添加到滑动窗口
	m.ReadLatency = append(m.ReadLatency, latency)
	if len(m.ReadLatency) > m.LastNCount {
		// 移除最旧的记录
		m.ReadLatency = m.ReadLatency[1:]
	}

	// 计算平均延迟
	m.updateAvgReadLatency()
}

// RecordWriteLatency 记录写入延迟
func (m *HybridPerformanceMetrics) RecordWriteLatency(latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.WriteCount++

	// 更新最小/最大延迟
	if latency < m.MinWriteLatency {
		m.MinWriteLatency = latency
	}
	if latency > m.MaxWriteLatency {
		m.MaxWriteLatency = latency
	}

	// 添加到滑动窗口
	m.WriteLatency = append(m.WriteLatency, latency)
	if len(m.WriteLatency) > m.LastNCount {
		// 移除最旧的记录
		m.WriteLatency = m.WriteLatency[1:]
	}

	// 计算平均延迟
	m.updateAvgWriteLatency()
}

// RecordCacheHit 记录缓存命中
func (m *HybridPerformanceMetrics) RecordCacheHit() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.CacheHitCount++
}

// RecordCacheMiss 记录缓存未命中
func (m *HybridPerformanceMetrics) RecordCacheMiss() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.CacheMissCount++
}

// RecordStrategyHit 记录策略命中
func (m *HybridPerformanceMetrics) RecordStrategyHit() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.StrategyHitCount++
	m.StrategyTotalCount++
}

// RecordStrategyMiss 记录策略未命中
func (m *HybridPerformanceMetrics) RecordStrategyMiss() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.StrategyTotalCount++
}

// GetCacheHitRate 获取缓存命中率
func (m *HybridPerformanceMetrics) GetCacheHitRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	total := m.CacheHitCount + m.CacheMissCount
	if total == 0 {
		return 0
	}

	return float64(m.CacheHitCount) / float64(total)
}

// GetStrategyHitRate 获取策略命中率
func (m *HybridPerformanceMetrics) GetStrategyHitRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.StrategyTotalCount == 0 {
		return 0
	}

	return float64(m.StrategyHitCount) / float64(m.StrategyTotalCount)
}

// GetReadLatencyPercentile 获取读取延迟的百分位数
func (m *HybridPerformanceMetrics) GetReadLatencyPercentile(percentile float64) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.ReadLatency) == 0 {
		return 0
	}

	if percentile <= 0 {
		return m.MinReadLatency
	}

	if percentile >= 100 {
		return m.MaxReadLatency
	}

	// 创建副本进行排序
	sorted := make([]time.Duration, len(m.ReadLatency))
	copy(sorted, m.ReadLatency)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// 计算百分位索引
	idx := int(float64(len(sorted)-1) * percentile / 100)
	return sorted[idx]
}

// GetWriteLatencyPercentile 获取写入延迟的百分位数
func (m *HybridPerformanceMetrics) GetWriteLatencyPercentile(percentile float64) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.WriteLatency) == 0 {
		return 0
	}

	if percentile <= 0 {
		return m.MinWriteLatency
	}

	if percentile >= 100 {
		return m.MaxWriteLatency
	}

	// 创建副本进行排序
	sorted := make([]time.Duration, len(m.WriteLatency))
	copy(sorted, m.WriteLatency)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// 计算百分位索引
	idx := int(float64(len(sorted)-1) * percentile / 100)
	return sorted[idx]
}

// GetSummary 获取性能指标摘要
func (m *HybridPerformanceMetrics) GetSummary() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"ReadCount":      m.ReadCount,
		"AvgReadLatency": m.AvgReadLatency,
		"MinReadLatency": m.MinReadLatency,
		"MaxReadLatency": m.MaxReadLatency,
		"ReadLatencyP50": m.GetReadLatencyPercentile(50),
		"ReadLatencyP95": m.GetReadLatencyPercentile(95),
		"ReadLatencyP99": m.GetReadLatencyPercentile(99),

		"WriteCount":      m.WriteCount,
		"AvgWriteLatency": m.AvgWriteLatency,
		"MinWriteLatency": m.MinWriteLatency,
		"MaxWriteLatency": m.MaxWriteLatency,
		"WriteLatencyP50": m.GetWriteLatencyPercentile(50),
		"WriteLatencyP95": m.GetWriteLatencyPercentile(95),
		"WriteLatencyP99": m.GetWriteLatencyPercentile(99),

		"CacheHitRate":    m.GetCacheHitRate(),
		"StrategyHitRate": m.GetStrategyHitRate(),
	}
}

// ResetMetrics 重置性能指标
func (m *HybridPerformanceMetrics) ResetMetrics() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.ReadLatency = make([]time.Duration, 0, m.LastNCount)
	m.WriteLatency = make([]time.Duration, 0, m.LastNCount)
	m.ReadCount = 0
	m.WriteCount = 0
	m.MinReadLatency = time.Hour
	m.MaxReadLatency = 0
	m.MinWriteLatency = time.Hour
	m.MaxWriteLatency = 0
	m.AvgReadLatency = 0
	m.AvgWriteLatency = 0
	m.CacheHitCount = 0
	m.CacheMissCount = 0
	m.StrategyHitCount = 0
	m.StrategyTotalCount = 0
}

// updateAvgReadLatency 更新平均读取延迟
func (m *HybridPerformanceMetrics) updateAvgReadLatency() {
	if len(m.ReadLatency) == 0 {
		m.AvgReadLatency = 0
		return
	}

	var sum time.Duration
	for _, latency := range m.ReadLatency {
		sum += latency
	}

	m.AvgReadLatency = sum / time.Duration(len(m.ReadLatency))
}

// updateAvgWriteLatency 更新平均写入延迟
func (m *HybridPerformanceMetrics) updateAvgWriteLatency() {
	if len(m.WriteLatency) == 0 {
		m.AvgWriteLatency = 0
		return
	}

	var sum time.Duration
	for _, latency := range m.WriteLatency {
		sum += latency
	}

	m.AvgWriteLatency = sum / time.Duration(len(m.WriteLatency))
}
