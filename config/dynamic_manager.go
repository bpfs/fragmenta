package config

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// DynamicConfigManager 动态配置管理器
// 扩展默认配置管理器，增加配置动态更新功能
type DynamicConfigManager struct {
	// 基础配置管理器
	baseManager *DefaultConfigManager

	// 配置监视器
	watcher *ConfigWatcher

	// 配置变更日志记录器
	changeLogger *ConfigChangeLogger

	// 配置文件路径
	configPath string

	// 监听器分组映射
	listenerGroups map[string][]ConfigChangeListener

	// 锁
	mu sync.RWMutex

	// 是否已初始化
	initialized bool

	// 是否启用变更日志
	enableChangeLog bool
}

// NewDynamicConfigManager 创建动态配置管理器
func NewDynamicConfigManager() (*DynamicConfigManager, error) {
	baseManager := NewDefaultConfigManager()

	// 创建配置监视器
	watcher, err := NewConfigWatcher(baseManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create config watcher: %v", err)
	}

	return &DynamicConfigManager{
		baseManager:     baseManager,
		watcher:         watcher,
		listenerGroups:  make(map[string][]ConfigChangeListener),
		enableChangeLog: false,
	}, nil
}

// InitWithConfigFile 使用配置文件初始化
func (dm *DynamicConfigManager) InitWithConfigFile(ctx context.Context, configPath string, watchChanges bool) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 保存配置路径
	dm.configPath = configPath

	// 加载配置
	config, err := dm.baseManager.LoadConfig(ctx, configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %v", err)
	}

	// 应用配置
	if err := dm.baseManager.ApplyConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to apply config: %v", err)
	}

	// 如果启用了监视，则开始监视配置文件
	if watchChanges {
		if err := dm.watcher.WatchConfig(configPath); err != nil {
			return fmt.Errorf("failed to watch config file: %v", err)
		}
	}

	dm.initialized = true
	return nil
}

// LoadConfig 从文件加载配置
func (dm *DynamicConfigManager) LoadConfig(ctx context.Context, path string) (*Config, error) {
	return dm.baseManager.LoadConfig(ctx, path)
}

// SaveConfig 保存配置到文件
func (dm *DynamicConfigManager) SaveConfig(ctx context.Context, config *Config, path string) error {
	return dm.baseManager.SaveConfig(ctx, config, path)
}

// GetDefaultConfig 获取默认配置
func (dm *DynamicConfigManager) GetDefaultConfig() *Config {
	return dm.baseManager.GetDefaultConfig()
}

// ValidateConfig 验证配置有效性
func (dm *DynamicConfigManager) ValidateConfig(ctx context.Context, config *Config) error {
	return dm.baseManager.ValidateConfig(ctx, config)
}

// ApplyConfig 应用配置到系统
func (dm *DynamicConfigManager) ApplyConfig(ctx context.Context, config *Config) error {
	// 记录配置变更（如果启用）
	if dm.enableChangeLog && dm.changeLogger != nil {
		oldConfig := dm.GetCurrentConfig()

		// 应用配置
		err := dm.baseManager.ApplyConfig(ctx, config)
		if err != nil {
			return err
		}

		// 记录手动应用的配置变更
		if err := dm.changeLogger.LogConfigChange(
			oldConfig,
			config,
			"手动应用",
			"通过ApplyConfig方法手动应用配置",
		); err != nil {
			log.Printf("记录手动配置变更失败: %v", err)
		}

		return nil
	}

	// 未启用日志，直接应用配置
	return dm.baseManager.ApplyConfig(ctx, config)
}

// GetCurrentConfig 获取当前配置
func (dm *DynamicConfigManager) GetCurrentConfig() *Config {
	return dm.baseManager.GetCurrentConfig()
}

// RegisterConfigChangeListener 注册配置变更监听器
func (dm *DynamicConfigManager) RegisterConfigChangeListener(listener ConfigChangeListener) {
	dm.baseManager.RegisterConfigChangeListener(listener)
}

// RegisterGroupedListener 注册分组监听器
func (dm *DynamicConfigManager) RegisterGroupedListener(groupName string, listener ConfigChangeListener) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 如果组不存在，创建新组
	if _, exists := dm.listenerGroups[groupName]; !exists {
		dm.listenerGroups[groupName] = make([]ConfigChangeListener, 0)
	}

	// 添加监听器到组
	dm.listenerGroups[groupName] = append(dm.listenerGroups[groupName], listener)

	// 同时注册到基础管理器
	dm.baseManager.RegisterConfigChangeListener(listener)
}

// UnregisterGroup 注销整个监听器组
func (dm *DynamicConfigManager) UnregisterGroup(groupName string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if group, exists := dm.listenerGroups[groupName]; exists {
		// 从基础列表中移除所有组内监听器
		// 注意：这需要扩展基础配置管理器以支持注销功能
		// 这里仅从组映射中删除
		delete(dm.listenerGroups, groupName)
		return len(group) > 0
	}

	return false
}

// IsWatchingConfig 检查是否正在监控配置文件
func (dm *DynamicConfigManager) IsWatchingConfig() bool {
	return dm.watcher.IsWatching()
}

// StartWatching 开始监控配置文件
func (dm *DynamicConfigManager) StartWatching() error {
	dm.mu.RLock()
	configPath := dm.configPath
	dm.mu.RUnlock()

	if configPath == "" {
		return fmt.Errorf("no config file path set")
	}

	return dm.watcher.WatchConfig(configPath)
}

// StopWatching 停止监控配置文件
func (dm *DynamicConfigManager) StopWatching() error {
	return dm.watcher.StopWatching()
}

// ForceReload 强制重新加载配置
func (dm *DynamicConfigManager) ForceReload(ctx context.Context) error {
	dm.mu.RLock()
	configPath := dm.configPath
	dm.mu.RUnlock()

	if configPath == "" {
		return fmt.Errorf("no config file path set")
	}

	// 加载配置
	config, err := dm.baseManager.LoadConfig(ctx, configPath)
	if err != nil {
		return fmt.Errorf("failed to reload config: %v", err)
	}

	// 应用配置
	if err := dm.baseManager.ApplyConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to apply reloaded config: %v", err)
	}

	log.Printf("配置已强制重新加载并应用: %s", configPath)
	return nil
}

// SetDebounceWindow 设置防抖时间窗口
func (dm *DynamicConfigManager) SetDebounceWindow(duration time.Duration) {
	dm.watcher.SetDebounceWindow(duration)
}

// SetAutoApply 设置是否自动应用配置
func (dm *DynamicConfigManager) SetAutoApply(autoApply bool) {
	dm.watcher.SetAutoApply(autoApply)
}

// EnableChangeLog 启用配置变更日志
func (dm *DynamicConfigManager) EnableChangeLog(logDir string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 创建变更日志记录器
	logger, err := NewConfigChangeLogger(logDir)
	if err != nil {
		return fmt.Errorf("failed to create change logger: %v", err)
	}

	// 如果之前有日志记录器，关闭它
	if dm.changeLogger != nil {
		if err := dm.changeLogger.Close(); err != nil {
			log.Printf("警告: 关闭旧的变更日志记录器失败: %v", err)
		}
	}

	dm.changeLogger = logger
	dm.enableChangeLog = true

	// 注册一个特殊的监听器，用于记录配置变更
	dm.baseManager.RegisterConfigChangeListener(&configChangeLoggerListener{
		manager: dm,
	})

	return nil
}

// DisableChangeLog 禁用配置变更日志
func (dm *DynamicConfigManager) DisableChangeLog() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.changeLogger != nil {
		if err := dm.changeLogger.Close(); err != nil {
			return fmt.Errorf("failed to close change logger: %v", err)
		}
		dm.changeLogger = nil
	}

	dm.enableChangeLog = false
	return nil
}

// SetLogFullConfig 设置是否记录完整配置
func (dm *DynamicConfigManager) SetLogFullConfig(logFullConfig bool) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.changeLogger != nil {
		dm.changeLogger.SetLogFullConfig(logFullConfig)
	}
}

// configChangeLoggerListener 配置变更日志监听器
type configChangeLoggerListener struct {
	manager *DynamicConfigManager
}

// OnConfigChange 实现ConfigChangeListener接口
func (l *configChangeLoggerListener) OnConfigChange(oldConfig, newConfig *Config) {
	l.manager.mu.RLock()
	logger := l.manager.changeLogger
	enabled := l.manager.enableChangeLog
	configPath := l.manager.configPath
	l.manager.mu.RUnlock()

	if !enabled || logger == nil {
		return
	}

	// 记录配置变更
	err := logger.LogConfigChange(
		oldConfig,
		newConfig,
		configPath,
		"配置文件变更",
	)

	if err != nil {
		log.Printf("记录配置变更失败: %v", err)
	}
}

// Close 关闭管理器并释放资源
func (dm *DynamicConfigManager) Close() error {
	if dm.changeLogger != nil {
		if err := dm.changeLogger.Close(); err != nil {
			log.Printf("关闭变更日志记录器失败: %v", err)
		}
		dm.changeLogger = nil
	}

	return dm.watcher.Close()
}
