// package storage 提供存储引擎的实现
package storage

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// StorageLocation 表示块存储的位置类型
type StorageLocation int

const (
	// LocationInline 表示内联存储
	LocationInline StorageLocation = iota
	// LocationContainer 表示容器存储
	LocationContainer
	// LocationDirectory 表示目录存储
	LocationDirectory
)

// String 返回存储位置的字符串表示
func (l StorageLocation) String() string {
	switch l {
	case LocationInline:
		return "Inline"
	case LocationContainer:
		return "Container"
	case LocationDirectory:
		return "Directory"
	default:
		return "Unknown"
	}
}

// StorageDecision 表示存储策略的决策结果
type StorageDecision struct {
	// Location 是建议的存储位置
	Location StorageLocation
	// Reason 是决策的原因
	Reason string
	// Score 是决策的评分（0-1.0），越高表示越适合
	Score float64
}

// PerformanceTarget 表示存储优化的性能目标
type PerformanceTarget string

const (
	// TargetBalanced 表示平衡速度和空间
	TargetBalanced PerformanceTarget = "balanced"
	// TargetSpeed 表示优先考虑速度
	TargetSpeed PerformanceTarget = "speed"
	// TargetSpace 表示优先考虑空间
	TargetSpace PerformanceTarget = "space"
)

// StrategyConfig 存储策略配置
type StrategyConfig struct {
	// InlineThreshold 内联存储阈值（字节）
	InlineThreshold int64
	// HotBlockThreshold 热块阈值（访问次数）
	HotBlockThreshold int
	// ColdBlockTimeMinutes 冷块时间阈值（分钟）
	ColdBlockTimeMinutes int
	// PerformanceTarget 性能目标
	PerformanceTarget PerformanceTarget
	// AutoBalanceEnabled 是否自动平衡存储分布
	AutoBalanceEnabled bool
	// StrategyName 策略名称
	StrategyName string
}

// NewDefaultStrategyConfig 创建默认的策略配置
func NewDefaultStrategyConfig() *StrategyConfig {
	return &StrategyConfig{
		InlineThreshold:      1024, // 1KB
		HotBlockThreshold:    5,
		ColdBlockTimeMinutes: 30,
		PerformanceTarget:    TargetBalanced,
		AutoBalanceEnabled:   true,
		StrategyName:         "adaptive",
	}
}

// BlockAccessRecord 表示块的访问记录
type BlockAccessRecord struct {
	// BlockKey 是块的键
	BlockKey string
	// AccessCount 是访问次数
	AccessCount int
	// LastAccessTime 是最后访问时间
	LastAccessTime time.Time
	// FirstAccessTime 是首次访问时间
	FirstAccessTime time.Time
	// Size 是块的大小
	Size int64
	// CurrentLocation 是当前存储位置
	CurrentLocation StorageLocation
}

// IsHot 判断块是否是热块
func (r *BlockAccessRecord) IsHot(threshold int) bool {
	return r.AccessCount >= threshold
}

// IsCold 判断块是否是冷块
func (r *BlockAccessRecord) IsCold(thresholdMinutes int) bool {
	return time.Since(r.LastAccessTime) > time.Duration(thresholdMinutes)*time.Minute
}

// GetScore 获取块的重要性评分
// 综合考虑访问频率和最近性，返回0-1.0的分数
func (r *BlockAccessRecord) GetScore() float64 {
	// 基础分数：考虑访问次数（正相关）和最后访问时间（反相关）
	timeFactor := 1.0 - float64(time.Since(r.LastAccessTime))/float64(24*time.Hour)
	if timeFactor < 0 {
		timeFactor = 0
	}

	// 简单公式：访问次数/10（最多贡献0.7）+ 时间因子（最多贡献0.3）
	countScore := float64(r.AccessCount) / 10.0
	if countScore > 0.7 {
		countScore = 0.7
	}

	return countScore + (timeFactor * 0.3)
}

// AccessTracker 跟踪块的访问情况
type AccessTracker struct {
	// records 存储所有块的访问记录
	records map[string]*BlockAccessRecord
	// hotBlocks 维护热点块集合
	hotBlocks map[string]struct{}
	// coldBlocks 维护冷块集合
	coldBlocks map[string]struct{}
	// mutex 用于并发访问保护
	mutex sync.RWMutex
	// config 策略配置
	config *StrategyConfig
}

// NewAccessTracker 创建一个新的访问追踪器
func NewAccessTracker(config *StrategyConfig) *AccessTracker {
	return &AccessTracker{
		records:    make(map[string]*BlockAccessRecord),
		hotBlocks:  make(map[string]struct{}),
		coldBlocks: make(map[string]struct{}),
		config:     config,
	}
}

// RecordAccess 记录块的访问
func (at *AccessTracker) RecordAccess(blockKey string, size int64, location StorageLocation) {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	now := time.Now()
	record, exists := at.records[blockKey]

	if !exists {
		record = &BlockAccessRecord{
			BlockKey:        blockKey,
			AccessCount:     1,
			LastAccessTime:  now,
			FirstAccessTime: now,
			Size:            size,
			CurrentLocation: location,
		}
		at.records[blockKey] = record
	} else {
		record.AccessCount++
		record.LastAccessTime = now
		record.CurrentLocation = location
		if size > 0 {
			record.Size = size
		}
	}

	// 更新热点块和冷块集合
	at.updateHotAndColdStatus(blockKey, record)
}

// UpdateLocation 更新块的存储位置
func (at *AccessTracker) UpdateLocation(blockKey string, location StorageLocation) {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	record, exists := at.records[blockKey]
	if exists {
		record.CurrentLocation = location
	}
}

// GetBlockAccessRecord 获取块的访问记录
func (at *AccessTracker) GetBlockAccessRecord(blockKey string) *BlockAccessRecord {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	record, exists := at.records[blockKey]
	if exists {
		// 返回副本以避免并发访问问题
		return &BlockAccessRecord{
			BlockKey:        record.BlockKey,
			AccessCount:     record.AccessCount,
			LastAccessTime:  record.LastAccessTime,
			FirstAccessTime: record.FirstAccessTime,
			Size:            record.Size,
			CurrentLocation: record.CurrentLocation,
		}
	}

	return nil
}

// GetHotBlocks 获取所有热点块的键
func (at *AccessTracker) GetHotBlocks() []string {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	result := make([]string, 0, len(at.hotBlocks))
	for key := range at.hotBlocks {
		result = append(result, key)
	}

	return result
}

// GetColdBlocks 获取所有冷块的键
func (at *AccessTracker) GetColdBlocks() []string {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	result := make([]string, 0, len(at.coldBlocks))
	for key := range at.coldBlocks {
		result = append(result, key)
	}

	return result
}

// updateHotAndColdStatus 更新块的热点和冷块状态
func (at *AccessTracker) updateHotAndColdStatus(blockKey string, record *BlockAccessRecord) {
	// 热点块判断
	if record.IsHot(at.config.HotBlockThreshold) {
		at.hotBlocks[blockKey] = struct{}{}
	} else {
		delete(at.hotBlocks, blockKey)
	}

	// 冷块判断
	if record.IsCold(at.config.ColdBlockTimeMinutes) {
		at.coldBlocks[blockKey] = struct{}{}
	} else {
		delete(at.coldBlocks, blockKey)
	}
}

// CleanupRecords 清理访问记录
// 当记录数超过maxRecords时，保留最重要的keepRecords条记录
func (at *AccessTracker) CleanupRecords(maxRecords, keepRecords int) {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	if len(at.records) <= maxRecords {
		return
	}

	// 创建记录评分列表
	type scoredRecord struct {
		key   string
		score float64
	}

	scoredRecords := make([]scoredRecord, 0, len(at.records))
	for key, record := range at.records {
		scoredRecords = append(scoredRecords, scoredRecord{
			key:   key,
			score: record.GetScore(),
		})
	}

	// 按评分降序排序
	sort.Slice(scoredRecords, func(i, j int) bool {
		return scoredRecords[i].score > scoredRecords[j].score
	})

	// 保留评分最高的keepRecords条记录
	newRecords := make(map[string]*BlockAccessRecord, keepRecords)
	for i := 0; i < keepRecords && i < len(scoredRecords); i++ {
		key := scoredRecords[i].key
		newRecords[key] = at.records[key]
	}

	// 使用新记录替换旧记录
	at.records = newRecords

	// 重新构建热点块和冷块集合
	at.rebuildHotAndColdSets()
}

// rebuildHotAndColdSets 重建热点和冷块集合
func (at *AccessTracker) rebuildHotAndColdSets() {
	at.hotBlocks = make(map[string]struct{})
	at.coldBlocks = make(map[string]struct{})

	for key, record := range at.records {
		if record.IsHot(at.config.HotBlockThreshold) {
			at.hotBlocks[key] = struct{}{}
		}

		if record.IsCold(at.config.ColdBlockTimeMinutes) {
			at.coldBlocks[key] = struct{}{}
		}
	}
}

// CleanupColdBlocks 清理冷块记录中不再是冷块的记录
func (at *AccessTracker) CleanupColdBlocks() {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	for key := range at.coldBlocks {
		record, exists := at.records[key]
		if !exists || !record.IsCold(at.config.ColdBlockTimeMinutes) {
			delete(at.coldBlocks, key)
		}
	}
}

// DistributionAnalysis 存储分布分析结果
type DistributionAnalysis struct {
	// TotalBlocks 总块数
	TotalBlocks int
	// InlineBlocks 内联块数量
	InlineBlocks int
	// ContainerBlocks 容器块数量
	ContainerBlocks int
	// DirectoryBlocks 目录块数量
	DirectoryBlocks int
	// PercentageInline 内联块百分比
	PercentageInline float64
	// PercentageContainer 容器块百分比
	PercentageContainer float64
	// PercentageDirectory 目录块百分比
	PercentageDirectory float64
	// StorageEfficiency 存储效率评分(0-1.0)
	StorageEfficiency float64
	// PerformanceScore 性能评分(0-100)
	PerformanceScore float64
	// Recommendations 优化建议列表
	Recommendations []string
}

// StorageStrategy 存储策略接口
type StorageStrategy interface {
	// DecideLocation 决定块的存储位置
	DecideLocation(blockKey string, size int64, accessRecord *BlockAccessRecord) StorageDecision

	// AnalyzeDistribution 分析存储分布情况
	AnalyzeDistribution(tracker *AccessTracker) *DistributionAnalysis

	// Name 返回策略名称
	Name() string
}

// SimpleThresholdStrategy 简单阈值策略
type SimpleThresholdStrategy struct {
	config *StrategyConfig
}

// NewSimpleThresholdStrategy 创建一个新的简单阈值策略
func NewSimpleThresholdStrategy(config *StrategyConfig) *SimpleThresholdStrategy {
	return &SimpleThresholdStrategy{
		config: config,
	}
}

// DecideLocation 决定块的存储位置
func (s *SimpleThresholdStrategy) DecideLocation(blockKey string, size int64, accessRecord *BlockAccessRecord) StorageDecision {
	// 简单策略：仅根据大小决定位置
	if size <= s.config.InlineThreshold {
		return StorageDecision{
			Location: LocationInline,
			Reason:   fmt.Sprintf("Block size %d <= inline threshold %d", size, s.config.InlineThreshold),
			Score:    1.0,
		}
	} else {
		// 默认使用目录存储
		return StorageDecision{
			Location: LocationDirectory,
			Reason:   fmt.Sprintf("Block size %d > inline threshold %d", size, s.config.InlineThreshold),
			Score:    1.0,
		}
	}
}

// AnalyzeDistribution 分析存储分布情况
func (s *SimpleThresholdStrategy) AnalyzeDistribution(tracker *AccessTracker) *DistributionAnalysis {
	tracker.mutex.RLock()
	defer tracker.mutex.RUnlock()

	analysis := &DistributionAnalysis{
		TotalBlocks: len(tracker.records),
	}

	// 统计各存储位置的块数量
	for _, record := range tracker.records {
		switch record.CurrentLocation {
		case LocationInline:
			analysis.InlineBlocks++
		case LocationContainer:
			analysis.ContainerBlocks++
		case LocationDirectory:
			analysis.DirectoryBlocks++
		}
	}

	// 计算各位置的百分比
	if analysis.TotalBlocks > 0 {
		analysis.PercentageInline = float64(analysis.InlineBlocks) / float64(analysis.TotalBlocks)
		analysis.PercentageContainer = float64(analysis.ContainerBlocks) / float64(analysis.TotalBlocks)
		analysis.PercentageDirectory = float64(analysis.DirectoryBlocks) / float64(analysis.TotalBlocks)
	}

	// 简单策略的存储效率：内联块比例越接近理论最佳值越好
	// 理论最佳值：假设所有块的大小都符合理想分布
	// 如果内联阈值为1KB，我们期望有大约20%的块是内联存储的
	expectedInlinePercentage := 0.2 // 这个值可以根据实际情况调整
	analysis.StorageEfficiency = 1.0 - abs(analysis.PercentageInline-expectedInlinePercentage)

	// 性能评分：简单策略主要考虑存储分布的平衡性
	analysis.PerformanceScore = analysis.StorageEfficiency * 100

	// 生成优化建议
	analysis.Recommendations = s.generateRecommendations(analysis)

	return analysis
}

// generateRecommendations 生成优化建议
func (s *SimpleThresholdStrategy) generateRecommendations(analysis *DistributionAnalysis) []string {
	recommendations := []string{}

	// 如果内联块比例过高，建议降低内联阈值
	if analysis.PercentageInline > 0.3 {
		recommendations = append(recommendations,
			fmt.Sprintf("内联块比例过高(%.1f%%)，建议将内联阈值从%d字节降低",
				analysis.PercentageInline*100, s.config.InlineThreshold))
	}

	// 如果内联块比例过低，建议提高内联阈值
	if analysis.PercentageInline < 0.1 && analysis.TotalBlocks > 20 {
		recommendations = append(recommendations,
			fmt.Sprintf("内联块比例过低(%.1f%%)，建议将内联阈值从%d字节提高",
				analysis.PercentageInline*100, s.config.InlineThreshold))
	}

	// 如果大部分块都是大型块，建议优化空间使用
	if analysis.PercentageDirectory > 0.8 {
		recommendations = append(recommendations,
			"大部分是目录存储的块，建议开启压缩或考虑块分片以提高存储效率")
	}

	return recommendations
}

// Name 返回策略名称
func (s *SimpleThresholdStrategy) Name() string {
	return "simple"
}

// AdaptiveStrategy 自适应策略
type AdaptiveStrategy struct {
	config *StrategyConfig
}

// NewAdaptiveStrategy 创建一个新的自适应策略
func NewAdaptiveStrategy(config *StrategyConfig) *AdaptiveStrategy {
	return &AdaptiveStrategy{
		config: config,
	}
}

// DecideLocation 决定块的存储位置
func (a *AdaptiveStrategy) DecideLocation(blockKey string, size int64, accessRecord *BlockAccessRecord) StorageDecision {
	// 1. 首先基于块大小进行初步决策
	if size <= a.config.InlineThreshold {
		return StorageDecision{
			Location: LocationInline,
			Reason:   fmt.Sprintf("小块(size=%d)适合内联存储", size),
			Score:    1.0,
		}
	}

	// 如果没有访问记录或者是新块，基于大小决定
	if accessRecord == nil {
		// 大块（>1MB）默认放在目录存储中
		if size > 1024*1024 {
			return StorageDecision{
				Location: LocationDirectory,
				Reason:   "大块(>1MB)放入目录存储",
				Score:    0.8,
			}
		}

		// 中等大小的块默认放在容器存储中
		return StorageDecision{
			Location: LocationContainer,
			Reason:   "中等大小块放入容器存储",
			Score:    0.8,
		}
	}

	// 2. 基于访问频率的优化
	// 热点块倾向于放入容器存储
	if accessRecord.IsHot(a.config.HotBlockThreshold) {
		return StorageDecision{
			Location: LocationContainer,
			Reason:   fmt.Sprintf("热点块(访问次数=%d)放入容器存储", accessRecord.AccessCount),
			Score:    0.9,
		}
	}

	// 冷块倾向于放入目录存储
	if accessRecord.IsCold(a.config.ColdBlockTimeMinutes) {
		return StorageDecision{
			Location: LocationDirectory,
			Reason:   fmt.Sprintf("冷块(最后访问时间=%v)放入目录存储", accessRecord.LastAccessTime),
			Score:    0.9,
		}
	}

	// 3. 根据性能目标进行额外调整
	switch a.config.PerformanceTarget {
	case TargetSpeed:
		// 优先考虑速度：更多地使用容器存储
		return StorageDecision{
			Location: LocationContainer,
			Reason:   "以速度为目标，使用容器存储",
			Score:    0.7,
		}
	case TargetSpace:
		// 优先考虑空间：更多地使用目录存储
		return StorageDecision{
			Location: LocationDirectory,
			Reason:   "以空间效率为目标，使用目录存储",
			Score:    0.7,
		}
	default: // TargetBalanced
		// 均衡模式：中等大小的块放在容器存储，大块放在目录存储
		if size > 512*1024 { // 512KB
			return StorageDecision{
				Location: LocationDirectory,
				Reason:   "均衡模式下大块(>512KB)放入目录存储",
				Score:    0.7,
			}
		}
		return StorageDecision{
			Location: LocationContainer,
			Reason:   "均衡模式下中等块放入容器存储",
			Score:    0.7,
		}
	}
}

// AnalyzeDistribution 分析存储分布情况
func (a *AdaptiveStrategy) AnalyzeDistribution(tracker *AccessTracker) *DistributionAnalysis {
	tracker.mutex.RLock()
	defer tracker.mutex.RUnlock()

	analysis := &DistributionAnalysis{
		TotalBlocks: len(tracker.records),
	}

	// 热块和冷块计数
	hotBlocks := len(tracker.hotBlocks)
	coldBlocks := len(tracker.coldBlocks)

	// 统计各存储位置的块数量
	countByLocation := make(map[StorageLocation]int)
	countByLocationHot := make(map[StorageLocation]int)
	countByLocationCold := make(map[StorageLocation]int)

	for key, record := range tracker.records {
		countByLocation[record.CurrentLocation]++

		_, isHot := tracker.hotBlocks[key]
		if isHot {
			countByLocationHot[record.CurrentLocation]++
		}

		_, isCold := tracker.coldBlocks[key]
		if isCold {
			countByLocationCold[record.CurrentLocation]++
		}
	}

	// 填充分析结果
	analysis.InlineBlocks = countByLocation[LocationInline]
	analysis.ContainerBlocks = countByLocation[LocationContainer]
	analysis.DirectoryBlocks = countByLocation[LocationDirectory]

	// 计算各位置的百分比
	if analysis.TotalBlocks > 0 {
		analysis.PercentageInline = float64(analysis.InlineBlocks) / float64(analysis.TotalBlocks)
		analysis.PercentageContainer = float64(analysis.ContainerBlocks) / float64(analysis.TotalBlocks)
		analysis.PercentageDirectory = float64(analysis.DirectoryBlocks) / float64(analysis.TotalBlocks)
	}

	// 计算存储效率
	// 1. 热块应该主要在容器存储中
	hotBlockEfficiency := 1.0
	if hotBlocks > 0 {
		hotBlockEfficiency = float64(countByLocationHot[LocationContainer]) / float64(hotBlocks)
	}

	// 2. 冷块应该主要在目录存储中
	coldBlockEfficiency := 1.0
	if coldBlocks > 0 {
		coldBlockEfficiency = float64(countByLocationCold[LocationDirectory]) / float64(coldBlocks)
	}

	// 3. 内联块比例应该在一个合理范围内
	inlineEfficiency := 1.0 - abs(analysis.PercentageInline-0.2) // 假设20%是理想值

	// 综合计算存储效率
	analysis.StorageEfficiency = (hotBlockEfficiency*0.4 + coldBlockEfficiency*0.3 + inlineEfficiency*0.3)

	// 计算性能评分
	// 不同性能目标下的评分权重不同
	var speedWeight, spaceWeight float64

	switch a.config.PerformanceTarget {
	case TargetSpeed:
		speedWeight = 0.7
		spaceWeight = 0.3
	case TargetSpace:
		speedWeight = 0.3
		spaceWeight = 0.7
	default: // TargetBalanced
		speedWeight = 0.5
		spaceWeight = 0.5
	}

	// 速度评分主要看热块在容器存储的比例
	speedScore := hotBlockEfficiency

	// 空间评分主要看冷块在目录存储的比例和总体存储分布
	spaceScore := (coldBlockEfficiency*0.7 + inlineEfficiency*0.3)

	// 综合评分
	analysis.PerformanceScore = (speedScore*speedWeight + spaceScore*spaceWeight) * 100

	// 生成优化建议
	analysis.Recommendations = a.generateRecommendations(analysis, hotBlocks, coldBlocks,
		countByLocationHot, countByLocationCold)

	return analysis
}

// generateRecommendations 生成优化建议
func (a *AdaptiveStrategy) generateRecommendations(
	analysis *DistributionAnalysis,
	hotBlocks, coldBlocks int,
	countByLocationHot, countByLocationCold map[StorageLocation]int) []string {

	recommendations := []string{}

	// 检查热块分布
	if hotBlocks > 0 && float64(countByLocationHot[LocationContainer])/float64(hotBlocks) < 0.6 {
		recommendations = append(recommendations,
			"热点块应该更多地放在容器存储中，建议检查热块判定逻辑")
	}

	// 检查冷块分布
	if coldBlocks > 0 && float64(countByLocationCold[LocationDirectory])/float64(coldBlocks) < 0.6 {
		recommendations = append(recommendations,
			"冷块应该更多地放在目录存储中，建议检查冷块判定逻辑")
	}

	// 内联块比例检查
	if analysis.PercentageInline > 0.3 {
		recommendations = append(recommendations,
			fmt.Sprintf("内联块比例过高(%.1f%%)，建议降低内联阈值",
				analysis.PercentageInline*100))
	} else if analysis.PercentageInline < 0.1 && analysis.TotalBlocks > 50 {
		recommendations = append(recommendations,
			fmt.Sprintf("内联块比例过低(%.1f%%)，建议提高内联阈值",
				analysis.PercentageInline*100))
	}

	// 如果存储效率低于阈值，建议进行优化
	if analysis.StorageEfficiency < 0.6 {
		recommendations = append(recommendations,
			fmt.Sprintf("存储效率较低(%.2f)，建议执行优化操作",
				analysis.StorageEfficiency))
	}

	// 根据性能目标给出针对性建议
	switch a.config.PerformanceTarget {
	case TargetSpeed:
		if analysis.PercentageContainer < 0.5 && analysis.TotalBlocks > 20 {
			recommendations = append(recommendations,
				"以速度为目标，建议增加容器存储的使用比例")
		}
	case TargetSpace:
		if analysis.PercentageDirectory < 0.5 && analysis.TotalBlocks > 20 {
			recommendations = append(recommendations,
				"以空间为目标，建议增加目录存储的使用比例")
		}
	}

	return recommendations
}

// Name 返回策略名称
func (a *AdaptiveStrategy) Name() string {
	return "adaptive"
}

// StorageStrategyFactory 存储策略工厂
type StorageStrategyFactory struct{}

// CreateStrategy 创建存储策略
func (f *StorageStrategyFactory) CreateStrategy(config *StrategyConfig) (StorageStrategy, error) {
	switch config.StrategyName {
	case "simple":
		return NewSimpleThresholdStrategy(config), nil
	case "adaptive":
		return NewAdaptiveStrategy(config), nil
	default:
		return nil, fmt.Errorf("unsupported strategy name: %s", config.StrategyName)
	}
}

// 辅助函数：计算绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
