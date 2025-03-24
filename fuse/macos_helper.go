//go:build darwin
// +build darwin

package fuse

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// macFUSE安装指南
const MacFUSEInstallInstructions = `
要在macOS上使用FUSE功能，请按照以下步骤安装macFUSE:

方法1: 使用Homebrew安装(推荐)
  $ brew install --cask macfuse

方法2: 手动下载安装
  1. 访问 https://github.com/osxfuse/osxfuse/releases
  2. 下载最新版本的.pkg安装文件
  3. 双击下载的.pkg文件并按照向导完成安装
  4. 安装完成后重启系统

安装后的注意事项:
  - macOS Sonoma及更高版本中，需要在"系统设置 → 隐私与安全性"中允许macFUSE内核扩展
  - 如果挂载操作失败，尝试重启系统后再试
  - 由于系统安全限制，首次使用时可能需要在系统偏好设置中明确允许

常见问题:
  1. 挂载延迟问题: 在macOS Sonoma上可能遇到写入延迟，可使用noasync选项解决
  2. 权限问题: 确保应用有足够权限访问挂载点目录
  3. 内核扩展被阻止: 查看系统安全设置并允许macFUSE内核扩展
`

// MacOSFUSEHelper 提供macOS特定的FUSE挂载辅助功能
type MacOSFUSEHelper struct{}

// CheckFUSEInstallation 检查macFUSE是否已安装
func (h *MacOSFUSEHelper) CheckFUSEInstallation() error {
	// 检查macFUSE安装
	cmd := exec.Command("pkgutil", "--pkg-info", "com.github.osxfuse.pkg.MacFUSE")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("macFUSE未安装，无法使用FUSE功能: %v\n\n%s", err, MacFUSEInstallInstructions)
	}

	// 检查macFUSE是否可用
	_, err := os.Stat("/usr/local/lib/libfuse.dylib")
	if os.IsNotExist(err) {
		return fmt.Errorf("macFUSE库文件不存在，可能安装不完整。请重新安装macFUSE\n\n%s", MacFUSEInstallInstructions)
	}

	return nil
}

// SafeRemount 使用noasync选项重新挂载FUSE卷以解决macOS Sonoma写入延迟问题
func (h *MacOSFUSEHelper) SafeRemount(mountPoint string) error {
	// 首先找到设备
	cmd := exec.Command("df")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("无法获取已挂载文件系统列表: %v", err)
	}

	// 查找挂载点对应的设备
	var devicePath string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, mountPoint) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				devicePath = fields[0]
				break
			}
		}
	}

	if devicePath == "" {
		return fmt.Errorf("无法找到挂载点 %s 对应的设备", mountPoint)
	}

	// 卸载当前挂载点
	umountCmd := exec.Command("umount", mountPoint)
	if err := umountCmd.Run(); err != nil {
		return fmt.Errorf("无法卸载 %s: %v", mountPoint, err)
	}

	// 确保挂载点存在
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return fmt.Errorf("无法创建挂载点目录 %s: %v", mountPoint, err)
		}
	}

	// 等待2秒确保完全卸载
	time.Sleep(2 * time.Second)

	// 使用noasync选项重新挂载
	mountCmd := exec.Command("mount", "-o", "noasync", devicePath, mountPoint)
	if err := mountCmd.Run(); err != nil {
		return fmt.Errorf("无法重新挂载 %s: %v", mountPoint, err)
	}

	return nil
}

// GetMacOSFUSEOptions 返回macOS特定的FUSE挂载选项
func GetMacOSFUSEOptions() []string {
	return []string{
		"-o", "noasync", // 解决写入延迟问题
		"-o", "local", // 本地卷
		"-o", "volname=Fragmenta", // 卷名
		"-o", "defer_permissions", // 延迟权限检查
	}
}
