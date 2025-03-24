//go:build darwin
// +build darwin

package fuse

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MacOSAlternativeMounter 提供替代的挂载方法
type MacOSAlternativeMounter struct {
	// 挂载点
	MountPoint string
	// 设备路径（如有）
	DevicePath string
	// 错误通道
	ErrorChan chan error
	// 是否启用调试
	Debug bool
}

// NewMacOSAlternativeMounter 创建一个新的macOS替代挂载器
func NewMacOSAlternativeMounter(mountPoint string, debug bool) *MacOSAlternativeMounter {
	return &MacOSAlternativeMounter{
		MountPoint: mountPoint,
		ErrorChan:  make(chan error, 1),
		Debug:      debug,
	}
}

// Mount 使用macFUSE执行挂载
func (m *MacOSAlternativeMounter) Mount() error {
	// 确保挂载点存在
	absPath, err := filepath.Abs(m.MountPoint)
	if err != nil {
		return fmt.Errorf("获取挂载点绝对路径失败: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("创建挂载点目录失败: %v", err)
		}
	}

	// 检查是否已挂载
	if isMounted(absPath) {
		// 尝试先卸载
		if err := m.Unmount(); err != nil {
			return fmt.Errorf("卸载已存在的挂载失败: %v", err)
		}
	}

	// 创建临时目录作为设备源
	tempDir, err := os.MkdirTemp("", "fragmenta-fuse-")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	m.DevicePath = tempDir

	// 构建挂载选项
	mountOptions := []string{
		"-o", "local",
		"-o", "noowners",
		"-o", "noasync",
		"-o", "volname=Fragmenta",
	}

	// 构建挂载命令
	args := append([]string{tempDir, absPath}, mountOptions...)
	cmd := exec.Command("mount_fusefs", args...)

	// 执行挂载
	if m.Debug {
		fmt.Printf("执行挂载命令: %s\n", cmd.String())
	}

	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("macOS挂载失败: %v", err)
	}

	// 验证挂载是否成功
	if !isMounted(absPath) {
		os.RemoveAll(tempDir)
		return fmt.Errorf("挂载点验证失败，挂载未成功")
	}

	return nil
}

// Unmount 卸载文件系统
func (m *MacOSAlternativeMounter) Unmount() error {
	absPath, err := filepath.Abs(m.MountPoint)
	if err != nil {
		return fmt.Errorf("获取挂载点绝对路径失败: %v", err)
	}

	// 如果未挂载，直接返回成功
	if !isMounted(absPath) {
		return nil
	}

	// 尝试使用系统umount命令
	cmd := exec.Command("umount", absPath)
	if err := cmd.Run(); err != nil {
		// 尝试使用强制卸载
		cmd = exec.Command("umount", "-f", absPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("卸载失败: %v", err)
		}
	}

	// 等待确保卸载完成
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		if !isMounted(absPath) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 清理临时目录
	if m.DevicePath != "" {
		os.RemoveAll(m.DevicePath)
		m.DevicePath = ""
	}

	return nil
}

// isMounted 检查给定路径是否已挂载
func isMounted(path string) bool {
	cmd := exec.Command("mount")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, path) {
			return true
		}
	}
	return false
}
