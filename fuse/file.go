package fuse

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seaweedfs/fuse"
	"github.com/seaweedfs/fuse/fs"
)

// File 表示文件系统中的文件节点
type File struct {
	// 文件系统引用
	fs *FragmentaFS
	// 文件路径
	path string
	// 所属目录
	dir *Dir
	// 打开的句柄数
	handles int32
	// 文件锁
	lock sync.RWMutex
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
	// 文件大小
	size uint64
	// 文件数据
	data []byte
	// 数据锁
	dataLock sync.RWMutex
	// 是否已修改
	dirty bool
}

// Path 实现Node接口，返回文件路径
func (f *File) Path() string {
	return f.path
}

// Attr 实现fs.Node接口，返回文件属性
func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	f.dataLock.RLock()
	defer f.dataLock.RUnlock()

	// 使用通用函数填充属性
	if err := fillAttr(ctx, f.fs, f.path, a); err != nil {
		return ToFuseError(err)
	}

	return nil
}

// Open 实现fs.NodeOpener接口，打开文件
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	// 增加句柄计数
	atomic.AddInt32(&f.handles, 1)

	// 更新访问时间
	f.atime = time.Now()

	// 设置响应标志，不使用DirectIO
	// resp.Flags = fuse.OpenResponseFlags{
	// 	Direct: true,
	// }

	return f, nil
}

// Read 实现HandleReader接口，读取文件内容
func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	f.dataLock.RLock()
	defer f.dataLock.RUnlock()

	// 从本地缓存读取
	if len(f.data) > 0 {
		if req.Offset < int64(len(f.data)) {
			end := req.Offset + int64(req.Size)
			if end > int64(len(f.data)) {
				end = int64(len(f.data))
			}
			resp.Data = make([]byte, end-req.Offset)
			copy(resp.Data, f.data[req.Offset:end])
			return nil
		}
		return nil
	}

	// 从存储读取
	data, err := f.fs.storage.ReadFile(ctx, f.path)
	if err != nil {
		return ToFuseError(err)
	}

	// 如果请求的偏移量超过文件大小，返回空数据
	if req.Offset >= int64(len(data)) {
		resp.Data = nil
		return nil
	}

	// 计算返回数据的范围
	end := req.Offset + int64(req.Size)
	if end > int64(len(data)) {
		end = int64(len(data))
	}

	// 复制数据
	resp.Data = make([]byte, end-req.Offset)
	copy(resp.Data, data[req.Offset:end])

	return nil
}

// Write 实现HandleWriter接口，写入文件内容
func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	f.dataLock.Lock()
	defer f.dataLock.Unlock()

	// 获取当前文件内容
	var existingData []byte
	if len(f.data) > 0 {
		existingData = f.data
	} else {
		var err error
		existingData, err = f.fs.storage.ReadFile(ctx, f.path)
		if err != nil && err != io.EOF {
			return ToFuseError(err)
		}
	}

	// 如果写入位置超出文件末尾，需要扩展文件
	writeEnd := req.Offset + int64(len(req.Data))
	if writeEnd > int64(len(existingData)) {
		// 创建新的较大缓冲区
		newData := make([]byte, writeEnd)
		// 复制原有数据
		copy(newData, existingData)
		existingData = newData
	}

	// 写入新数据
	copy(existingData[req.Offset:], req.Data)

	// 更新缓存
	f.data = existingData
	f.dirty = true
	f.mtime = time.Now()

	// 更新大小
	f.size = uint64(len(existingData))

	// 设置响应中的写入字节数
	resp.Size = len(req.Data)

	return nil
}

// Flush 实现HandleFlusher接口，文件关闭时调用
func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	// 如果文件已修改，写回存储
	f.dataLock.Lock()
	defer f.dataLock.Unlock()

	if f.dirty {
		if err := f.fs.storage.WriteFile(ctx, f.path, f.data); err != nil {
			return ToFuseError(err)
		}
		f.dirty = false
	}

	return nil
}

// Fsync 实现HandleFsyncer接口，刷新文件内容到存储
func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	f.dataLock.Lock()
	defer f.dataLock.Unlock()

	if f.dirty {
		// 写回文件内容
		if err := f.fs.storage.WriteFile(ctx, f.path, f.data); err != nil {
			return ToFuseError(err)
		}
		f.dirty = false

		// 更新元数据
		if err := f.updateMetadata(ctx); err != nil {
			return ToFuseError(err)
		}
	}

	return nil
}

// Release 实现HandleReleaser接口，关闭文件
func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	// 减少句柄计数
	newHandles := atomic.AddInt32(&f.handles, -1)

	// 如果没有打开的句柄，刷新缓存到存储并释放内存
	if newHandles <= 0 {
		err := f.Flush(ctx, &fuse.FlushRequest{})
		f.dataLock.Lock()
		f.data = nil // 释放内存
		f.dataLock.Unlock()
		return err
	}

	return nil
}

// Setattr 实现NodeSetattrer接口，修改文件属性
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	f.dataLock.Lock()
	defer f.dataLock.Unlock()

	// 获取现有文件内容
	var existingData []byte
	if len(f.data) > 0 {
		existingData = f.data
	} else {
		var err error
		existingData, err = f.fs.storage.ReadFile(ctx, f.path)
		if err != nil && err != io.EOF {
			return ToFuseError(err)
		}
	}

	// 处理大小改变
	if req.Valid.Size() {
		if uint64(req.Size) < uint64(len(existingData)) {
			// 截断
			existingData = existingData[:req.Size]
		} else if uint64(req.Size) > uint64(len(existingData)) {
			// 扩展
			newData := make([]byte, req.Size)
			copy(newData, existingData)
			existingData = newData
		}
		f.data = existingData
		f.size = uint64(len(existingData))
		f.dirty = true
	}

	// 更新时间
	now := time.Now()
	if req.Valid.MtimeNow() {
		f.mtime = now
	} else if req.Valid.Mtime() {
		f.mtime = req.Mtime
	}

	if req.Valid.AtimeNow() {
		f.atime = now
	} else if req.Valid.Atime() {
		f.atime = req.Atime
	}

	// 返回更新后的属性
	return fillAttr(ctx, f.fs, f.path, &resp.Attr)
}

// NewFile 创建一个新的文件节点
func NewFile(path string, uid, gid uint32, mode os.FileMode) *File {
	now := time.Now()
	return &File{
		path:  path,
		uid:   uid,
		gid:   gid,
		mode:  mode,
		mtime: now,
		atime: now,
		ctime: now,
	}
}

// Truncate 实现文件截断
func (f *File) Truncate(ctx context.Context, size uint64) error {
	f.dataLock.Lock()
	defer f.dataLock.Unlock()

	// 获取现有文件内容
	var existingData []byte
	if len(f.data) > 0 {
		existingData = f.data
	} else {
		var err error
		existingData, err = f.fs.storage.ReadFile(ctx, f.path)
		if err != nil && err != io.EOF {
			return ToFuseError(err)
		}
	}

	// 调整大小
	if uint64(len(existingData)) != size {
		newData := make([]byte, size)
		copyLen := size
		if copyLen > uint64(len(existingData)) {
			copyLen = uint64(len(existingData))
		}
		copy(newData, existingData[:copyLen])
		existingData = newData
	}

	// 更新文件内容和状态
	f.data = existingData
	f.size = size
	f.dirty = true
	f.mtime = time.Now()

	return nil
}

// readAll 读取整个文件内容
func (f *File) readAll() ([]byte, error) {
	f.dataLock.RLock()
	defer f.dataLock.RUnlock()

	// 如果已有数据缓存，直接返回
	if len(f.data) > 0 {
		return f.data, nil
	}

	// 否则从存储读取
	data, err := f.fs.storage.ReadFile(context.Background(), f.path)
	if err != nil {
		return nil, err
	}

	// 缓存读取的数据
	f.dataLock.RUnlock()
	f.dataLock.Lock()
	f.data = data
	f.dataLock.Unlock()
	f.dataLock.RLock()

	return data, nil
}

// updateMetadata 更新文件元数据并应用到存储
func (f *File) updateMetadata(ctx context.Context) error {
	// 准备元数据
	info := &FileInfo{
		Path:       f.path,
		IsDir:      false,
		Size:       int64(f.size),
		Inode:      0, // 由存储适配器分配
		UID:        f.uid,
		GID:        f.gid,
		Mode:       f.mode,
		CreatedAt:  f.ctime.Unix(),
		ModifiedAt: f.mtime.Unix(),
		AccessedAt: f.atime.Unix(),
	}

	// 获取现有元数据，保留inode
	existingInfo, err := f.fs.storage.GetInfo(ctx, f.path)
	if err == nil && existingInfo != nil {
		info.Inode = existingInfo.Inode
	}

	// 使用存储适配器的UpdateMetadata方法更新元数据
	return f.fs.storage.UpdateMetadata(ctx, f.path, info)
}
