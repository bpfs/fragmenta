package fuse

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MemoryStorageAdapter 提供基于内存的存储实现，用于测试和演示目的
type MemoryStorageAdapter struct {
	// 文件内容映射 - 路径到内容
	files map[string][]byte
	// 文件元数据映射 - 路径到元数据
	metadata map[string]*FileInfo
	// 互斥锁保证线程安全
	mu sync.RWMutex
}

// NewMemoryStorageAdapter 创建新的内存存储适配器
func NewMemoryStorageAdapter() *MemoryStorageAdapter {
	adapter := &MemoryStorageAdapter{
		files:    make(map[string][]byte),
		metadata: make(map[string]*FileInfo),
	}

	// 初始化根目录
	rootInfo := &FileInfo{
		Path:       "/",
		IsDir:      true,
		Size:       0,
		Inode:      1, // 根目录inode为1
		UID:        0,
		GID:        0,
		Mode:       0755 | os.ModeDir,
		CreatedAt:  time.Now().Unix(),
		ModifiedAt: time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}
	adapter.metadata["/"] = rootInfo

	return adapter
}

// ReadFile 实现StorageManager接口，从内存中读取文件
func (m *MemoryStorageAdapter) ReadFile(ctx context.Context, path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查文件是否存在
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}

	// 更新访问时间
	if info, exists := m.metadata[path]; exists {
		info.AccessedAt = time.Now().Unix()
	}

	return data, nil
}

// WriteFile 实现StorageManager接口，将文件写入内存
func (m *MemoryStorageAdapter) WriteFile(ctx context.Context, path string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 存储文件内容
	m.files[path] = data

	// 更新或创建元数据
	now := time.Now().Unix()
	info, exists := m.metadata[path]
	if exists {
		info.Size = int64(len(data))
		info.ModifiedAt = now
		info.AccessedAt = now
	} else {
		// 确保父目录存在
		dirPath := filepath.Dir(path)
		if dirPath != "/" && dirPath != "." {
			if _, exists := m.metadata[dirPath]; !exists {
				return os.ErrNotExist
			}
		}

		// 创建新元数据
		m.metadata[path] = &FileInfo{
			Path:       path,
			IsDir:      false,
			Size:       int64(len(data)),
			Inode:      uint32(len(m.metadata) + 1), // 简单的inode分配策略
			UID:        0,
			GID:        0,
			Mode:       0644,
			CreatedAt:  now,
			ModifiedAt: now,
			AccessedAt: now,
		}
	}

	return nil
}

// CreateDirectory 实现StorageManager接口，创建目录
func (m *MemoryStorageAdapter) CreateDirectory(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查目录是否已存在
	if _, exists := m.metadata[path]; exists {
		return os.ErrExist
	}

	// 确保父目录存在
	dirPath := filepath.Dir(path)
	if dirPath != "/" && dirPath != "." {
		parentInfo, exists := m.metadata[dirPath]
		if !exists {
			return os.ErrNotExist
		}

		// 确保父路径是目录
		if !parentInfo.IsDir {
			return os.ErrInvalid
		}
	}

	// 创建目录元数据
	now := time.Now().Unix()
	m.metadata[path] = &FileInfo{
		Path:       path,
		IsDir:      true,
		Size:       0,
		Inode:      uint32(len(m.metadata) + 1), // 简单的inode分配策略
		UID:        0,
		GID:        0,
		Mode:       0755 | os.ModeDir,
		CreatedAt:  now,
		ModifiedAt: now,
		AccessedAt: now,
	}

	return nil
}

// Delete 实现StorageManager接口，删除文件或目录
func (m *MemoryStorageAdapter) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查路径是否存在
	info, exists := m.metadata[path]
	if !exists {
		return os.ErrNotExist
	}

	// 如果是目录，检查是否为空
	if info.IsDir {
		for metaPath := range m.metadata {
			if metaPath != path && strings.HasPrefix(metaPath, path+"/") {
				return os.ErrInvalid // 目录不为空
			}
		}
	}

	// 删除元数据
	delete(m.metadata, path)

	// 如果是文件，删除内容
	if !info.IsDir {
		delete(m.files, path)
	}

	return nil
}

// GetInfo 实现StorageManager接口，获取文件信息
func (m *MemoryStorageAdapter) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 获取元数据
	info, exists := m.metadata[path]
	if !exists {
		return nil, os.ErrNotExist
	}

	// 创建副本避免外部修改
	result := *info
	return &result, nil
}

// ListDirectory 实现StorageManager接口，列出目录内容
func (m *MemoryStorageAdapter) ListDirectory(ctx context.Context, path string) ([]FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查目录是否存在
	dirInfo, exists := m.metadata[path]
	if !exists {
		return nil, os.ErrNotExist
	}

	if !dirInfo.IsDir {
		return nil, os.ErrInvalid // 不是目录
	}

	// 规范化路径，确保以/结尾
	normPath := path
	if path != "/" && !strings.HasSuffix(path, "/") {
		normPath = path + "/"
	}

	// 查找直接子项
	var results []FileInfo
	for entryPath, info := range m.metadata {
		// 跳过自身
		if entryPath == path {
			continue
		}

		// 检查是否是直接子项
		parent := filepath.Dir(entryPath)
		if parent == "." {
			parent = "/"
		}
		if !strings.HasSuffix(parent, "/") {
			parent = parent + "/"
		}

		if (path == "/" && parent == "/") || parent == normPath {
			results = append(results, *info)
		}
	}

	return results, nil
}

// Move 实现StorageManager接口，移动或重命名文件/目录
func (m *MemoryStorageAdapter) Move(ctx context.Context, oldPath, newPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查源路径是否存在
	info, exists := m.metadata[oldPath]
	if !exists {
		return os.ErrNotExist
	}

	// 检查目标路径是否已存在
	if _, exists := m.metadata[newPath]; exists {
		return os.ErrExist
	}

	// 确保目标父目录存在
	newDirPath := filepath.Dir(newPath)
	if newDirPath != "/" && newDirPath != "." {
		parentInfo, exists := m.metadata[newDirPath]
		if !exists {
			return os.ErrNotExist
		}

		// 确保父路径是目录
		if !parentInfo.IsDir {
			return os.ErrInvalid
		}
	}

	// 创建新元数据
	newInfo := *info
	newInfo.Path = newPath
	newInfo.ModifiedAt = time.Now().Unix()

	// 更新元数据映射
	m.metadata[newPath] = &newInfo
	delete(m.metadata, oldPath)

	// 如果是文件，移动内容
	if !info.IsDir {
		m.files[newPath] = m.files[oldPath]
		delete(m.files, oldPath)
	} else {
		// 如果是目录，递归更新所有子路径
		prefix := oldPath
		if prefix != "/" && !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}

		for entryPath, entryInfo := range m.metadata {
			if strings.HasPrefix(entryPath, prefix) {
				// 计算新路径
				relativePath := strings.TrimPrefix(entryPath, prefix)
				newEntryPath := filepath.Join(newPath, relativePath)

				// 创建新元数据
				newEntryInfo := *entryInfo
				newEntryInfo.Path = newEntryPath

				// 更新映射
				m.metadata[newEntryPath] = &newEntryInfo
				delete(m.metadata, entryPath)

				// 如果是文件，移动内容
				if !entryInfo.IsDir {
					m.files[newEntryPath] = m.files[entryPath]
					delete(m.files, entryPath)
				}
			}
		}
	}

	return nil
}

// Exists 实现StorageManager接口，检查路径是否存在
func (m *MemoryStorageAdapter) Exists(ctx context.Context, path string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.metadata[path]
	return exists, nil
}

// UpdateMetadata 实现StorageManager接口，更新文件或目录的元数据
func (m *MemoryStorageAdapter) UpdateMetadata(ctx context.Context, path string, info *FileInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查路径是否存在
	existing, exists := m.metadata[path]
	if !exists {
		return os.ErrNotExist
	}

	// 保留原始Inode
	info.Inode = existing.Inode

	// 更新元数据
	m.metadata[path] = info

	return nil
}
