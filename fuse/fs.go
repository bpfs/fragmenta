package fuse

import (
	"context"
	"os"
	"sort"
	"sync"
	"time"

	"syscall"

	"github.com/seaweedfs/fuse"
	"github.com/seaweedfs/fuse/fs"
)

// FragmentaFS 实现基本的FUSE文件系统接口
type FragmentaFS struct {
	// 存储适配器
	storage StorageManager
	// 安全适配器
	security SecurityManager
	// 默认UID
	uid uint32
	// 默认GID
	gid uint32
	// 挂载点
	mountPoint string
	// 连接
	conn *fuse.Conn
	// 根目录
	root *Dir
	// 最大文件缓存大小(字节)
	maxCacheSize int64
	// 文件缓存
	fileCache map[string]*cachedFile
	// 缓存锁
	cacheLock sync.RWMutex
	// 上下文，用于取消操作
	ctx context.Context
	// 取消函数
	cancel context.CancelFunc
}

// cachedFile 表示缓存的文件内容
type cachedFile struct {
	data       []byte
	accessTime time.Time
	dirty      bool
}

// Mount 挂载Fragmenta文件系统
func Mount(mountPoint string, storageManager StorageManager, securityManager SecurityManager) (*FragmentaFS, error) {
	// 检查挂载点是否存在
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		// 创建挂载点目录
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return nil, err
		}
	}

	// 简化挂载选项，使用最基本的挂载设置
	conn, err := fuse.Mount(
		mountPoint,
		fuse.FSName("fragmenta"),
		fuse.Subtype("fragmentafs"),
	)
	if err != nil {
		return nil, err
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建文件系统实例，使用更简化的根节点初始化
	fragmentaFS := &FragmentaFS{
		storage:      storageManager,
		security:     securityManager,
		mountPoint:   mountPoint,
		conn:         conn,
		maxCacheSize: 64 * 1024 * 1024, // 默认64MB缓存
		fileCache:    make(map[string]*cachedFile),
		ctx:          ctx,
		cancel:       cancel,
	}

	// 创建根目录节点
	fragmentaFS.root = &Dir{
		fs:    fragmentaFS,
		path:  "/",
		mode:  0755 | os.ModeDir,
		mtime: time.Now(),
		atime: time.Now(),
		ctime: time.Now(),
	}

	return fragmentaFS, nil
}

// NewDir 创建一个新的目录节点
func NewDir(path string, uid, gid uint32, mode os.FileMode) *Dir {
	return &Dir{
		path: path,
		uid:  uid,
		gid:  gid,
		mode: mode,
	}
}

// initRootDirectory 初始化根目录
func (fs *FragmentaFS) initRootDirectory() error {
	// 检查根目录是否存在
	_, err := fs.storage.GetInfo(fs.ctx, "/")
	if err != nil {
		if os.IsNotExist(err) {
			// 创建根目录
			return fs.storage.CreateDirectory(fs.ctx, "/")
		}
		return err
	}
	return nil
}

// Serve 开始服务FUSE请求
func (f *FragmentaFS) Serve() error {
	// 启动后台缓存清理
	go f.cacheCleanupLoop()

	// 处理FUSE请求
	return fs.Serve(f.conn, f)
}

// Root 是FUSE文件系统接口的必要实现，返回根目录节点
func (f *FragmentaFS) Root() (fs.Node, error) {
	// 创建根目录节点
	return &Dir{
		fs:     f,
		path:   "/",
		parent: nil,
	}, nil
}

// Unmount 卸载文件系统
func (f *FragmentaFS) Unmount() error {
	// 取消所有操作
	f.cancel()

	// 刷新缓存
	if err := f.flushCache(); err != nil {
		return err
	}

	// 卸载FUSE文件系统
	return fuse.Unmount(f.mountPoint)
}

// Close 关闭文件系统
func (f *FragmentaFS) Close() error {
	// 卸载文件系统
	if err := f.Unmount(); err != nil {
		return err
	}

	// 关闭连接
	return f.conn.Close()
}

// flushCache 将所有脏缓存写入存储
func (f *FragmentaFS) flushCache() error {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()

	for path, cache := range f.fileCache {
		if cache.dirty {
			// TODO: 将文件内容写入存储引擎
			// 将path转换为存储ID并保存
			_ = path // 标记变量已使用
			cache.dirty = false
		}
	}

	return nil
}

// cacheCleanupLoop 定期清理过期缓存
func (f *FragmentaFS) cacheCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.cleanupCache()
		case <-f.ctx.Done():
			return
		}
	}
}

// cleanupCache 清理过期缓存
func (f *FragmentaFS) cleanupCache() {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()

	// 首先尝试写回脏数据
	var totalSize int64
	now := time.Now()

	// 计算总缓存大小并找出过期项
	for path, cache := range f.fileCache {
		totalSize += int64(len(cache.data))

		// 如果文件脏且超过30分钟未访问，强制写回
		if cache.dirty && now.Sub(cache.accessTime) > 30*time.Minute {
			// TODO: 将文件内容写入存储引擎
			_ = path // 标记变量已使用
			cache.dirty = false
		}
	}

	// 如果总缓存大小超过限制，清理最旧的缓存
	if totalSize > f.maxCacheSize {
		// 按访问时间排序
		type cacheEntry struct {
			path string
			time time.Time
		}

		entries := make([]cacheEntry, 0, len(f.fileCache))
		for path, cache := range f.fileCache {
			if !cache.dirty { // 只考虑非脏缓存
				entries = append(entries, cacheEntry{path, cache.accessTime})
			}
		}

		// 排序，最旧的在前面
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].time.Before(entries[j].time)
		})

		// 删除足够多的缓存，使总大小降至限制以下
		for _, entry := range entries {
			if totalSize <= f.maxCacheSize {
				break
			}

			cache := f.fileCache[entry.path]
			totalSize -= int64(len(cache.data))
			delete(f.fileCache, entry.path)
		}
	}
}

// Node 是通用的文件系统节点接口，包含了目录和文件共有的方法
type Node interface {
	fs.Node
	Path() string
}

// Attr 填充通用的属性信息
func fillAttr(ctx context.Context, fs *FragmentaFS, path string, a *fuse.Attr) error {
	// 获取文件信息
	info, err := fs.storage.GetInfo(ctx, path)
	if err != nil {
		return err
	}

	// 填充基本属性
	a.Valid = time.Hour
	a.Inode = uint64(info.Inode)
	a.Size = uint64(info.Size)
	a.Blocks = (uint64(info.Size) + 511) / 512
	a.Atime = time.Unix(info.AccessedAt, 0)
	a.Mtime = time.Unix(info.ModifiedAt, 0)
	a.Ctime = time.Unix(info.CreatedAt, 0)
	a.Mode = info.Mode
	a.Nlink = 1
	a.Uid = info.UID
	a.Gid = info.GID

	// 如果UID/GID为0，使用默认值
	if a.Uid == 0 {
		a.Uid = fs.uid
	}
	if a.Gid == 0 {
		a.Gid = fs.gid
	}

	return nil
}

// ToFuseError 将Go错误转换为FUSE错误码
func ToFuseError(err error) error {
	if err == nil {
		return nil
	}

	// 根据错误类型转换
	switch {
	case err == syscall.ENOENT:
		return fuse.ENOENT
	case err == syscall.EACCES || err == syscall.EPERM:
		return fuse.EPERM
	case err == syscall.EEXIST:
		return fuse.EEXIST
	case err == syscall.EINVAL:
		return fuse.EIO // 使用EIO替代不存在的EINVAL
	case err == syscall.EIO:
		return fuse.EIO
	case err == syscall.ENOTDIR:
		return fuse.EIO // 使用EIO替代不存在的ENOTDIR
	case err == syscall.EISDIR:
		return fuse.EIO // 使用EIO替代不存在的EISDIR
	}

	return fuse.EIO
}
