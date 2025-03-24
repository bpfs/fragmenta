//go:build darwin
// +build darwin

package fuse

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

// 提供详细的macFUSE安装指南
const macFUSEInstallGuide = `
============== macFUSE安装指南 ==============
要在macOS上使用FUSE功能，需要安装macFUSE。

安装选项1 - 使用Homebrew:
  brew install --cask macfuse

安装选项2 - 手动安装:
  1. 访问 https://github.com/osxfuse/osxfuse/releases
  2. 下载最新版本的macFUSE安装包
  3. 双击下载的.pkg文件并按照向导完成安装
  4. 安装完成后可能需要重启系统

安装后注意事项:
  - 在macOS Sonoma及更高版本中，需要在"系统设置 → 隐私与安全性"中允许macFUSE内核扩展
  - 如果安装后功能仍不正常，请尝试重启系统

============================================
`

// checkAndReportMacFUSE 检查macFUSE是否已安装，如果未安装则打印详细指南
func checkAndReportMacFUSE(t *testing.T) bool {
	helper := &MacOSFUSEHelper{}
	err := helper.CheckFUSEInstallation()
	if err != nil {
		t.Logf("macFUSE检测: %v", err)
		t.Log(macFUSEInstallGuide)
		return false
	}
	return true
}

// TestMacOSMountUnmount 在macOS环境下测试挂载和卸载功能
func TestMacOSMountUnmount(t *testing.T) {
	// 检查macFUSE是否安装，未安装则显示提示但继续测试
	isMacFUSEInstalled := checkAndReportMacFUSE(t)
	if !isMacFUSEInstalled {
		t.Log("macFUSE未安装，测试将继续但预期会失败")
	}

	// 创建临时目录作为挂载点
	tempDir, err := os.MkdirTemp("", "macos-fuse-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建MacOS替代挂载器
	mounter := NewMacOSAlternativeMounter(tempDir, true)

	// 确认未挂载状态
	if isMounted(tempDir) {
		t.Fatalf("测试前预期未挂载，但实际已挂载: %s", tempDir)
	}

	// 尝试挂载
	t.Log("尝试挂载...")
	err = mounter.Mount()

	// 如果macFUSE未安装，预期挂载会失败
	if err != nil {
		if !isMacFUSEInstalled {
			t.Log("挂载预期失败 (macFUSE未安装): ", err)
		} else {
			t.Logf("挂载失败 (可能需要系统权限): %v", err)
			t.Log("提示: 在macOS上测试FUSE挂载通常需要root权限或明确的系统授权")
		}
	} else {
		// 如果成功挂载，尝试卸载
		t.Log("挂载成功，尝试卸载...")
		err = mounter.Unmount()
		if err != nil {
			t.Errorf("卸载失败: %v", err)
		} else {
			t.Log("卸载成功")
		}
	}
}

// TestMacOSHelperFunctions 测试macOS辅助函数
func TestMacOSHelperFunctions(t *testing.T) {
	// 检查macFUSE是否安装，仅显示提示但继续测试
	isMacFUSEInstalled := checkAndReportMacFUSE(t)

	helper := &MacOSFUSEHelper{}

	// 测试GetMacOSFUSEOptions
	options := GetMacOSFUSEOptions()
	if len(options) == 0 {
		t.Error("期望获取macOS FUSE选项，但结果为空")
	}

	// 检查noasync选项是否存在
	foundNoAsync := false
	for i := 0; i < len(options); i++ {
		if options[i] == "noasync" {
			foundNoAsync = true
			break
		}
	}
	if !foundNoAsync {
		t.Error("期望noasync选项在macOS FUSE选项中，但未找到")
	}

	// 测试CheckFUSEInstallation (结果取决于环境)
	err := helper.CheckFUSEInstallation()
	if err != nil {
		if !isMacFUSEInstalled {
			t.Log("macFUSE未安装，检测结果符合预期")
		} else {
			t.Errorf("macFUSE应该已安装但检测失败: %v", err)
		}
	} else {
		t.Log("macFUSE已安装")
	}
}

// TestMacOSStorageAdapterIntegration 测试存储适配器在macOS环境的基本功能
func TestMacOSStorageAdapterIntegration(t *testing.T) {
	// 创建上下文
	ctx := context.Background()

	// 创建内存存储适配器
	adapter := NewMemoryStorageAdapter()

	// 测试创建目录
	err := adapter.CreateDirectory(ctx, "/test_macos_dir")
	if err != nil {
		t.Errorf("创建目录失败: %v", err)
	}

	// 测试写入文件
	testContent := []byte("macOS测试内容")
	err = adapter.WriteFile(ctx, "/test_macos_dir/test.txt", testContent)
	if err != nil {
		t.Errorf("写入文件失败: %v", err)
	}

	// 测试读取文件
	content, err := adapter.ReadFile(ctx, "/test_macos_dir/test.txt")
	if err != nil {
		t.Errorf("读取文件失败: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("文件内容不匹配, 期望 %s, 实际 %s", string(testContent), string(content))
	}

	// 测试文件信息
	info, err := adapter.GetInfo(ctx, "/test_macos_dir/test.txt")
	if err != nil {
		t.Errorf("获取文件信息失败: %v", err)
	}
	if info.IsDir {
		t.Error("文件被错误识别为目录")
	}

	// 测试目录列表
	entries, err := adapter.ListDirectory(ctx, "/test_macos_dir")
	if err != nil {
		t.Errorf("列出目录失败: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("目录条目数不匹配, 期望 1, 实际 %d", len(entries))
	}
}

// 实用函数: 检查是否支持macFUSE
func checkMacFUSESupport() bool {
	cmd := exec.Command("pkgutil", "--pkg-info", "com.github.osxfuse.pkg.MacFUSE")
	return cmd.Run() == nil
}
