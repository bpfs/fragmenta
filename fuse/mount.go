package fuse

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/seaweedfs/fuse"
	"github.com/seaweedfs/fuse/fs"
)

// MountOptions 挂载选项
type MountOptions struct {
	// 挂载点路径
	MountPoint string
	// 只读模式
	ReadOnly bool
	// 允许其他用户访问
	AllowOther bool
	// 允许开发模式
	Debug bool
	// 所有者UID
	UID uint32
	// 所有者GID
	GID uint32
	// 启用元数据持久化
	PersistMetadata bool
	// 元数据持久化路径
	MetadataPath string
	// 缓存大小（MB）
	CacheSizeMB int64
}

// MountManager 管理文件系统挂载
type MountManager struct {
	// 存储管理器
	storageManager StorageManager
	// 安全管理器（可选）
	securityManager SecurityManager
	// 挂载连接
	connections map[string]*mountConnection
	// 连接锁
	connLock sync.RWMutex
	// 元数据持久化选项
	metadataPersistenceEnabled bool
	// 元数据持久化路径
	metadataPersistencePath string
}

// mountConnection 表示一个挂载连接
type mountConnection struct {
	// 挂载点
	mountPoint string
	// FUSE连接
	conn *fuse.Conn
	// 文件系统接口
	filesys fs.FS
	// 存储适配器
	storage *FragmentaStorageAdapter
	// 安全适配器
	security SecurityManager
	// 服务器
	server *fs.Server
	// 错误通道
	errChan chan error
	// 关闭函数
	close func() error
}

// NewMountManager 创建新的挂载管理器
func NewMountManager(storageManager StorageManager) *MountManager {
	return &MountManager{
		storageManager: storageManager,
		connections:    make(map[string]*mountConnection),
	}
}

// SetSecurityManager 设置安全管理器
func (m *MountManager) SetSecurityManager(securityManager SecurityManager) {
	m.securityManager = securityManager
}

// EnableMetadataPersistence 启用元数据持久化
func (m *MountManager) EnableMetadataPersistence(path string) {
	m.metadataPersistenceEnabled = true
	m.metadataPersistencePath = path
}

// Mount 挂载文件系统到指定挂载点
func (m *MountManager) Mount(ctx context.Context, options MountOptions) error {
	// 检查挂载点是否已经存在
	m.connLock.RLock()
	if _, exists := m.connections[options.MountPoint]; exists {
		m.connLock.RUnlock()
		return fmt.Errorf("挂载点 %s 已被使用", options.MountPoint)
	}
	m.connLock.RUnlock()

	// 确保挂载目录存在
	if err := os.MkdirAll(options.MountPoint, 0755); err != nil {
		return fmt.Errorf("创建挂载点目录失败: %v", err)
	}

	// 在macOS上检查FUSE安装
	if runtime.GOOS == "darwin" {
		helper := &MacOSFUSEHelper{}
		if err := helper.CheckFUSEInstallation(); err != nil {
			return fmt.Errorf("macFUSE检查失败: %v", err)
		}
	}

	// 准备FUSE挂载选项
	var fuseOptions []fuse.MountOption

	if options.AllowOther {
		fuseOptions = append(fuseOptions, fuse.AllowOther())
	}

	if options.ReadOnly {
		fuseOptions = append(fuseOptions, fuse.ReadOnly())
	}

	// macOS特定选项
	if runtime.GOOS == "darwin" {
		// github.com/seaweedfs/fuse库对macOS可能支持不完整，使用字符串选项
		fuseOptions = append(fuseOptions, fuse.MountOption(fuse.FSName("Fragmenta")))
		// 其他特殊处理会在后续步骤中进行
	}

	// 设置调试输出
	if options.Debug {
		// 在挂载前设置全局Debug函数
		fuse.Debug = debugLog
	}

	// 挂载文件系统
	conn, err := fuse.Mount(options.MountPoint, fuseOptions...)
	if err != nil {
		return fmt.Errorf("挂载FUSE文件系统失败: %v", err)
	}

	// macOS Sonoma 特定处理：重新挂载以解决写入延迟问题
	if runtime.GOOS == "darwin" {
		// 使用优化的macOS挂载方法替代标准FUSE挂载
		if options.Debug {
			debugLog("检测到macOS系统，使用优化的macOS挂载方法")
		}

		// 关闭标准FUSE连接
		conn.Close()

		// 使用替代挂载器
		mounter := NewMacOSAlternativeMounter(options.MountPoint, options.Debug)
		if err := mounter.Mount(); err != nil {
			return fmt.Errorf("macOS挂载失败: %v", err)
		}

		// 重新连接
		conn, err = fuse.Mount(options.MountPoint, fuseOptions...)
		if err != nil {
			// 如果无法重新连接，尝试清理
			mounter.Unmount()
			return fmt.Errorf("重新挂载FUSE文件系统失败: %v", err)
		}
	}

	// 创建存储适配器
	storage, err := NewFragmentaStorageAdapter(m.storageManager)
	if err != nil {
		// 安全关闭连接
		conn.Close()
		return fmt.Errorf("创建存储适配器失败: %v", err)
	}

	// 如果启用了元数据持久化，尝试加载元数据
	if options.PersistMetadata && options.MetadataPath != "" {
		// 创建元数据目录
		metadataDir := options.MetadataPath
		if metadataDir == "" && m.metadataPersistencePath != "" {
			metadataDir = m.metadataPersistencePath
		}

		if metadataDir != "" {
			if err := os.MkdirAll(metadataDir, 0755); err != nil {
				conn.Close()
				return fmt.Errorf("创建元数据目录失败: %v", err)
			}

			// 加载元数据的代码将在这里实现
			// TODO: 实现元数据加载功能
		}
	}

	// 创建安全适配器
	var security SecurityManager
	if m.securityManager != nil {
		security = m.securityManager
	} else {
		// 使用默认安全适配器
		defaultSecurity, err := NewFragmentaSecurityAdapter()
		if err != nil {
			conn.Close()
			return fmt.Errorf("创建安全适配器失败: %v", err)
		}
		security = defaultSecurity
	}

	// 设置缓存大小
	cacheSizeBytes := int64(64 * 1024 * 1024) // 默认64MB
	if options.CacheSizeMB > 0 {
		cacheSizeBytes = options.CacheSizeMB * 1024 * 1024
	}

	// 创建上下文和取消函数
	mountCtx, cancelFunc := context.WithCancel(ctx)

	// 创建文件系统
	filesys := &FragmentaFS{
		storage:      storage,
		security:     security,
		uid:          options.UID,
		gid:          options.GID,
		mountPoint:   options.MountPoint,
		conn:         conn,
		maxCacheSize: cacheSizeBytes,
		fileCache:    make(map[string]*cachedFile),
		ctx:          mountCtx,
		cancel:       cancelFunc,
	}

	// 确保根目录存在
	if err := filesys.initRootDirectory(); err != nil {
		cancelFunc()
		conn.Close()
		return fmt.Errorf("初始化根目录失败: %v", err)
	}

	// 创建错误通道
	errChan := make(chan error, 1)

	// 简化卸载方法，不依赖conn状态
	closeFn := func() error {
		// 取消文件系统操作
		cancelFunc()

		// 刷新文件系统缓存
		if err := filesys.flushCache(); err != nil {
			return fmt.Errorf("刷新缓存失败: %v", err)
		}

		// 尝试直接卸载
		err := fuse.Unmount(options.MountPoint)
		if err != nil {
			// 如果失败，尝试使用系统命令卸载
			cmd := exec.Command("umount", options.MountPoint)
			return cmd.Run()
		}
		return nil
	}

	// 构建挂载连接
	mountConn := &mountConnection{
		mountPoint: options.MountPoint,
		conn:       conn,
		filesys:    filesys,
		storage:    storage,
		security:   security,
		errChan:    errChan,
		close:      closeFn,
	}

	// 添加到连接映射
	m.connLock.Lock()
	m.connections[options.MountPoint] = mountConn
	m.connLock.Unlock()

	// 启动服务
	go func() {
		// 创建服务器，不再设置Debug选项
		server := fs.New(conn, nil)
		mountConn.server = server

		err := server.Serve(filesys)
		if err != nil {
			errChan <- err
		}

		// 服务结束后通知
		errChan <- nil
	}()

	// 检查服务是否立即失败
	select {
	case err := <-errChan:
		if err != nil {
			// 出错，清理资源
			m.connLock.Lock()
			delete(m.connections, options.MountPoint)
			m.connLock.Unlock()

			conn.Close()
			closeFn()
			return fmt.Errorf("FUSE服务启动失败: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// 超过100ms没有错误，认为启动成功
	}

	return nil
}

// Unmount 卸载指定挂载点的文件系统
func (m *MountManager) Unmount(mountPoint string) error {
	// 获取挂载连接
	m.connLock.RLock()
	conn, exists := m.connections[mountPoint]
	m.connLock.RUnlock()

	if !exists {
		return fmt.Errorf("挂载点 %s 不存在", mountPoint)
	}

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用通道来协调卸载流程
	unmountDone := make(chan error, 1)

	go func() {
		// 尝试正常卸载
		err := conn.close()
		if err != nil {
			unmountDone <- err
			return
		}

		// 关闭连接
		err = conn.conn.Close()
		unmountDone <- err
	}()

	// 等待卸载完成或超时
	select {
	case err := <-unmountDone:
		// 无论成功还是失败，都从连接映射中移除
		m.connLock.Lock()
		delete(m.connections, mountPoint)
		m.connLock.Unlock()

		if err != nil {
			// 卸载失败，尝试强制卸载
			cmd := exec.Command("umount", "-f", mountPoint)
			if forceErr := cmd.Run(); forceErr != nil {
				return fmt.Errorf("卸载失败 (常规: %v, 强制: %v)", err, forceErr)
			}
		}
		return nil

	case <-ctx.Done():
		// 超时，尝试强制卸载
		cmd := exec.Command("umount", "-f", mountPoint)
		err := cmd.Run()

		// 清理连接
		m.connLock.Lock()
		delete(m.connections, mountPoint)
		m.connLock.Unlock()

		if err != nil {
			return fmt.Errorf("卸载超时，强制卸载也失败: %v", err)
		}
		return fmt.Errorf("卸载超时，已强制卸载")
	}
}

// UnmountAll 卸载所有文件系统挂载点
func (m *MountManager) UnmountAll() []error {
	var errors []error

	// 获取所有挂载点
	m.connLock.RLock()
	mountPoints := make([]string, 0, len(m.connections))
	for mountPoint := range m.connections {
		mountPoints = append(mountPoints, mountPoint)
	}
	m.connLock.RUnlock()

	// 逐个卸载
	for _, mountPoint := range mountPoints {
		if err := m.Unmount(mountPoint); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// IsMounted 检查指定路径是否已挂载
func (m *MountManager) IsMounted(mountPoint string) bool {
	// 规范化路径
	absPath, err := filepath.Abs(mountPoint)
	if err != nil {
		return false
	}

	m.connLock.RLock()
	_, exists := m.connections[absPath]
	m.connLock.RUnlock()

	return exists
}

// GetMountPoints 获取所有挂载点
func (m *MountManager) GetMountPoints() []string {
	m.connLock.RLock()
	defer m.connLock.RUnlock()

	mountPoints := make([]string, 0, len(m.connections))
	for mountPoint := range m.connections {
		mountPoints = append(mountPoints, mountPoint)
	}

	return mountPoints
}

// debugLog 用于FUSE调试日志输出
func debugLog(msg interface{}) {
	fmt.Printf("FUSE调试: %v\n", msg)
}
