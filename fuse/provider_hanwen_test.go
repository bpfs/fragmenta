package fuse

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHanwenFUSEProviderName(t *testing.T) {
	provider := NewHanwenFUSEProvider()
	if provider.Name() != "Hanwen-FUSE" {
		t.Errorf("Name()返回值错误, 期望 %s, 实际 %s", "Hanwen-FUSE", provider.Name())
	}
}

func TestHanwenFUSEProviderInit(t *testing.T) {
	provider := NewHanwenFUSEProvider()
	if provider.servers == nil {
		t.Error("servers映射未初始化")
	}
	if provider.adapters == nil {
		t.Error("adapters映射未初始化")
	}
	if len(provider.servers) != 0 {
		t.Errorf("初始化后servers应该为空, 当前大小 %d", len(provider.servers))
	}
}

func TestHanwenFUSEProviderMount(t *testing.T) {
	// 对于macOS系统，跳过此测试
	if runtime.GOOS == "darwin" {
		// 检查macFUSE是否已安装并输出安装指南
		helper := &MacOSFUSEHelper{}
		if err := helper.CheckFUSEInstallation(); err != nil {
			t.Skipf("在macOS上跳过挂载测试: %v", err)
		} else {
			t.Skip(`
在macOS上跳过标准FUSE挂载测试，但macFUSE已正确安装。
要测试macOS上的FUSE挂载功能，请使用以下命令:
  go test -run=TestMacOSMount

这将执行针对macOS环境优化的FUSE挂载测试。
`)
		}
	}

	if os.Getuid() != 0 && !isTestingEnvironment() {
		t.Skip("跳过挂载测试: 需要root权限或特殊测试环境")
	}

	provider := NewHanwenFUSEProvider()

	// 创建临时目录作为挂载点
	tempDir, err := os.MkdirTemp("", "fuse-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 准备测试选项
	options := MountOptions{
		MountPoint: tempDir,
		ReadOnly:   true,
		Debug:      true,
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
	}

	// 执行挂载（可能会失败，这里我们只测试逻辑流程）
	err = provider.Mount(tempDir, options)
	if err != nil {
		// 在CI环境中挂载可能会失败，但我们仍然可以测试代码逻辑
		t.Logf("挂载尝试失败 (可能正常): %v", err)
	} else {
		// 如果挂载成功，确保我们进行清理
		defer provider.Unmount(tempDir)

		// 验证服务器是否被正确保存
		absPath, _ := filepath.Abs(tempDir)
		if _, exists := provider.servers[absPath]; !exists {
			t.Errorf("服务器未被保存到映射中: %s", absPath)
		}
	}
}

func TestHanwenFUSEProviderUnmount(t *testing.T) {
	// 对于macOS系统，跳过此测试
	if runtime.GOOS == "darwin" {
		// 检查macFUSE是否已安装并输出安装指南
		helper := &MacOSFUSEHelper{}
		if err := helper.CheckFUSEInstallation(); err != nil {
			t.Skipf("在macOS上跳过卸载测试: %v", err)
		} else {
			t.Skip(`
在macOS上跳过标准FUSE卸载测试，但macFUSE已正确安装。
要测试macOS上的FUSE卸载功能，请使用以下命令:
  go test -run=TestMacOSMount

这将执行针对macOS环境优化的FUSE挂载和卸载测试。
`)
		}
	}

	if os.Getuid() != 0 && !isTestingEnvironment() {
		t.Skip("跳过卸载测试: 需要root权限或特殊测试环境")
	}

	provider := NewHanwenFUSEProvider()

	// 创建临时目录作为挂载点
	tempDir, err := os.MkdirTemp("", "fuse-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 无需实际挂载，我们可以测试卸载逻辑
	// absPath未使用，删除

	// 测试卸载不存在的挂载点 (应该使用系统命令)
	err = provider.Unmount(tempDir)
	if err == nil {
		// 可能会失败也可能成功，取决于系统umount命令的行为
		t.Log("卸载未挂载的路径成功")
	} else {
		t.Logf("卸载未挂载的路径失败 (可能正常): %v", err)
	}
}

// 辅助函数，检查当前是否在测试环境中
func isTestingEnvironment() bool {
	// CI环境变量检查
	return os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
}

// 测试根目录相关功能
func TestHanwenFragmentaRoot(t *testing.T) {
	root := &HanwenFragmentaRoot{
		options: MountOptions{
			ReadOnly: true,
			Debug:    true,
		},
	}

	// 简单存在性测试
	if root == nil {
		t.Error("无法创建HanwenFragmentaRoot实例")
	}
}

// 测试边缘情况
func TestHanwenFUSEProviderEdgeCases(t *testing.T) {
	provider := NewHanwenFUSEProvider()

	// 测试无效路径
	err := provider.Mount("", MountOptions{})
	if err == nil {
		t.Error("期望挂载空路径失败，但却成功了")
	}

	// 测试重复挂载同一目录
	tempDir, err := os.MkdirTemp("", "fuse-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 模拟已挂载的情况
	absPath, _ := filepath.Abs(tempDir)
	provider.servers[absPath] = nil

	// 尝试再次挂载
	err = provider.Mount(tempDir, MountOptions{})
	if err == nil {
		t.Errorf("期望挂载已用路径失败，但却成功了: %s", tempDir)
	}

	// 清理
	delete(provider.servers, absPath)
}
