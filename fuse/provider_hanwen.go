package fuse

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// HanwenFUSEProvider 是基于hanwen/go-fuse的FUSE实现提供者
type HanwenFUSEProvider struct {
	servers  map[string]*fuse.Server
	lock     sync.RWMutex
	adapters map[string]*FragmentaStorageAdapter
}

// NewHanwenFUSEProvider 创建新的hanwen/go-fuse提供者
func NewHanwenFUSEProvider() *HanwenFUSEProvider {
	return &HanwenFUSEProvider{
		servers:  make(map[string]*fuse.Server),
		adapters: make(map[string]*FragmentaStorageAdapter),
	}
}

// Name 返回提供者名称
func (p *HanwenFUSEProvider) Name() string {
	return "Hanwen-FUSE"
}

// Mount 使用hanwen/go-fuse挂载文件系统
func (p *HanwenFUSEProvider) Mount(mountPoint string, options MountOptions) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// 确保挂载点存在
	absPath, err := filepath.Abs(mountPoint)
	if err != nil {
		return fmt.Errorf("获取挂载点绝对路径失败: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("创建挂载点目录失败: %v", err)
		}
	}

	// 检查是否已挂载
	if _, exists := p.servers[absPath]; exists {
		return fmt.Errorf("挂载点 %s 已被使用", absPath)
	}

	// 准备FUSE选项
	timeoutDuration := 1 * time.Second
	fuseOptions := &fs.Options{
		AttrTimeout:     &timeoutDuration,
		EntryTimeout:    &timeoutDuration,
		NegativeTimeout: &timeoutDuration,
		MountOptions: fuse.MountOptions{
			Debug:      options.Debug,
			AllowOther: options.AllowOther,
			Name:       "fragmenta",
			FsName:     "FragmentaFS",
		},

		// 设置各种权限和所有者
		UID: options.UID,
		GID: options.GID,
	}

	if options.ReadOnly {
		fuseOptions.MountOptions.Options = append(fuseOptions.MountOptions.Options, "ro")
	}

	// 创建HanwenFragmentaFS
	root := &HanwenFragmentaRoot{
		options: options,
	}

	// 挂载
	server, err := fs.Mount(absPath, root, fuseOptions)
	if err != nil {
		return fmt.Errorf("挂载FUSE文件系统失败: %v", err)
	}

	// 保存服务器引用
	p.servers[absPath] = server

	// 如果是调试模式，打印一些信息
	if options.Debug {
		fmt.Printf("已挂载FragmentaFS到 %s\n", absPath)
	}

	return nil
}

// Unmount 卸载FUSE文件系统
func (p *HanwenFUSEProvider) Unmount(mountPoint string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	absPath, err := filepath.Abs(mountPoint)
	if err != nil {
		return fmt.Errorf("获取挂载点绝对路径失败: %v", err)
	}

	// 检查是否有已保存的服务器实例
	server, exists := p.servers[absPath]
	if !exists {
		// 尝试使用系统命令卸载
		cmd := exec.Command("umount", absPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("卸载失败: %v", err)
		}
		return nil
	}

	// 优雅关闭服务器
	server.Unmount()
	delete(p.servers, absPath)
	delete(p.adapters, absPath)

	return nil
}

// HanwenFragmentaRoot 是基于hanwen/go-fuse的根文件系统实现
type HanwenFragmentaRoot struct {
	fs.Inode
	options MountOptions
}

// OnMount 在挂载时被调用
func (r *HanwenFragmentaRoot) OnMount(ctx context.Context) {
	// 创建一些根目录的初始内容，如README文件
	child := &fs.MemRegularFile{
		Data: []byte("Fragmenta文件系统 - 由Hanwen/go-fuse提供支持\n"),
		Attr: fuse.Attr{
			Mode: 0444, // 只读
		},
	}

	// 使用NewInode创建
	r.NewInode(
		ctx,
		child,
		fs.StableAttr{
			Ino:  2,
			Mode: fuse.S_IFREG,
		})
}
