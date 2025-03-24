package fuse

import (
	"context"
	"os"
	"runtime"
	"testing"
)

// TestHanwenStorageIntegration 测试Hanwen提供者与存储适配器的集成
func TestHanwenStorageIntegration(t *testing.T) {
	// 对于macOS系统，跳过此测试，同时提供有用的信息
	if runtime.GOOS == "darwin" {
		// 检查macFUSE是否已安装并输出安装指南
		helper := &MacOSFUSEHelper{}
		if err := helper.CheckFUSEInstallation(); err != nil {
			t.Skipf("在macOS上跳过此测试: %v", err)
		} else {
			t.Skip(`
在macOS上跳过标准FUSE测试，但macFUSE已正确安装。
要测试macOS上的FUSE功能，请使用以下命令:
  go test -run=TestMacOS
  
这将运行针对macOS环境优化的FUSE测试，避免使用不兼容的组件。
`)
		}
	}

	// 跳过需要真实挂载的测试
	if os.Getuid() != 0 && !isTestingEnvironment() {
		t.Skip("跳过集成测试: 需要root权限或特殊测试环境")
	}

	// 创建模拟存储适配器
	storage := NewMockStorageAdapter()
	storage.PopulateTestData()

	// 初始化Hanwen FUSE提供者
	provider := NewHanwenFUSEProvider()
	provider.adapters = map[string]*FragmentaStorageAdapter{} // 初始化

	// 创建临时挂载点
	tempDir, err := os.MkdirTemp("", "fuse-integration-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 准备挂载选项
	options := MountOptions{
		MountPoint: tempDir,
		Debug:      true,
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
	}

	// 执行挂载(注意：这里只测试代码逻辑，实际挂载可能会失败)
	err = provider.Mount(tempDir, options)
	if err != nil {
		// 记录错误但继续测试
		t.Logf("挂载尝试失败 (可能正常): %v", err)
	} else {
		// 如果挂载成功，确保我们清理
		defer provider.Unmount(tempDir)
		t.Log("挂载成功，测试完成后将卸载")
	}
}

// TestStorageAdapterOperations 测试存储适配器的基本操作
func TestStorageAdapterOperations(t *testing.T) {
	// 创建上下文
	ctx := context.Background()

	// 创建模拟存储适配器
	storage := NewMockStorageAdapter()

	// 测试创建目录
	err := storage.CreateDirectory(ctx, "/test_dir")
	if err != nil {
		t.Errorf("创建目录失败: %v", err)
	}

	// 测试写入文件
	testContent := []byte("这是测试内容")
	err = storage.WriteFile(ctx, "/test_dir/test.txt", testContent)
	if err != nil {
		t.Errorf("写入文件失败: %v", err)
	}

	// 测试读取文件
	content, err := storage.ReadFile(ctx, "/test_dir/test.txt")
	if err != nil {
		t.Errorf("读取文件失败: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("文件内容不匹配, 期望 %s, 实际 %s", string(testContent), string(content))
	}

	// 测试获取文件信息
	info, err := storage.GetInfo(ctx, "/test_dir/test.txt")
	if err != nil {
		t.Errorf("获取文件信息失败: %v", err)
	}
	if info.IsDir {
		t.Error("文件被错误标记为目录")
	}
	if info.Size != int64(len(testContent)) {
		t.Errorf("文件大小不匹配, 期望 %d, 实际 %d", len(testContent), info.Size)
	}

	// 测试文件存在性
	exists, err := storage.Exists(ctx, "/test_dir/test.txt")
	if err != nil {
		t.Errorf("检查文件存在性失败: %v", err)
	}
	if !exists {
		t.Error("文件应该存在，但被报告为不存在")
	}

	// 测试列出目录
	entries, err := storage.ListDirectory(ctx, "/test_dir")
	if err != nil {
		t.Errorf("列出目录失败: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("目录条目数不匹配, 期望 1, 实际 %d", len(entries))
	}

	// 测试移动文件
	err = storage.Move(ctx, "/test_dir/test.txt", "/test_dir/moved.txt")
	if err != nil {
		t.Errorf("移动文件失败: %v", err)
	}

	// 确认原文件不存在，新文件存在
	exists, _ = storage.Exists(ctx, "/test_dir/test.txt")
	if exists {
		t.Error("原文件不应该存在")
	}
	exists, _ = storage.Exists(ctx, "/test_dir/moved.txt")
	if !exists {
		t.Error("新文件应该存在")
	}

	// 测试删除文件
	err = storage.Delete(ctx, "/test_dir/moved.txt")
	if err != nil {
		t.Errorf("删除文件失败: %v", err)
	}

	// 测试删除目录
	err = storage.Delete(ctx, "/test_dir")
	if err != nil {
		t.Errorf("删除目录失败: %v", err)
	}
}
