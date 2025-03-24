// package common 提供FUSE挂载示例的共享类型和函数
package common

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/bpfs/fragmenta"
)

// FuseMounter 是FUSE挂载器接口
type FuseMounter interface {
	// Mount 将存储挂载到指定路径
	Mount(mountPoint string) error
	// Unmount 卸载文件系统
	Unmount() error
}

// FuseMountOptions 定义FUSE挂载选项
type FuseMountOptions struct {
	// AllowOther 允许其他用户访问
	AllowOther bool
	// ReadOnly 以只读模式挂载
	ReadOnly bool
	// Debug 启用调试模式
	Debug bool
	// FSName 文件系统名称
	FSName string
	// VolumeName 卷名称
	VolumeName string
	// Permissions 默认权限
	Permissions uint32
}

// MacFuseOptions 定义macOS专用FUSE挂载选项
type MacFuseOptions struct {
	// AllowRoot 允许root访问
	AllowRoot bool
	// AllowOther 允许其他用户访问
	AllowOther bool
	// VolumeName 卷名称
	VolumeName string
	// NoAppleDouble 禁止创建.AppleDouble文件
	NoAppleDouble bool
	// NoBrowse 是否允许在Finder中浏览
	NoBrowse bool
	// FilePermissions 文件默认权限
	FilePermissions uint32
	// DirPermissions 目录默认权限
	DirPermissions uint32
	// Debug 启用调试模式
	Debug bool
}

// BasicFuseMounter 实现基本的FUSE挂载功能
type BasicFuseMounter struct {
	Storage fragmenta.Fragmenta
	Options *FuseMountOptions
}

// NewFuseMounter 创建新的FUSE挂载器
func NewFuseMounter(storage fragmenta.Fragmenta, options *FuseMountOptions) (FuseMounter, error) {
	// 简单实现，实际上需要检查FUSE是否可用
	if storage == nil {
		return nil, fmt.Errorf("存储对象不能为空")
	}

	return &BasicFuseMounter{
		Storage: storage,
		Options: options,
	}, nil
}

// Mount 挂载存储为FUSE文件系统
func (m *BasicFuseMounter) Mount(mountPoint string) error {
	// 这里是模拟实现，实际应当使用真正的FUSE库
	fmt.Printf("模拟挂载到 %s (实验性功能)\n", mountPoint)
	fmt.Println("注意: 此功能尚未完全实现，仅作为演示")

	// 等待模拟挂载被中断
	<-make(chan struct{})
	return nil
}

// Unmount 卸载FUSE文件系统
func (m *BasicFuseMounter) Unmount() error {
	// 模拟卸载
	return nil
}

// MacFuseMounter 实现macOS特定的FUSE挂载
type MacFuseMounter struct {
	Storage fragmenta.Fragmenta
	Options *MacFuseOptions
}

// NewMacFuseMounter 创建新的macOS FUSE挂载器
func NewMacFuseMounter(storage fragmenta.Fragmenta, options *MacFuseOptions) (FuseMounter, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("MacFuseMounter 仅支持macOS系统")
	}

	if storage == nil {
		return nil, fmt.Errorf("存储对象不能为空")
	}

	// 检查macFUSE是否安装
	if !checkMacFUSEAvailable() {
		return nil, fmt.Errorf("未检测到macFUSE，请先安装")
	}

	return &MacFuseMounter{
		Storage: storage,
		Options: options,
	}, nil
}

// Mount 在macOS上挂载FUSE文件系统
func (m *MacFuseMounter) Mount(mountPoint string) error {
	fmt.Printf("模拟在macOS上挂载到 %s (实验性功能)\n", mountPoint)
	fmt.Println("注意: 此功能尚未完全实现，仅作为演示")

	// 等待模拟挂载被中断
	<-make(chan struct{})
	return nil
}

// Unmount 在macOS上卸载FUSE文件系统
func (m *MacFuseMounter) Unmount() error {
	// 模拟卸载
	return nil
}

// 检查macFUSE是否可用
func checkMacFUSEAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	// 简单检查是否有mount_macfuse命令
	cmd := exec.Command("which", "mount_macfuse")
	return cmd.Run() == nil
}
