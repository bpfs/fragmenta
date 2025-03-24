package fuse

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// FragmentaSecurityAdapter 适配Fragmenta安全子系统到文件系统安全需求
type FragmentaSecurityAdapter struct {
	// TODO: 添加安全管理器引用
	// 权限缓存
	permissionCache map[string]uint32
	// 缓存锁
	cacheLock sync.RWMutex
}

// NewFragmentaSecurityAdapter 创建新的安全适配器
func NewFragmentaSecurityAdapter() (*FragmentaSecurityAdapter, error) {
	return &FragmentaSecurityAdapter{
		permissionCache: make(map[string]uint32),
	}, nil
}

// CheckReadPermission 检查读取权限
func (a *FragmentaSecurityAdapter) CheckReadPermission(ctx context.Context, path string, uid, gid uint32) bool {
	// 检查缓存
	cacheKey := fmt.Sprintf("r:%s:%d:%d", path, uid, gid)
	a.cacheLock.RLock()
	if perm, ok := a.permissionCache[cacheKey]; ok && perm != 0 {
		a.cacheLock.RUnlock()
		return true
	}
	a.cacheLock.RUnlock()

	// TODO: 实现真正的权限检查

	// 默认允许读取
	a.cacheLock.Lock()
	a.permissionCache[cacheKey] = 1
	a.cacheLock.Unlock()

	return true
}

// CheckWritePermission 检查写入权限
func (a *FragmentaSecurityAdapter) CheckWritePermission(ctx context.Context, path string, uid, gid uint32) bool {
	// 检查缓存
	cacheKey := fmt.Sprintf("w:%s:%d:%d", path, uid, gid)
	a.cacheLock.RLock()
	if perm, ok := a.permissionCache[cacheKey]; ok {
		a.cacheLock.RUnlock()
		return perm != 0
	}
	a.cacheLock.RUnlock()

	// TODO: 实现真正的权限检查

	// 默认允许写入
	a.cacheLock.Lock()
	a.permissionCache[cacheKey] = 1
	a.cacheLock.Unlock()

	return true
}

// CheckExecutePermission 检查执行权限
func (a *FragmentaSecurityAdapter) CheckExecutePermission(ctx context.Context, path string, uid, gid uint32) bool {
	// 检查缓存
	cacheKey := fmt.Sprintf("x:%s:%d:%d", path, uid, gid)
	a.cacheLock.RLock()
	if perm, ok := a.permissionCache[cacheKey]; ok {
		a.cacheLock.RUnlock()
		return perm != 0
	}
	a.cacheLock.RUnlock()

	// TODO: 实现真正的权限检查

	// 目录默认允许执行（用于遍历），文件默认不允许
	// 这里简单根据路径末尾是否有斜杠判断
	isDir := len(path) > 0 && path[len(path)-1] == '/'

	var permValue uint32
	if isDir {
		permValue = 1
	} else {
		permValue = 0
	}

	a.cacheLock.Lock()
	a.permissionCache[cacheKey] = permValue
	a.cacheLock.Unlock()

	return permValue != 0
}

// GetPermissions 获取文件权限
func (a *FragmentaSecurityAdapter) GetPermissions(ctx context.Context, path string) (os.FileMode, error) {
	// TODO: 实现真正的权限获取

	// 默认权限:
	// - 目录: rwxr-xr-x (0755)
	// - 文件: rw-r--r-- (0644)

	// 简单根据路径末尾是否有斜杠判断
	isDir := len(path) > 0 && path[len(path)-1] == '/'

	if isDir || path == "/" {
		return os.FileMode(0755) | os.ModeDir, nil
	}

	return os.FileMode(0644), nil
}

// SetPermissions 设置文件权限
func (a *FragmentaSecurityAdapter) SetPermissions(ctx context.Context, path string, mode os.FileMode) error {
	// TODO: 实现真正的权限设置

	// 清除缓存
	a.cacheLock.Lock()
	defer a.cacheLock.Unlock()

	// 清除与该路径相关的所有缓存
	for key := range a.permissionCache {
		if key[2:len(path)+3] == path+":" {
			delete(a.permissionCache, key)
		}
	}

	return nil
}
