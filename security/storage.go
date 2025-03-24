package security

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileSecureStorage 基于文件系统的安全存储
type FileSecureStorage struct {
	// 存储根路径
	rootPath string

	// 文件操作锁
	fileLocks sync.Map
}

// NewFileSecureStorage 创建文件安全存储
func NewFileSecureStorage(rootPath string) (*FileSecureStorage, error) {
	// 确保存储目录存在
	if err := os.MkdirAll(rootPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &FileSecureStorage{
		rootPath: rootPath,
	}, nil
}

// Store 存储数据
func (fs *FileSecureStorage) Store(ctx context.Context, key string, data []byte) error {
	// 验证键名有效性
	if err := validateKeyName(key); err != nil {
		return err
	}

	// 获取文件路径
	filePath := fs.getFilePath(key)

	// 获取锁
	lock := fs.getLock(key)
	lock.Lock()
	defer lock.Unlock()

	// 确保目录存在
	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 以安全的方式写入文件
	// 先写入临时文件，然后重命名，以确保原子性
	tempPath := filePath + ".tmp"
	if err := ioutil.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// 重命名文件
	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath) // 清理临时文件
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// Retrieve 获取数据
func (fs *FileSecureStorage) Retrieve(ctx context.Context, key string) ([]byte, error) {
	// 验证键名有效性
	if err := validateKeyName(key); err != nil {
		return nil, err
	}

	// 获取文件路径
	filePath := fs.getFilePath(key)

	// 获取锁
	lock := fs.getLock(key)
	lock.RLock()
	defer lock.RUnlock()

	// 读取文件
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// Delete 删除数据
func (fs *FileSecureStorage) Delete(ctx context.Context, key string) error {
	// 验证键名有效性
	if err := validateKeyName(key); err != nil {
		return err
	}

	// 获取文件路径
	filePath := fs.getFilePath(key)

	// 获取锁
	lock := fs.getLock(key)
	lock.Lock()
	defer lock.Unlock()

	// 删除文件
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// List 列出存储的所有键
func (fs *FileSecureStorage) List(ctx context.Context) ([]string, error) {
	keys := []string{}

	// 遍历存储目录
	err := filepath.Walk(fs.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理文件
		if !info.IsDir() && !strings.HasSuffix(path, ".tmp") {
			// 获取相对路径
			relPath, err := filepath.Rel(fs.rootPath, path)
			if err != nil {
				return err
			}

			// 获取键名
			key := filepath.ToSlash(relPath)
			keys = append(keys, key)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return keys, nil
}

// 辅助方法：获取文件路径
func (fs *FileSecureStorage) getFilePath(key string) string {
	// 对键名进行哈希以避免路径问题
	hashedKey := hashKeyName(key)

	// 使用前4个字符作为目录名，以分散文件
	dirName := hashedKey[:4]
	fileName := hashedKey[4:]

	return filepath.Join(fs.rootPath, dirName, fileName)
}

// 辅助方法：获取文件名
func (fs *FileSecureStorage) getFilename(key string) string {
	// 对键名进行哈希以避免路径问题
	hashedKey := hashKeyName(key)

	// 使用前4个字符作为目录名，以分散文件
	dirName := hashedKey[:4]
	fileName := hashedKey[4:]

	return filepath.Join(dirName, fileName)
}

// 辅助方法：获取键的锁
func (fs *FileSecureStorage) getLock(key string) *sync.RWMutex {
	// 获取或创建锁
	value, _ := fs.fileLocks.LoadOrStore(key, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

// 辅助方法：验证键名有效性
func validateKeyName(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	if strings.Contains(key, "..") {
		return fmt.Errorf("key cannot contain '..'")
	}

	return nil
}

// 辅助方法：对键名进行哈希
func hashKeyName(key string) string {
	// 简单实现，在实际应用中可以使用更安全的哈希函数
	return hex.EncodeToString([]byte(key))
}
