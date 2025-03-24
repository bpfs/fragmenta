package fuse

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/seaweedfs/fuse"
	"github.com/seaweedfs/fuse/fs"
)

// Dir 表示文件系统中的目录节点
type Dir struct {
	// 文件系统引用
	fs *FragmentaFS
	// 目录路径
	path string
	// 父目录
	parent *Dir
	// 所有者用户ID
	uid uint32
	// 所有者组ID
	gid uint32
	// 权限模式
	mode os.FileMode
	// 修改时间
	mtime time.Time
	// 访问时间
	atime time.Time
	// 创建时间
	ctime time.Time
	// 子节点缓存
	children map[string]fs.Node
	// 子节点锁
	childrenLock sync.RWMutex
}

// Path 实现Node接口，返回目录路径
func (d *Dir) Path() string {
	return d.path
}

// Attr 实现fs.Node接口，返回目录属性
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	// 使用通用函数填充属性
	if err := fillAttr(ctx, d.fs, d.path, a); err != nil {
		return ToFuseError(err)
	}

	// 确保是目录类型
	a.Mode |= os.ModeDir

	return nil
}

// Lookup 实现fs.NodeStringLookuper接口，在目录中查找项目
func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	// 构建完整路径
	path := filepath.Join(d.path, name)

	// 检查文件是否存在
	exists, err := d.fs.storage.Exists(ctx, path)
	if err != nil {
		return nil, ToFuseError(err)
	}

	if !exists {
		return nil, fuse.ENOENT
	}

	// 获取文件信息
	info, err := d.fs.storage.GetInfo(ctx, path)
	if err != nil {
		return nil, ToFuseError(err)
	}

	// 根据类型创建相应节点
	if info.IsDir {
		return &Dir{
			fs:     d.fs,
			path:   path,
			parent: d,
		}, nil
	}

	// 否则是文件
	return &File{
		fs:   d.fs,
		path: path,
		dir:  d,
	}, nil
}

// ReadDirAll 实现fs.HandleReadDirAller接口，列出目录内容
func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	// 获取目录内容
	entries, err := d.fs.storage.ListDirectory(ctx, d.path)
	if err != nil {
		return nil, ToFuseError(err)
	}

	// 转换为FUSE目录项
	dirents := make([]fuse.Dirent, 0, len(entries))
	for _, entry := range entries {
		// 提取文件名部分
		name := filepath.Base(entry.Path)

		// 确定类型
		var typ fuse.DirentType
		if entry.IsDir {
			typ = fuse.DT_Dir
		} else {
			typ = fuse.DT_File
		}

		dirents = append(dirents, fuse.Dirent{
			Name:  name,
			Type:  typ,
			Inode: uint64(entry.Inode),
		})
	}

	return dirents, nil
}

// Create 实现fs.NodeCreater接口，创建新文件
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	// 检查写入权限
	if !d.fs.security.CheckWritePermission(ctx, d.path, req.Uid, req.Gid) {
		return nil, nil, fuse.EPERM
	}

	// 构建文件路径
	path := filepath.Join(d.path, req.Name)

	// 检查文件是否已存在
	exists, err := d.fs.storage.Exists(ctx, path)
	if err != nil {
		return nil, nil, ToFuseError(err)
	}

	if exists {
		return nil, nil, fuse.EEXIST
	}

	// 创建文件节点
	file := &File{
		fs:    d.fs,
		path:  path,
		dir:   d,
		uid:   req.Header.Uid,
		gid:   req.Header.Gid,
		mode:  req.Mode,
		mtime: time.Now(),
		atime: time.Now(),
		ctime: time.Now(),
		size:  0,
		data:  []byte{},
	}

	// 写入初始数据
	if err := d.fs.storage.WriteFile(ctx, path, []byte{}); err != nil {
		return nil, nil, ToFuseError(err)
	}

	// 更新元数据
	info := &FileInfo{
		Path:       path,
		IsDir:      false,
		Size:       0,
		UID:        req.Header.Uid,
		GID:        req.Header.Gid,
		Mode:       req.Mode,
		CreatedAt:  file.ctime.Unix(),
		ModifiedAt: file.mtime.Unix(),
		AccessedAt: file.atime.Unix(),
	}

	// 更新元数据
	if err := d.fs.storage.UpdateMetadata(ctx, path, info); err != nil {
		// 如果元数据更新失败，尝试删除已创建的文件
		d.fs.storage.Delete(ctx, path)
		return nil, nil, ToFuseError(err)
	}

	// 设置响应
	if err := fillAttr(ctx, d.fs, path, &resp.Attr); err != nil {
		return nil, nil, ToFuseError(err)
	}

	// 增加引用计数
	file.handles = 1

	return file, file, nil
}

// Mkdir 实现fs.NodeMkdirer接口，创建子目录
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	// 检查权限
	if !d.fs.security.CheckWritePermission(ctx, d.path, req.Uid, req.Gid) {
		return nil, fuse.EPERM
	}

	// 构建目录路径
	path := filepath.Join(d.path, req.Name)

	// 检查是否已存在
	exists, err := d.fs.storage.Exists(ctx, path)
	if err != nil {
		return nil, ToFuseError(err)
	}

	if exists {
		return nil, fuse.EEXIST
	}

	// 创建目录
	if err := d.fs.storage.CreateDirectory(ctx, path); err != nil {
		return nil, ToFuseError(err)
	}

	// 创建目录节点
	return &Dir{
		fs:     d.fs,
		path:   path,
		parent: d,
	}, nil
}

// Remove 实现fs.NodeRemover接口，删除目录项
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	// 检查权限
	if !d.fs.security.CheckWritePermission(ctx, d.path, req.Uid, req.Gid) {
		return fuse.EPERM
	}

	// 构建路径
	path := filepath.Join(d.path, req.Name)

	// 获取文件信息
	info, err := d.fs.storage.GetInfo(ctx, path)
	if err != nil {
		return ToFuseError(err)
	}

	// 如果是非空目录且未指定目录标志，返回错误
	if info.IsDir && !req.Dir {
		return fuse.EIO // 使用EIO替代不存在的EISDIR
	}

	// 删除文件或目录
	if err := d.fs.storage.Delete(ctx, path); err != nil {
		return ToFuseError(err)
	}

	return nil
}

// Rename 实现fs.NodeRenamer接口，重命名文件或目录
func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	// 检查源目录权限
	if !d.fs.security.CheckWritePermission(ctx, d.path, req.Uid, req.Gid) {
		return fuse.EPERM
	}

	// 获取目标目录
	targetDir, ok := newDir.(*Dir)
	if !ok {
		return fuse.EIO // 使用EIO替代不存在的ENOTDIR
	}

	// 检查目标目录权限
	if !d.fs.security.CheckWritePermission(ctx, targetDir.path, req.Uid, req.Gid) {
		return fuse.EPERM
	}

	// 构建源路径和目标路径
	oldPath := filepath.Join(d.path, req.OldName)
	newPath := filepath.Join(targetDir.path, req.NewName)

	// 检查源是否存在
	oldExists, err := d.fs.storage.Exists(ctx, oldPath)
	if err != nil {
		return ToFuseError(err)
	}

	if !oldExists {
		return fuse.ENOENT
	}

	// 检查目标是否已存在
	newExists, err := d.fs.storage.Exists(ctx, newPath)
	if err != nil {
		return ToFuseError(err)
	}

	// 如果目标已存在，先删除
	if newExists {
		if err := d.fs.storage.Delete(ctx, newPath); err != nil {
			return ToFuseError(err)
		}
	}

	// 移动/重命名
	if err := d.fs.storage.Move(ctx, oldPath, newPath); err != nil {
		return ToFuseError(err)
	}

	return nil
}

// 辅助函数，判断节点类型

// isDir 判断节点是否为目录
func isDir(node fs.Node) bool {
	if d, ok := node.(*Dir); ok {
		return d.mode&os.ModeDir != 0
	}
	return false
}

// isFile 判断节点是否为文件
func isFile(node fs.Node) bool {
	_, ok := node.(*File)
	return ok
}

// isSymlink 判断节点是否为符号链接
func isSymlink(node fs.Node) bool {
	if f, ok := node.(*File); ok {
		return f.mode&os.ModeSymlink != 0
	}
	return false
}
