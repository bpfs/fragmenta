package fuse

import (
	"context"
	"os"
)

// 定义与存储引擎和安全系统交互所需的接口，简化依赖管理

// FUSEProvider 提供FUSE挂载实现
type FUSEProvider interface {
	// Name 返回提供者名称
	Name() string

	// Mount 挂载文件系统
	Mount(mountPoint string, options MountOptions) error

	// Unmount 卸载文件系统
	Unmount(mountPoint string) error
}

// FragmentaStorageAdapter 实现此接口
type StorageManager interface {
	// 读取文件内容
	ReadFile(ctx context.Context, path string) ([]byte, error)

	// 写入文件内容
	WriteFile(ctx context.Context, path string, data []byte) error

	// 创建目录
	CreateDirectory(ctx context.Context, path string) error

	// 删除文件或目录
	Delete(ctx context.Context, path string) error

	// 获取文件或目录信息
	GetInfo(ctx context.Context, path string) (*FileInfo, error)

	// 列出目录内容
	ListDirectory(ctx context.Context, path string) ([]FileInfo, error)

	// 移动或重命名文件/目录
	Move(ctx context.Context, oldPath, newPath string) error

	// 文件是否存在
	Exists(ctx context.Context, path string) (bool, error)

	// 更新文件或目录的元数据
	UpdateMetadata(ctx context.Context, path string, info *FileInfo) error
}

// FragmentaSecurityAdapter 实现此接口
type SecurityManager interface {
	// 检查读取权限
	CheckReadPermission(ctx context.Context, path string, uid, gid uint32) bool

	// 检查写入权限
	CheckWritePermission(ctx context.Context, path string, uid, gid uint32) bool

	// 检查执行权限
	CheckExecutePermission(ctx context.Context, path string, uid, gid uint32) bool

	// 获取文件权限
	GetPermissions(ctx context.Context, path string) (os.FileMode, error)

	// 设置文件权限
	SetPermissions(ctx context.Context, path string, mode os.FileMode) error
}

// FileInfo 文件信息结构
type FileInfo struct {
	// 文件路径
	Path string

	// 是否是目录
	IsDir bool

	// 文件大小
	Size int64

	// Inode编号，用于FUSE文件系统
	Inode uint32

	// 所有者用户ID
	UID uint32

	// 所有者组ID
	GID uint32

	// 文件权限模式
	Mode os.FileMode

	// 创建时间戳
	CreatedAt int64

	// 修改时间戳
	ModifiedAt int64

	// 访问时间戳
	AccessedAt int64
}

// PathMapper 路径映射器接口，用于在文件系统路径和存储标识符之间转换
type PathMapper interface {
	// 路径转存储ID
	PathToID(path string) (uint32, error)

	// 存储ID转路径
	IDToPath(id uint32) (string, error)

	// 存储路径映射
	StorePath(path string) (uint32, error)

	// 检索路径映射
	LoadPath(id uint32) (string, error)
}

// FSEventListener 文件系统事件监听器接口
type FSEventListener interface {
	// 文件创建事件
	OnFileCreated(path string)

	// 文件修改事件
	OnFileModified(path string)

	// 文件删除事件
	OnFileDeleted(path string)

	// 目录创建事件
	OnDirCreated(path string)

	// 目录删除事件
	OnDirDeleted(path string)
}
