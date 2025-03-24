package fuse

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MockStorageAdapter 模拟存储适配器用于测试
type MockStorageAdapter struct {
	files map[string][]byte
	info  map[string]*FileInfo
	lock  sync.RWMutex
}

// NewMockStorageAdapter 创建新的模拟存储适配器
func NewMockStorageAdapter() *MockStorageAdapter {
	return &MockStorageAdapter{
		files: make(map[string][]byte),
		info:  make(map[string]*FileInfo),
	}
}

// ReadFile 实现StorageManager接口
func (m *MockStorageAdapter) ReadFile(ctx context.Context, path string) ([]byte, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	data, exists := m.files[path]
	if !exists {
		return nil, os.ErrNotExist
	}
	return data, nil
}

// WriteFile 实现StorageManager接口
func (m *MockStorageAdapter) WriteFile(ctx context.Context, path string, data []byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.files[path] = data

	// 如果文件信息不存在，创建一个
	if _, exists := m.info[path]; !exists {
		m.info[path] = &FileInfo{
			Path:       path,
			IsDir:      false,
			Size:       int64(len(data)),
			Mode:       0644,
			CreatedAt:  time.Now().Unix(),
			ModifiedAt: time.Now().Unix(),
			AccessedAt: time.Now().Unix(),
		}
	} else {
		// 更新文件信息
		m.info[path].Size = int64(len(data))
		m.info[path].ModifiedAt = time.Now().Unix()
		m.info[path].AccessedAt = time.Now().Unix()
	}

	return nil
}

// CreateDirectory 实现StorageManager接口
func (m *MockStorageAdapter) CreateDirectory(ctx context.Context, path string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// 检查父目录是否存在
	parent := filepath.Dir(path)
	if parent != "." && parent != "/" {
		if _, exists := m.info[parent]; !exists {
			return fmt.Errorf("父目录不存在: %s", parent)
		}
		if !m.info[parent].IsDir {
			return fmt.Errorf("父路径不是目录: %s", parent)
		}
	}

	// 添加目录信息
	m.info[path] = &FileInfo{
		Path:       path,
		IsDir:      true,
		Size:       0,
		Mode:       0755,
		CreatedAt:  time.Now().Unix(),
		ModifiedAt: time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}

	return nil
}

// Delete 实现StorageManager接口
func (m *MockStorageAdapter) Delete(ctx context.Context, path string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// 检查是否存在
	if _, exists := m.info[path]; !exists {
		return os.ErrNotExist
	}

	// 检查是否为目录且非空
	if m.info[path].IsDir {
		// 检查是否有子项
		for p := range m.info {
			if filepath.Dir(p) == path {
				return fmt.Errorf("目录非空: %s", path)
			}
		}
	}

	// 删除文件数据和信息
	delete(m.files, path)
	delete(m.info, path)

	return nil
}

// GetInfo 实现StorageManager接口
func (m *MockStorageAdapter) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	info, exists := m.info[path]
	if !exists {
		return nil, os.ErrNotExist
	}
	return info, nil
}

// ListDirectory 实现StorageManager接口
func (m *MockStorageAdapter) ListDirectory(ctx context.Context, path string) ([]FileInfo, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	// 检查目录是否存在
	dirInfo, exists := m.info[path]
	if !exists {
		return nil, os.ErrNotExist
	}
	if !dirInfo.IsDir {
		return nil, fmt.Errorf("不是目录: %s", path)
	}

	var result []FileInfo
	// 列出所有直接子项
	for p, info := range m.info {
		if filepath.Dir(p) == path {
			result = append(result, *info)
		}
	}

	return result, nil
}

// Move 实现StorageManager接口
func (m *MockStorageAdapter) Move(ctx context.Context, oldPath, newPath string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// 检查源是否存在
	oldInfo, exists := m.info[oldPath]
	if !exists {
		return os.ErrNotExist
	}

	// 检查目标是否已存在
	if _, exists := m.info[newPath]; exists {
		return fmt.Errorf("目标已存在: %s", newPath)
	}

	// 移动文件或目录
	if !oldInfo.IsDir {
		// 移动文件
		m.files[newPath] = m.files[oldPath]
		delete(m.files, oldPath)
	}

	// 复制信息并更新路径
	newInfo := *oldInfo
	newInfo.Path = newPath
	m.info[newPath] = &newInfo
	delete(m.info, oldPath)

	return nil
}

// Exists 实现StorageManager接口
func (m *MockStorageAdapter) Exists(ctx context.Context, path string) (bool, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	_, exists := m.info[path]
	return exists, nil
}

// UpdateMetadata 实现StorageManager接口
func (m *MockStorageAdapter) UpdateMetadata(ctx context.Context, path string, info *FileInfo) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// 检查文件是否存在
	existing, exists := m.info[path]
	if !exists {
		return os.ErrNotExist
	}

	// 只能更新某些字段，有些字段如IsDir不应更改
	existing.Mode = info.Mode
	if info.ModifiedAt > 0 {
		existing.ModifiedAt = info.ModifiedAt
	}
	if info.AccessedAt > 0 {
		existing.AccessedAt = info.AccessedAt
	}

	return nil
}

// 填充一些测试数据
func (m *MockStorageAdapter) PopulateTestData() {
	ctx := context.Background()

	// 创建根目录
	m.CreateDirectory(ctx, "/")

	// 创建一些文件和目录
	m.CreateDirectory(ctx, "/docs")
	m.CreateDirectory(ctx, "/images")

	m.WriteFile(ctx, "/README.md", []byte("# 测试文件系统\n这是一个测试README文件。"))
	m.WriteFile(ctx, "/docs/guide.txt", []byte("这是使用指南。\n1. 第一步\n2. 第二步"))
	m.WriteFile(ctx, "/images/logo.txt", []byte("这里应该是一个图像，但为了测试使用文本代替。"))
}
