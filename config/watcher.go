package config

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher 配置文件监视器
// 监控配置文件变更并自动重新加载
type ConfigWatcher struct {
	// 配置管理器
	manager ConfigManager

	// 配置文件路径
	configPath string

	// 文件系统通知器
	watcher *fsnotify.Watcher

	// 上次修改时间，用于防止重复触发
	lastModTime time.Time

	// 防抖时间窗口
	debounceWindow time.Duration

	// 是否允许自动应用配置
	autoApply bool

	// 上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc

	// 监听状态锁
	mu sync.RWMutex

	// 是否正在监听
	watching bool
}

// NewConfigWatcher 创建配置监视器
func NewConfigWatcher(manager ConfigManager) (*ConfigWatcher, error) {
	// 创建文件系统监视器
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ConfigWatcher{
		manager:        manager,
		watcher:        fsWatcher,
		debounceWindow: 500 * time.Millisecond, // 默认防抖窗口500毫秒
		autoApply:      true,                   // 默认自动应用
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// SetDebounceWindow 设置防抖时间窗口
func (cw *ConfigWatcher) SetDebounceWindow(duration time.Duration) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.debounceWindow = duration
}

// SetAutoApply 设置是否自动应用配置
func (cw *ConfigWatcher) SetAutoApply(autoApply bool) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.autoApply = autoApply
}

// WatchConfig 开始监控配置文件
func (cw *ConfigWatcher) WatchConfig(configPath string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.watching {
		return fmt.Errorf("already watching config file")
	}

	// 保存配置文件路径
	cw.configPath = configPath

	// 添加监控
	if err := cw.watcher.Add(configPath); err != nil {
		return fmt.Errorf("failed to watch config file: %v", err)
	}

	cw.watching = true

	// 启动监控协程
	go cw.watchLoop()

	log.Printf("开始监控配置文件: %s", configPath)
	return nil
}

// StopWatching 停止监控
func (cw *ConfigWatcher) StopWatching() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.watching {
		return nil
	}

	// 取消上下文
	cw.cancel()

	// 关闭监视器
	err := cw.watcher.Close()
	cw.watching = false

	log.Printf("停止监控配置文件: %s", cw.configPath)
	return err
}

// IsWatching 检查是否正在监控
func (cw *ConfigWatcher) IsWatching() bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.watching
}

// ReloadConfig 重新加载配置文件
func (cw *ConfigWatcher) ReloadConfig() error {
	cw.mu.RLock()
	configPath := cw.configPath
	autoApply := cw.autoApply
	cw.mu.RUnlock()

	if configPath == "" {
		return fmt.Errorf("no config file specified")
	}

	// 加载配置
	config, err := cw.manager.LoadConfig(context.Background(), configPath)
	if err != nil {
		return fmt.Errorf("failed to reload config: %v", err)
	}

	log.Printf("配置文件已重新加载: %s", configPath)

	// 如果设置了自动应用，则应用配置
	if autoApply {
		if err := cw.manager.ApplyConfig(context.Background(), config); err != nil {
			return fmt.Errorf("failed to apply reloaded config: %v", err)
		}
		log.Printf("新配置已应用")
	}

	return nil
}

// watchLoop 监控循环
func (cw *ConfigWatcher) watchLoop() {
	// 创建一个定时器用于防抖
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// 只处理写入和创建事件
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// 如果定时器已存在，重置它
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				// 创建防抖定时器
				cw.mu.RLock()
				debounceWindow := cw.debounceWindow
				cw.mu.RUnlock()

				debounceTimer = time.AfterFunc(debounceWindow, func() {
					if err := cw.ReloadConfig(); err != nil {
						log.Printf("配置重新加载错误: %v", err)
					}
				})
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("配置监控错误: %v", err)

		case <-cw.ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		}
	}
}

// Close 关闭监视器并释放资源
func (cw *ConfigWatcher) Close() error {
	err := cw.StopWatching()
	cw.cancel() // 确保所有协程都能退出
	return err
}
