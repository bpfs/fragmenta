package storage

import (
	"os"
	"testing"
)

// TestHybridStorage 测试混合存储功能
func TestHybridStorage(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "hybrid_storage_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	config := &StorageConfig{
		Type:                       StorageTypeHybrid,
		Path:                       tempDir,
		AutoConvertThreshold:       1024 * 1024 * 10, // 10MB
		BlockSize:                  4096,
		InlineThreshold:            1024, // 1KB
		DedupEnabled:               true,
		CacheSize:                  1024 * 1024, // 1MB
		CachePolicy:                "lru",
		StrategyName:               "adaptive",
		EnableStrategyOptimization: true,
		HotBlockThreshold:          5,
		ColdBlockTimeMinutes:       30,
		PerformanceTarget:          "balanced",
		AutoBalanceEnabled:         true,
	}

	// 初始化混合存储
	hs, err := NewHybridStorage(config)
	if err != nil {
		t.Fatalf("初始化混合存储失败: %v", err)
	}

	// 测试写入小块（应该使用内联存储）
	smallData := make([]byte, 512)
	for i := range smallData {
		smallData[i] = byte(i % 256)
	}

	if err := hs.WriteBlock("block1", smallData); err != nil {
		t.Fatalf("写入小块失败: %v", err)
	}

	// 测试写入大块（应该使用目录存储）
	largeData := make([]byte, 1024*1024*2) // 2MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	if err := hs.WriteBlock("block2", largeData); err != nil {
		t.Fatalf("写入大块失败: %v", err)
	}

	// 测试读取块
	readData, err := hs.ReadBlock("block1")
	if err != nil {
		t.Fatalf("读取小块失败: %v", err)
	}

	// 验证数据完整性
	if len(readData) != len(smallData) {
		t.Errorf("读取的数据大小不匹配: 期望 %d, 实际 %d", len(smallData), len(readData))
	}

	for i := range smallData {
		if readData[i] != smallData[i] {
			t.Errorf("数据内容不匹配: 位置 %d, 期望 %d, 实际 %d", i, smallData[i], readData[i])
			break
		}
	}

	// 测试更新块（小块变大块，应该从内联移到目录）
	newLargeData := make([]byte, 1024*1024) // 1MB
	for i := range newLargeData {
		newLargeData[i] = byte((i + 1) % 256)
	}

	if err := hs.WriteBlock("block1", newLargeData); err != nil {
		t.Fatalf("更新小块为大块失败: %v", err)
	}

	// 计算存储分布
	hybridStats := hs.GetHybridStats()

	if hybridStats.DirectoryBlockCount != 1 {
		t.Errorf("目录块数统计不正确: 期望 1, 实际 %d", hybridStats.DirectoryBlockCount)
	}

	if hybridStats.InlineBlockCount != 0 {
		t.Errorf("内联块数统计不正确: 期望 0, 实际 %d", hybridStats.InlineBlockCount)
	}

	// 测试访问记录和热点块识别
	// 模拟多次访问以使其成为热点
	for i := 0; i < 10; i++ {
		_, err := hs.ReadBlock("block2")
		if err != nil {
			t.Fatalf("读取块失败: %v", err)
		}
	}

	// 优化存储，这应该触发块迁移
	if err := hs.Optimize(); err != nil {
		t.Fatalf("优化存储失败: %v", err)
	}

	// 测试删除块
	if err := hs.DeleteBlock("block1"); err != nil {
		t.Fatalf("删除块失败: %v", err)
	}

	// 确认已删除
	_, err = hs.ReadBlock("block1")
	if err == nil {
		t.Errorf("块1应已被删除，但仍可读取")
	}

	// 测试性能指标
	metrics := hs.GetPerformanceMetrics()
	if metrics.ReadCount < 10 {
		t.Errorf("读取计数不正确: 期望 >= 10, 实际 %d", metrics.ReadCount)
	}

	if metrics.WriteCount < 2 {
		t.Errorf("写入计数不正确: 期望 >= 2, 实际 %d", metrics.WriteCount)
	}
}
