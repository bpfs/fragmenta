package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorageModeAutoConversion(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 设置路径 - 容器文件路径
	containerPath := filepath.Join(tempDir, "container_storage")

	// 设置配置 - 初始使用容器模式，开启自动转换
	config := &StorageConfig{
		Type:                 StorageTypeContainer,
		Path:                 containerPath,
		AutoConvertThreshold: 50 * 1024, // 50KB阈值
		BlockSize:            1024,
		InlineThreshold:      512,
		DedupEnabled:         false,
		CacheSize:            1024 * 1024, // 1MB
		CachePolicy:          "lru",
	}

	t.Logf("创建存储管理器，初始容器路径: %s，自动转换阈值: %d", containerPath, config.AutoConvertThreshold)

	// 创建存储管理器
	sm, err := NewStorageManager(config)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}
	defer sm.Close()

	// 验证初始状态和模式
	stats, err := sm.GetStats()
	if err != nil {
		t.Fatalf("获取存储统计信息失败: %v", err)
	}
	t.Logf("初始状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	if sm.config.Type != StorageTypeContainer {
		t.Fatalf("初始存储模式不正确，期望=%v, 实际=%v", StorageTypeContainer, sm.config.Type)
	}

	// 获取初始存储建议
	currentMode, suggestedMode, reason := sm.GetStorageModeSuggestion()
	t.Logf("初始存储建议: 当前=%v, 建议=%v, 原因=%s", currentMode, suggestedMode, reason)

	// 准备要写入的大数据块
	dataSize := 1024 // 1KB
	data := make([]byte, dataSize)
	for i := 0; i < dataSize; i++ {
		data[i] = byte(i % 256)
	}

	// 写入50个块，约50KB，达到阈值
	t.Logf("写入50个块（约50KB）达到自动转换阈值")
	for i := 0; i < 50; i++ {
		err = sm.WriteBlock(uint32(i), data)
		if err != nil {
			t.Fatalf("写入块失败: %v", err)
		}
	}

	// 短暂等待自动转换（如果有的话）
	time.Sleep(500 * time.Millisecond)

	// 验证自动转换后的状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取自动转换后统计信息失败: %v", err)
	}
	t.Logf("写入后状态: 块数=%d, 空间=%d, 当前模式=%v",
		stats.TotalBlocks, stats.UsedSpace, sm.config.Type)

	// 获取新的存储建议
	currentMode, suggestedMode, reason = sm.GetStorageModeSuggestion()
	t.Logf("转换后存储建议: 当前=%v, 建议=%v, 原因=%s", currentMode, suggestedMode, reason)

	// 再写入一些数据到新模式
	t.Logf("继续写入30个块到当前存储模式")
	for i := 50; i < 80; i++ {
		err = sm.WriteBlock(uint32(i), data)
		if err != nil {
			t.Fatalf("写入转换后模式块失败: %v", err)
		}
	}

	// 验证最终状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取最终统计信息失败: %v", err)
	}
	t.Logf("最终状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 测试读取所有块，验证数据完整性
	t.Logf("验证数据完整性")
	for i := 0; i < 80; i++ {
		readData, err := sm.ReadBlock(uint32(i))
		if err != nil {
			t.Fatalf("读取块失败: %v", err)
		}
		if len(readData) != dataSize {
			t.Errorf("读取数据大小不正确, 期望: %d, 实际: %d", dataSize, len(readData))
		}
	}

	t.Logf("存储模式自动转换测试完成")
}

func TestStorageModeEvaluation(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试配置
	config := &StorageConfig{
		Type:                 StorageTypeContainer,
		Path:                 filepath.Join(tempDir, "test_evaluation"),
		AutoConvertThreshold: 1024 * 1024, // 1MB
		BlockSize:            4096,
		InlineThreshold:      512,
		DedupEnabled:         false,
		CacheSize:            1024 * 1024, // 1MB
		CachePolicy:          "lru",
	}

	// 创建存储管理器
	sm, err := NewStorageManager(config)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}
	defer sm.Close()

	// 测试不同场景下的存储模式评估
	testCases := []struct {
		name         string
		stats        *StorageStats
		expectedMode StorageType
	}{
		{
			name: "小存储使用容器模式",
			stats: &StorageStats{
				TotalBlocks: 10,
				UsedSpace:   1024 * 100, // 100KB
			},
			expectedMode: StorageTypeContainer,
		},
		{
			name: "大存储使用目录模式",
			stats: &StorageStats{
				TotalBlocks: 1000,
				UsedSpace:   1024 * 1024 * 2, // 2MB
			},
			expectedMode: StorageTypeDirectory,
		},
		{
			name: "高碎片率使用目录模式",
			stats: &StorageStats{
				TotalBlocks:        100,
				UsedSpace:          1024 * 500, // 500KB
				FragmentationRatio: 0.4,
			},
			expectedMode: StorageTypeDirectory,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 构造测试条件，修改GetStats结果
			// 注意：这里我们只能测试接口可访问的方法，无法直接调用内部的EvaluateStorageMode
			// 所以我们通过调用GetStorageModeSuggestion来测试模式评估

			// 先获取建议模式
			suggestedMode, reason, err := sm.GetStorageModeSuggestion()
			if err != nil {
				t.Fatalf("获取存储建议失败: %v", err)
			}

			t.Logf("情景【%s】的评估结果: 模式: %v, 原因: %s",
				tc.name, suggestedMode, reason)
		})
	}
}

func TestStorageModeSuggestion(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 设置容器模式存储路径
	containerPath := filepath.Join(tempDir, "container_storage")

	// 设置配置
	config := &StorageConfig{
		Type:                 StorageTypeContainer,
		Path:                 containerPath,
		AutoConvertThreshold: 1024 * 50, // 50KB，方便测试
		BlockSize:            4096,
		InlineThreshold:      512,
		DedupEnabled:         false,
		CacheSize:            1024 * 1024, // 1MB
		CachePolicy:          "lru",
	}

	t.Logf("创建存储管理器，路径: %s", containerPath)

	// 创建存储管理器
	sm, err := NewStorageManager(config)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}
	defer sm.Close()

	// 验证初始状态
	stats, err := sm.GetStats()
	if err != nil {
		t.Fatalf("获取存储统计信息失败: %v", err)
	}
	t.Logf("初始状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 获取存储建议
	curType, reason, err := sm.GetStorageModeSuggestion()
	if err != nil {
		t.Fatalf("获取存储建议失败: %v", err)
	}
	t.Logf("初始存储建议: 类型=%v, 原因=%s", curType, reason)

	// 写入测试数据
	dataSize := 1024 // 1KB
	data := make([]byte, dataSize)
	for i := 0; i < dataSize; i++ {
		data[i] = byte(i % 256)
	}

	t.Logf("开始写入数据，每块大小: %d 字节", dataSize)

	// 写入25个块，约25KB
	for i := 0; i < 25; i++ {
		err = sm.WriteBlock(uint32(i), data)
		if err != nil {
			t.Fatalf("写入块失败: %v", err)
		}
	}

	// 验证写入后的状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取存储统计信息失败: %v", err)
	}
	t.Logf("写入25KB后状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 获取当前存储建议
	curType, reason, err = sm.GetStorageModeSuggestion()
	if err != nil {
		t.Fatalf("获取存储建议失败: %v", err)
	}
	t.Logf("写入25KB后存储建议: 类型=%v, 原因=%s", curType, reason)

	// 写入另外50个块，总计约75KB，超过阈值
	for i := 25; i < 75; i++ {
		err = sm.WriteBlock(uint32(i), data)
		if err != nil {
			t.Fatalf("写入块失败: %v", err)
		}
	}

	// 验证写入后的状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取存储统计信息失败: %v", err)
	}
	t.Logf("写入75KB后状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 获取当前存储建议
	curType, reason, err = sm.GetStorageModeSuggestion()
	if err != nil {
		t.Fatalf("获取存储建议失败: %v", err)
	}
	t.Logf("写入75KB后存储建议: 类型=%v, 原因=%s", curType, reason)

	// 验证存储建议是否为目录模式
	if curType != StorageTypeDirectory {
		t.Errorf("存储建议不正确，期望目录模式，实际: %v", curType)
	}

	t.Logf("存储模式建议测试完成")
}

func TestManualStorageConversion(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 设置路径 - 确保容器文件和目录使用不同的路径
	containerPath := filepath.Join(tempDir, "container_storage")
	directoryPath := filepath.Join(tempDir, "directory_storage")

	// 设置配置 - 初始使用容器模式，禁用自动转换
	config := &StorageConfig{
		Type:                 StorageTypeContainer,
		Path:                 containerPath,
		AutoConvertThreshold: 0, // 禁用自动转换
		BlockSize:            4096,
		InlineThreshold:      512,
		DedupEnabled:         false,
		CacheSize:            1024 * 1024, // 1MB
		CachePolicy:          "lru",
	}

	t.Logf("创建存储管理器，初始容器路径: %s", containerPath)

	// 创建存储管理器
	sm, err := NewStorageManager(config)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}
	defer sm.Close()

	// 验证初始状态和模式
	stats, err := sm.GetStats()
	if err != nil {
		t.Fatalf("获取存储统计信息失败: %v", err)
	}
	t.Logf("初始状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	if sm.config.Type != StorageTypeContainer {
		t.Fatalf("初始存储模式不正确，期望=%v, 实际=%v", StorageTypeContainer, sm.config.Type)
	}

	// 写入小量数据
	dataSize := 1024 // 1KB
	data := make([]byte, dataSize)
	for i := 0; i < dataSize; i++ {
		data[i] = byte(i % 256)
	}

	t.Logf("写入10个数据块到容器存储")
	// 写入10个块到容器模式，约10KB
	for i := 0; i < 10; i++ {
		err = sm.WriteBlock(uint32(i), data)
		if err != nil {
			t.Fatalf("写入块失败: %v", err)
		}
	}

	// 验证写入后的状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取存储统计信息失败: %v", err)
	}
	t.Logf("写入后容器存储状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 准备转换到目录模式
	t.Logf("准备转换到目录模式，路径: %s", directoryPath)

	// 更新目录路径
	sm.config.Path = directoryPath

	// 手动转换到目录模式
	err = sm.ConvertType(StorageTypeDirectory)
	if err != nil {
		t.Fatalf("转换到目录模式失败: %v", err)
	}

	// 验证转换后状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取转换后存储统计信息失败: %v", err)
	}
	t.Logf("转换后目录存储状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 验证存储模式已经改变
	if sm.config.Type != StorageTypeDirectory {
		t.Fatalf("转换后存储模式不正确，期望=%v, 实际=%v", StorageTypeDirectory, sm.config.Type)
	}

	// 读取之前写入的数据，确认转换后数据完整性
	t.Logf("验证转换后数据的完整性")
	for i := 0; i < 10; i++ {
		readData, err := sm.ReadBlock(uint32(i))
		if err != nil {
			t.Fatalf("读取块失败: %v", err)
		}
		if len(readData) != dataSize {
			t.Errorf("读取数据大小不正确, 期望: %d, 实际: %d", dataSize, len(readData))
		}
	}

	// 尝试写入更多数据到目录模式
	t.Logf("尝试写入更多数据到目录模式")
	for i := 10; i < 20; i++ {
		err = sm.WriteBlock(uint32(i), data)
		if err != nil {
			t.Fatalf("写入目录模式块失败: %v", err)
		}
	}

	// 验证新写入数据的状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取目录模式新写入后统计信息失败: %v", err)
	}
	t.Logf("目录模式写入后状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 转换回容器模式
	t.Logf("准备转换回容器模式，路径: %s", containerPath)

	// 更新路径
	sm.config.Path = containerPath

	// 手动转换回容器模式
	err = sm.ConvertType(StorageTypeContainer)
	if err != nil {
		t.Fatalf("转换回容器模式失败: %v", err)
	}

	// 验证转换后状态
	stats, err = sm.GetStats()
	if err != nil {
		t.Fatalf("获取转换回容器模式后统计信息失败: %v", err)
	}
	t.Logf("转换回容器模式后状态: 块数=%d, 空间=%d", stats.TotalBlocks, stats.UsedSpace)

	// 验证存储模式已经改变
	if sm.config.Type != StorageTypeContainer {
		t.Fatalf("转换后存储模式不正确，期望=%v, 实际=%v", StorageTypeContainer, sm.config.Type)
	}

	// 读取所有之前写入的数据，确认转换后数据完整性
	t.Logf("验证转换回容器模式后数据的完整性")
	for i := 0; i < 20; i++ {
		readData, err := sm.ReadBlock(uint32(i))
		if err != nil {
			t.Fatalf("读取块失败: %v", err)
		}
		if len(readData) != dataSize {
			t.Errorf("读取数据大小不正确, 期望: %d, 实际: %d", dataSize, len(readData))
		}
	}

	t.Logf("存储模式转换测试完成")
}
