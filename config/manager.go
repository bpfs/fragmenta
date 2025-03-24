package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"
	"sync"
	"time"
)

// DefaultConfigManager 默认配置管理器实现
type DefaultConfigManager struct {
	// 当前配置
	currentConfig *Config

	// 配置验证器
	validator ConfigValidator

	// 配置变更监听器
	listeners []ConfigChangeListener

	// 锁
	mu sync.RWMutex
}

// NewDefaultConfigManager 创建默认配置管理器
func NewDefaultConfigManager() *DefaultConfigManager {
	manager := &DefaultConfigManager{
		validator: NewDefaultConfigValidator(),
		listeners: make([]ConfigChangeListener, 0),
	}

	// 初始化为默认配置
	manager.currentConfig = manager.GetDefaultConfig()

	return manager
}

// LoadConfig 从文件加载配置
func (cm *DefaultConfigManager) LoadConfig(ctx context.Context, path string) (*Config, error) {
	// 读取配置文件
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// 解析配置
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	// 验证配置
	if err := cm.ValidateConfig(ctx, &config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	return &config, nil
}

// SaveConfig 保存配置到文件
func (cm *DefaultConfigManager) SaveConfig(ctx context.Context, config *Config, path string) error {
	// 验证配置
	if err := cm.ValidateConfig(ctx, config); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	// 序列化配置
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %v", err)
	}

	// 写入文件
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// GetDefaultConfig 获取默认配置
func (cm *DefaultConfigManager) GetDefaultConfig() *Config {
	// 计算系统默认值
	numCPU := runtime.NumCPU()

	return &Config{
		Storage: StoragePolicy{
			Mode:                 HybridMode,
			AutoConvertThreshold: 1024 * 1024 * 10, // 10MB
			BlockStrategy: BlockStrategy{
				BlockSize:            64 * 1024, // 64KB
				PreallocateBlocks:    32,
				BlockCacheSize:       256 * 1024 * 1024, // 256MB
				AllowUnalignedWrites: true,
			},
			CacheStrategy: CacheStrategy{
				MetadataCacheSize:  128 * 1024 * 1024,  // 128MB
				MetadataCacheTTL:   3600,               // 1小时
				DataCacheSize:      1024 * 1024 * 1024, // 1GB
				PrefetchStrategy:   "adaptive",
				PrefetchWindowSize: 16 * 1024 * 1024, // 16MB
			},
			Compression: CompressionSettings{
				Enabled:   true,
				Algorithm: "lz4",
				Level:     4,
				MinSize:   4 * 1024, // 4KB
			},
		},
		Performance: PerformanceConfig{
			Parallelism: ParallelismConfig{
				MaxWorkers:      numCPU * 2,
				WorkQueueLength: 1000,
				BatchSize:       64,
			},
			IO: IOConfig{
				UseDirectIO:      false,
				UseAsyncIO:       true,
				FDCacheSize:      1024,
				WriteMergeWindow: 50,
			},
			Memory: MemoryConfig{
				MaxMemoryUsage:       "4GB",
				ReclamationThreshold: 75,
				UseMemoryPool:        true,
			},
		},
		Security: SecurityPolicy{
			Encryption: EncryptionSettings{
				Enabled:   false, // 默认不启用加密
				Algorithm: "AES-256-GCM",
				KeySource: "keystore",
			},
			AccessControl: AccessControlSettings{
				Enabled: false, // 默认不启用访问控制
				Model:   "simple",
			},
		},
		Index: IndexPolicy{
			Enabled:         true,
			Types:           []string{"metadata", "content"},
			Mode:            "async",
			PersistenceMode: "hybrid",
			Fields: []IndexField{
				{Name: "TagTitle", Enable: true},
				{Name: "TagAuthor", Enable: true},
				{Name: "TagContentType", Enable: true},
			},
		},
		System: SystemConfig{
			RootPath:        "./data",
			TempPath:        "./temp",
			LogLevel:        "info",
			AutoCleanupTemp: true,
			EnableTelemetry: false,
		},
		Metadata: ConfigMetadata{
			Version:     "1.0",
			LastUpdated: time.Now(),
			Description: "Default configuration",
		},
	}
}

// ValidateConfig 验证配置有效性
func (cm *DefaultConfigManager) ValidateConfig(ctx context.Context, config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	return cm.validator.Validate(config)
}

// ApplyConfig 应用配置到系统
func (cm *DefaultConfigManager) ApplyConfig(ctx context.Context, config *Config) error {
	// 验证配置
	if err := cm.ValidateConfig(ctx, config); err != nil {
		return err
	}

	cm.mu.Lock()
	oldConfig := cm.currentConfig
	cm.currentConfig = config
	cm.mu.Unlock()

	// 通知所有监听器
	for _, listener := range cm.listeners {
		listener.OnConfigChange(oldConfig, config)
	}

	return nil
}

// GetCurrentConfig 获取当前配置
func (cm *DefaultConfigManager) GetCurrentConfig() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 返回当前配置的副本以避免并发修改
	configCopy := *cm.currentConfig
	return &configCopy
}

// RegisterConfigChangeListener 注册配置变更监听器
func (cm *DefaultConfigManager) RegisterConfigChangeListener(listener ConfigChangeListener) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.listeners = append(cm.listeners, listener)
}
