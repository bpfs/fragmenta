package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/bpfs/fragmenta/storage"
)

// 生成随机数据
func generateRandomData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

// 打印混合存储统计信息
func printHybridStats(stats *storage.HybridStorageExtendedStats) {
	fmt.Println("===== 混合存储统计信息 =====")
	fmt.Printf("总块数: %d\n", stats.StorageStats.TotalBlocks)
	fmt.Printf("总大小: %.2f MB\n", float64(stats.StorageStats.TotalSize)/(1024*1024))
	fmt.Printf("内联块: %d (%.1f%%)\n", stats.InlineBlockCount,
		float64(stats.InlineBlockCount)*100/float64(stats.StorageStats.TotalBlocks))
	fmt.Printf("容器块: %d (%.1f%%)\n", stats.ContainerBlockCount,
		float64(stats.ContainerBlockCount)*100/float64(stats.StorageStats.TotalBlocks))
	fmt.Printf("目录块: %d (%.1f%%)\n", stats.DirectoryBlockCount,
		float64(stats.DirectoryBlockCount)*100/float64(stats.StorageStats.TotalBlocks))
	// 以下属性当前版本不可用，暂时注释
	// fmt.Printf("热块数: %d\n", stats.HotBlockCount)
	// fmt.Printf("冷块数: %d\n", stats.ColdBlockCount)
	// fmt.Printf("存储效率: %.2f\n", stats.StorageEfficiency)
	// fmt.Printf("性能评分: %.1f\n", stats.PerformanceScore)
	fmt.Println("=============================")
}

// 打印性能指标
func printPerformanceMetrics(metrics *storage.HybridStoragePerformanceMetrics) {
	fmt.Println("===== 性能指标 =====")
	fmt.Printf("读取次数: %d\n", metrics.ReadCount)
	fmt.Printf("平均读取延迟: %.2f ms\n", float64(metrics.AvgReadLatency)/float64(time.Millisecond))
	fmt.Printf("最小读取延迟: %.2f ms\n", float64(metrics.MinReadLatency)/float64(time.Millisecond))
	fmt.Printf("最大读取延迟: %.2f ms\n", float64(metrics.MaxReadLatency)/float64(time.Millisecond))

	fmt.Printf("写入次数: %d\n", metrics.WriteCount)
	fmt.Printf("平均写入延迟: %.2f ms\n", float64(metrics.AvgWriteLatency)/float64(time.Millisecond))
	fmt.Printf("最小写入延迟: %.2f ms\n", float64(metrics.MinWriteLatency)/float64(time.Millisecond))
	fmt.Printf("最大写入延迟: %.2f ms\n", float64(metrics.MaxWriteLatency)/float64(time.Millisecond))

	fmt.Printf("缓存命中率: %.1f%%\n", metrics.GetCacheHitRate()*100)
	// 当前版本不支持策略命中率
	// fmt.Printf("策略命中率: %.1f%%\n", metrics.GetStrategyHitRate()*100)
	fmt.Println("=====================")
}

/*
// 当前HybridStorage未实现该功能，临时注释
// 打印存储分布分析
func printDistributionAnalysis(analysis *storage.DistributionAnalysis) {
	fmt.Println("===== 存储分布分析 =====")
	fmt.Printf("内联块比例: %.1f%%\n", analysis.PercentageInline*100)
	fmt.Printf("容器块比例: %.1f%%\n", analysis.PercentageContainer*100)
	fmt.Printf("目录块比例: %.1f%%\n", analysis.PercentageDirectory*100)
	fmt.Printf("存储效率: %.2f\n", analysis.StorageEfficiency)
	fmt.Printf("性能评分: %.1f\n", analysis.PerformanceScore)

	fmt.Println("优化建议:")
	for _, rec := range analysis.Recommendations {
		fmt.Printf("- %s\n", rec)
	}
	fmt.Println("=========================")
}
*/

func main() {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "hybrid-storage-test")
	if err != nil {
		log.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("使用临时目录: %s\n", tempDir)

	// 配置混合存储
	config := &storage.StorageConfig{
		Type:            storage.StorageTypeHybrid,
		Path:            tempDir,
		BlockSize:       4096,
		InlineThreshold: 1024,             // 1KB
		CacheSize:       10 * 1024 * 1024, // 10MB
		CachePolicy:     "lru",
		DedupEnabled:    true,
		// 以下配置在当前版本可能不支持，如不需要可注释
		StrategyName:               "adaptive",
		EnableStrategyOptimization: true,
		HotBlockThreshold:          5,
		ColdBlockTimeMinutes:       1, // 1分钟，仅为了演示
		PerformanceTarget:          "balanced",
		AutoBalanceEnabled:         true,
	}

	// 创建混合存储
	fmt.Println("初始化混合存储...")
	hybridStorage, err := storage.NewHybridStorage(config)
	if err != nil {
		log.Fatalf("创建混合存储失败: %v", err)
	}

	// 写入不同大小的数据块
	fmt.Println("写入数据块...")
	blockCount := 100
	blockKeys := make([]string, 0, blockCount)

	// 小块（内联）
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("small_block_%d", i)
		data := generateRandomData(500) // 500B

		if err := hybridStorage.WriteBlock(key, data); err != nil {
			log.Fatalf("写入小块失败: %v", err)
		}

		blockKeys = append(blockKeys, key)
	}

	// 中等块（容器）
	for i := 0; i < 40; i++ {
		key := fmt.Sprintf("medium_block_%d", i)
		data := generateRandomData(10 * 1024) // 10KB

		if err := hybridStorage.WriteBlock(key, data); err != nil {
			log.Fatalf("写入中等块失败: %v", err)
		}

		blockKeys = append(blockKeys, key)
	}

	// 大块（目录）
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("large_block_%d", i)
		data := generateRandomData(2 * 1024 * 1024) // 2MB

		if err := hybridStorage.WriteBlock(key, data); err != nil {
			log.Fatalf("写入大块失败: %v", err)
		}

		blockKeys = append(blockKeys, key)
	}

	// 打印初始统计信息
	fmt.Println("\n初始状态:")
	printHybridStats(hybridStorage.GetHybridStats())

	// 模拟频繁访问某些块，使其成为热点块
	fmt.Println("\n模拟频繁访问，创建热点块...")
	for i := 0; i < 20; i++ {
		// 随机选择10个块重复读取
		hotBlockIndices := rand.Perm(len(blockKeys))[:10]

		for _, idx := range hotBlockIndices {
			key := blockKeys[idx]
			// 每个热点块读取7次
			for j := 0; j < 7; j++ {
				data, err := hybridStorage.ReadBlock(key)
				if err != nil {
					log.Fatalf("读取块失败: %v", err)
				}
				fmt.Printf("读取块 %s: %d 字节\n", key, len(data))
			}
		}
	}

	// 打印性能指标
	fmt.Println("\n访问后性能指标:")
	printPerformanceMetrics(hybridStorage.GetPerformanceMetrics())

	// 打印热点访问后的统计信息
	fmt.Println("\n热点访问后统计信息:")
	printHybridStats(hybridStorage.GetHybridStats())

	// 手动触发优化
	fmt.Println("\n触发存储优化...")
	err = hybridStorage.Optimize()
	if err != nil {
		log.Fatalf("存储优化失败: %v", err)
	}

	/* 当前版本不支持存储分布分析，临时注释
	// 打印优化后的存储分布分析
	fmt.Println("\n优化后的存储分布分析:")
	printDistributionAnalysis(hybridStorage.GetStorageDistributionAnalysis())
	*/

	// 打印优化后的统计信息
	fmt.Println("\n优化后统计信息:")
	printHybridStats(hybridStorage.GetHybridStats())

	// 删除部分块
	fmt.Println("\n删除部分块...")
	for i := 0; i < 20; i++ {
		idx := rand.Intn(len(blockKeys))
		key := blockKeys[idx]

		if err := hybridStorage.DeleteBlock(key); err != nil {
			if err != storage.ErrBlockNotFound {
				log.Fatalf("删除块失败: %v", err)
			}
		} else {
			fmt.Printf("删除块 %s 成功\n", key)
		}
	}

	// 等待一段时间，让一些块变成冷块
	fmt.Println("\n等待使部分块变成冷块...")
	time.Sleep(2 * time.Minute)

	// 再次触发优化
	fmt.Println("\n再次触发存储优化...")
	err = hybridStorage.Optimize()
	if err != nil {
		log.Fatalf("存储优化失败: %v", err)
	}

	// 打印最终统计信息
	fmt.Println("\n最终统计信息:")
	printHybridStats(hybridStorage.GetHybridStats())
	printPerformanceMetrics(hybridStorage.GetPerformanceMetrics())

	/* 当前版本不支持存储分布分析，临时注释
	printDistributionAnalysis(hybridStorage.GetStorageDistributionAnalysis())
	*/

	fmt.Println("\n混合存储测试完成!")
}
