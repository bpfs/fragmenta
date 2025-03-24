package fuse

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FragmentaStorageAdapter 适配基本存储服务到文件系统要求
type FragmentaStorageAdapter struct {
	// 原始存储管理器
	storage StorageManager
	// 路径到ID的映射
	pathToID map[string]uint32
	// ID到路径的映射
	idToPath map[uint32]string
	// 映射锁
	mappingLock sync.RWMutex
	// 根目录ID
	rootDirID uint32
	// 下一个可用ID
	nextID uint32
	// ID锁
	idLock sync.Mutex
	// 元数据缓存
	metadataCache map[uint32]*FileInfo
	// 缓存锁
	cacheLock sync.RWMutex
}

// 常量定义
const (
	// 元数据标签 - 表示文件类型
	MetaTypeTag uint16 = 1
	// 元数据标签 - 表示权限模式
	MetaModeTag uint16 = 2
	// 元数据标签 - 表示所有者ID
	MetaOwnerTag uint16 = 3
	// 元数据标签 - 表示组ID
	MetaGroupTag uint16 = 4
	// 元数据标签 - 表示创建时间
	MetaCreationTimeTag uint16 = 5
	// 元数据标签 - 表示修改时间
	MetaModificationTimeTag uint16 = 6
	// 元数据标签 - 文件名
	MetaNameTag uint16 = 7
	// 元数据标签 - 父目录ID
	MetaParentDirTag uint16 = 8

	// 文件类型 - 目录
	FileTypeDir uint8 = 1
	// 文件类型 - 常规文件
	FileTypeRegular uint8 = 2
	// 文件类型 - 符号链接
	FileTypeSymlink uint8 = 3
)

// NewFragmentaStorageAdapter 创建新的存储适配器
func NewFragmentaStorageAdapter(storageManager StorageManager) (*FragmentaStorageAdapter, error) {
	adapter := &FragmentaStorageAdapter{
		storage:       storageManager,
		pathToID:      make(map[string]uint32),
		idToPath:      make(map[uint32]string),
		rootDirID:     1, // 根目录ID为1
		nextID:        2, // ID从2开始分配
		metadataCache: make(map[uint32]*FileInfo),
	}

	// 初始化根目录
	if err := adapter.initRootDirectory(); err != nil {
		return nil, err
	}

	// 加载现有路径映射
	if err := adapter.loadPathMappings(); err != nil {
		return nil, err
	}

	return adapter, nil
}

// initRootDirectory 初始化根目录
func (a *FragmentaStorageAdapter) initRootDirectory() error {
	// 检查根目录是否存在
	exists, err := a.blockExists(a.rootDirID)
	if err != nil {
		return err
	}

	if !exists {
		// 创建根目录元数据
		rootInfo := &FileInfo{
			Path:       "/",
			IsDir:      true,
			Size:       0,
			Inode:      a.rootDirID,
			UID:        0,
			GID:        0,
			Mode:       0755 | os.ModeDir,
			CreatedAt:  time.Now().Unix(),
			ModifiedAt: time.Now().Unix(),
			AccessedAt: time.Now().Unix(),
		}

		// 序列化元数据
		metaData, err := json.Marshal(rootInfo)
		if err != nil {
			return err
		}

		// 存储元数据
		err = a.storage.WriteFile(context.Background(), fmt.Sprintf("meta:%d", a.rootDirID), metaData)
		if err != nil {
			return err
		}

		// 添加路径映射
		a.mappingLock.Lock()
		a.pathToID["/"] = a.rootDirID
		a.idToPath[a.rootDirID] = "/"
		a.mappingLock.Unlock()

		// 添加到元数据缓存
		a.cacheLock.Lock()
		a.metadataCache[a.rootDirID] = rootInfo
		a.cacheLock.Unlock()
	}

	return nil
}

// blockExists 检查块是否存在
func (a *FragmentaStorageAdapter) blockExists(id uint32) (bool, error) {
	// 检查元数据是否存在
	return a.storage.Exists(context.Background(), fmt.Sprintf("meta:%d", id))
}

// loadPathMappings 加载路径映射
func (a *FragmentaStorageAdapter) loadPathMappings() error {
	// TODO: 从存储中加载路径映射
	return nil
}

// ReadFile 读取文件内容
func (a *FragmentaStorageAdapter) ReadFile(ctx context.Context, path string) ([]byte, error) {
	// 获取文件ID
	id, err := a.PathToID(path)
	if err != nil {
		return nil, err
	}

	// 从存储读取文件内容
	data, err := a.storage.ReadFile(ctx, fmt.Sprintf("data:%d", id))
	if err != nil {
		return nil, err
	}

	// 更新访问时间
	a.updateAccessTime(id)

	return data, nil
}

// WriteFile 写入文件内容
func (a *FragmentaStorageAdapter) WriteFile(ctx context.Context, path string, data []byte) error {
	// 检查路径是否存在
	id, err := a.PathToID(path)
	if err != nil {
		// 如果不存在，创建新文件
		dirPath := filepath.Dir(path)

		// 确保父目录存在
		_, err := a.PathToID(dirPath)
		if err != nil {
			return fmt.Errorf("父目录不存在: %s", dirPath)
		}

		// 创建新的文件元数据
		fileInfo := &FileInfo{
			Path:       path,
			IsDir:      false,
			Size:       int64(len(data)),
			Inode:      a.getNextID(),
			UID:        0,    // 默认UID
			GID:        0,    // 默认GID
			Mode:       0644, // 默认权限
			CreatedAt:  time.Now().Unix(),
			ModifiedAt: time.Now().Unix(),
			AccessedAt: time.Now().Unix(),
		}

		// 保存元数据
		metaData, err := json.Marshal(fileInfo)
		if err != nil {
			return err
		}

		err = a.storage.WriteFile(ctx, fmt.Sprintf("meta:%d", fileInfo.Inode), metaData)
		if err != nil {
			return err
		}

		// 添加路径映射
		a.mappingLock.Lock()
		a.pathToID[path] = fileInfo.Inode
		a.idToPath[fileInfo.Inode] = path
		a.mappingLock.Unlock()

		// 添加到元数据缓存
		a.cacheLock.Lock()
		a.metadataCache[fileInfo.Inode] = fileInfo
		a.cacheLock.Unlock()

		// 写入文件内容
		id = fileInfo.Inode
	}

	// 写入文件内容
	err = a.storage.WriteFile(ctx, fmt.Sprintf("data:%d", id), data)
	if err != nil {
		return err
	}

	// 更新文件大小和修改时间
	a.updateFileMetadata(id, int64(len(data)))

	return nil
}

// 更新访问时间
func (a *FragmentaStorageAdapter) updateAccessTime(id uint32) {
	a.cacheLock.Lock()
	defer a.cacheLock.Unlock()

	info, ok := a.metadataCache[id]
	if ok {
		info.AccessedAt = time.Now().Unix()
	}
}

// 更新文件元数据
func (a *FragmentaStorageAdapter) updateFileMetadata(id uint32, size int64) {
	a.cacheLock.Lock()
	defer a.cacheLock.Unlock()

	info, ok := a.metadataCache[id]
	if ok {
		info.Size = size
		info.ModifiedAt = time.Now().Unix()

		// 序列化和保存元数据（异步）
		go func() {
			metaData, err := json.Marshal(info)
			if err == nil {
				a.storage.WriteFile(context.Background(), fmt.Sprintf("meta:%d", id), metaData)
			}
		}()
	}
}

// CreateDirectory 创建目录
func (a *FragmentaStorageAdapter) CreateDirectory(ctx context.Context, path string) error {
	// 检查路径是否已存在
	_, err := a.PathToID(path)
	if err == nil {
		return os.ErrExist
	}

	// 确保父目录存在
	dirPath := filepath.Dir(path)
	parentID, err := a.PathToID(dirPath)
	if err != nil {
		return fmt.Errorf("父目录不存在: %s", dirPath)
	}

	// 创建目录元数据
	dirInfo := &FileInfo{
		Path:       path,
		IsDir:      true,
		Size:       0,
		Inode:      a.getNextID(),
		UID:        0,                 // 默认UID
		GID:        0,                 // 默认GID
		Mode:       0755 | os.ModeDir, // 默认目录权限
		CreatedAt:  time.Now().Unix(),
		ModifiedAt: time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}

	// 序列化元数据
	metaData, err := json.Marshal(dirInfo)
	if err != nil {
		return err
	}

	// 存储元数据
	err = a.storage.WriteFile(ctx, fmt.Sprintf("meta:%d", dirInfo.Inode), metaData)
	if err != nil {
		return err
	}

	// 添加路径映射
	a.mappingLock.Lock()
	a.pathToID[path] = dirInfo.Inode
	a.idToPath[dirInfo.Inode] = path
	a.mappingLock.Unlock()

	// 添加到元数据缓存
	a.cacheLock.Lock()
	a.metadataCache[dirInfo.Inode] = dirInfo
	a.cacheLock.Unlock()

	// 更新父目录修改时间
	a.updateDirectoryModifiedTime(parentID)

	return nil
}

// 更新目录修改时间
func (a *FragmentaStorageAdapter) updateDirectoryModifiedTime(dirID uint32) {
	a.cacheLock.Lock()
	defer a.cacheLock.Unlock()

	info, ok := a.metadataCache[dirID]
	if ok {
		info.ModifiedAt = time.Now().Unix()

		// 序列化和保存元数据（异步）
		go func() {
			metaData, err := json.Marshal(info)
			if err == nil {
				a.storage.WriteFile(context.Background(), fmt.Sprintf("meta:%d", dirID), metaData)
			}
		}()
	}
}

// Delete 删除文件或目录
func (a *FragmentaStorageAdapter) Delete(ctx context.Context, path string) error {
	// 获取文件ID
	id, err := a.PathToID(path)
	if err != nil {
		return os.ErrNotExist
	}

	// 获取文件信息
	info, err := a.GetInfo(ctx, path)
	if err != nil {
		return err
	}

	// 如果是目录，检查是否为空
	if info.IsDir {
		// 列出目录内容
		entries, err := a.ListDirectory(ctx, path)
		if err != nil {
			return err
		}

		if len(entries) > 0 {
			return fmt.Errorf("不能删除非空目录: %s", path)
		}
	}

	// 删除元数据
	err = a.storage.Delete(ctx, fmt.Sprintf("meta:%d", id))
	if err != nil {
		return err
	}

	// 如果是文件，删除数据
	if !info.IsDir {
		err = a.storage.Delete(ctx, fmt.Sprintf("data:%d", id))
		if err != nil {
			return err
		}
	}

	// 删除路径映射
	a.mappingLock.Lock()
	delete(a.pathToID, path)
	delete(a.idToPath, id)
	a.mappingLock.Unlock()

	// 从缓存中删除
	a.cacheLock.Lock()
	delete(a.metadataCache, id)
	a.cacheLock.Unlock()

	// 更新父目录修改时间
	parentPath := filepath.Dir(path)
	parentID, err := a.PathToID(parentPath)
	if err == nil {
		a.updateDirectoryModifiedTime(parentID)
	}

	return nil
}

// GetInfo 获取文件或目录信息
func (a *FragmentaStorageAdapter) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	// 获取文件ID
	id, err := a.PathToID(path)
	if err != nil {
		return nil, os.ErrNotExist
	}

	// 检查缓存
	a.cacheLock.RLock()
	info, ok := a.metadataCache[id]
	a.cacheLock.RUnlock()

	if ok {
		return info, nil
	}

	// 从存储读取元数据
	metaData, err := a.storage.ReadFile(ctx, fmt.Sprintf("meta:%d", id))
	if err != nil {
		return nil, err
	}

	// 解析元数据
	info = &FileInfo{}
	err = json.Unmarshal(metaData, info)
	if err != nil {
		return nil, err
	}

	// 添加到缓存
	a.cacheLock.Lock()
	a.metadataCache[id] = info
	a.cacheLock.Unlock()

	return info, nil
}

// ListDirectory 列出目录内容
func (a *FragmentaStorageAdapter) ListDirectory(ctx context.Context, path string) ([]FileInfo, error) {
	// 获取目录ID
	dirID, err := a.PathToID(path)
	if err != nil {
		return nil, os.ErrNotExist
	}

	// 获取目录信息
	dirInfo, err := a.GetInfo(ctx, path)
	if err != nil {
		return nil, err
	}

	// 确保这是一个目录
	if !dirInfo.IsDir {
		return nil, fmt.Errorf("不是目录: %s", path)
	}

	// 遍历所有路径映射，找出所有子项
	var result []FileInfo
	a.mappingLock.RLock()
	for childPath, childID := range a.pathToID {
		// 跳过自身
		if childID == dirID {
			continue
		}

		// 检查是否是这个目录的直接子项
		if filepath.Dir(childPath) == path || (path == "/" && filepath.Dir(childPath) == "") {
			// 获取子项信息
			childInfo, err := a.GetInfo(ctx, childPath)
			if err == nil {
				result = append(result, *childInfo)
			}
		}
	}
	a.mappingLock.RUnlock()

	return result, nil
}

// Move 移动或重命名文件/目录
func (a *FragmentaStorageAdapter) Move(ctx context.Context, oldPath, newPath string) error {
	// 获取源文件ID
	srcID, err := a.PathToID(oldPath)
	if err != nil {
		return os.ErrNotExist
	}

	// 检查目标路径是否已存在
	_, err = a.PathToID(newPath)
	if err == nil {
		return os.ErrExist
	}

	// 确保目标目录存在
	targetDir := filepath.Dir(newPath)
	_, err = a.PathToID(targetDir)
	if err != nil {
		return fmt.Errorf("目标目录不存在: %s", targetDir)
	}

	// 获取源文件信息
	_, err = a.GetInfo(ctx, oldPath)
	if err != nil {
		return err
	}

	// 更新路径映射
	a.mappingLock.Lock()
	delete(a.pathToID, oldPath)
	a.pathToID[newPath] = srcID
	a.idToPath[srcID] = newPath
	a.mappingLock.Unlock()

	// 更新元数据
	a.cacheLock.Lock()
	if info, ok := a.metadataCache[srcID]; ok {
		info.Path = newPath
		info.ModifiedAt = time.Now().Unix()

		// 序列化和保存元数据
		metaData, err := json.Marshal(info)
		if err == nil {
			a.storage.WriteFile(ctx, fmt.Sprintf("meta:%d", srcID), metaData)
		}
	}
	a.cacheLock.Unlock()

	return nil
}

// Exists 检查文件是否存在
func (a *FragmentaStorageAdapter) Exists(ctx context.Context, path string) (bool, error) {
	// 获取文件ID
	_, err := a.PathToID(path)
	if err != nil {
		// 如果是不存在错误，返回false
		if os.IsNotExist(err) {
			return false, nil
		}
		// 其他错误直接返回
		return false, err
	}

	return true, nil
}

// PathToID 路径转ID
func (a *FragmentaStorageAdapter) PathToID(path string) (uint32, error) {
	// 规范化路径
	if path == "" {
		path = "/"
	}

	a.mappingLock.RLock()
	id, ok := a.pathToID[path]
	a.mappingLock.RUnlock()

	if ok {
		return id, nil
	}

	return 0, os.ErrNotExist
}

// IDToPath ID转路径
func (a *FragmentaStorageAdapter) IDToPath(id uint32) (string, error) {
	a.mappingLock.RLock()
	path, ok := a.idToPath[id]
	a.mappingLock.RUnlock()

	if ok {
		return path, nil
	}

	return "", os.ErrNotExist
}

// StorePath 存储路径映射
func (a *FragmentaStorageAdapter) StorePath(path string) (uint32, error) {
	// 检查路径是否已经映射
	a.mappingLock.RLock()
	id, ok := a.pathToID[path]
	a.mappingLock.RUnlock()

	if ok {
		return id, nil
	}

	// 分配新ID
	id = a.getNextID()

	// 添加映射
	a.mappingLock.Lock()
	a.pathToID[path] = id
	a.idToPath[id] = path
	a.mappingLock.Unlock()

	// TODO: 持久化映射

	return id, nil
}

// LoadPath 加载路径映射
func (a *FragmentaStorageAdapter) LoadPath(id uint32) (string, error) {
	return a.IDToPath(id)
}

// UpdateMetadata 实现StorageManager接口，更新文件或目录的元数据
func (a *FragmentaStorageAdapter) UpdateMetadata(ctx context.Context, path string, info *FileInfo) error {
	// 获取文件ID
	id, err := a.PathToID(path)
	if err != nil {
		return err
	}

	// 准备元数据
	metaData, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// 缓存更新
	a.cacheLock.Lock()
	a.metadataCache[id] = info
	a.cacheLock.Unlock()

	// 存储更新
	return a.storage.WriteFile(ctx, fmt.Sprintf("meta:%d", id), metaData)
}

// getNextID 获取下一个可用ID
func (a *FragmentaStorageAdapter) getNextID() uint32 {
	a.idLock.Lock()
	id := a.nextID
	a.nextID++
	a.idLock.Unlock()
	return id
}
