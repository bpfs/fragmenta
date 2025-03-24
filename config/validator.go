package config

import (
	"fmt"
	"strings"
)

// DefaultConfigValidator 默认配置验证器实现
type DefaultConfigValidator struct {
	// 可设置验证选项
}

// NewDefaultConfigValidator 创建默认配置验证器
func NewDefaultConfigValidator() *DefaultConfigValidator {
	return &DefaultConfigValidator{}
}

// Validate 验证配置
func (v *DefaultConfigValidator) Validate(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// 验证各个部分
	if err := v.ValidateSection("storage", config.Storage); err != nil {
		return err
	}

	if err := v.ValidateSection("performance", config.Performance); err != nil {
		return err
	}

	if err := v.ValidateSection("security", config.Security); err != nil {
		return err
	}

	if err := v.ValidateSection("index", config.Index); err != nil {
		return err
	}

	if err := v.ValidateSection("system", config.System); err != nil {
		return err
	}

	return nil
}

// ValidateSection 验证特定配置段
func (v *DefaultConfigValidator) ValidateSection(sectionName string, section interface{}) error {
	switch sectionName {
	case "storage":
		return v.validateStoragePolicy(section.(StoragePolicy))
	case "performance":
		return v.validatePerformanceConfig(section.(PerformanceConfig))
	case "security":
		return v.validateSecurityPolicy(section.(SecurityPolicy))
	case "index":
		return v.validateIndexPolicy(section.(IndexPolicy))
	case "system":
		return v.validateSystemConfig(section.(SystemConfig))
	default:
		return fmt.Errorf("unknown configuration section: %s", sectionName)
	}
}

// 验证存储策略
func (v *DefaultConfigValidator) validateStoragePolicy(policy StoragePolicy) error {
	// 验证存储模式
	if !v.isValidStorageMode(policy.Mode) {
		return fmt.Errorf("invalid storage mode: %s", policy.Mode)
	}

	// 验证自动转换阈值
	if policy.AutoConvertThreshold < 0 {
		return fmt.Errorf("auto convert threshold cannot be negative")
	}

	// 验证块大小(应为2的幂)
	if !v.isPowerOfTwo(policy.BlockStrategy.BlockSize) {
		return fmt.Errorf("block size must be a power of 2")
	}

	// 验证其他字段
	if policy.BlockStrategy.PreallocateBlocks < 0 {
		return fmt.Errorf("preallocate blocks cannot be negative")
	}

	if policy.CacheStrategy.MetadataCacheTTL < 0 {
		return fmt.Errorf("metadata cache TTL cannot be negative")
	}

	// 验证压缩设置
	if policy.Compression.Enabled && policy.Compression.Level < 0 {
		return fmt.Errorf("compression level cannot be negative")
	}

	return nil
}

// 验证性能配置
func (v *DefaultConfigValidator) validatePerformanceConfig(config PerformanceConfig) error {
	// 验证并行设置
	if config.Parallelism.MaxWorkers <= 0 {
		return fmt.Errorf("max workers must be positive")
	}

	if config.Parallelism.WorkQueueLength <= 0 {
		return fmt.Errorf("work queue length must be positive")
	}

	// 验证内存设置
	if config.Memory.ReclamationThreshold < 0 || config.Memory.ReclamationThreshold > 100 {
		return fmt.Errorf("reclamation threshold must be between 0 and 100")
	}

	// 验证IO设置
	if config.IO.FDCacheSize < 0 {
		return fmt.Errorf("FD cache size cannot be negative")
	}

	return nil
}

// 验证安全策略
func (v *DefaultConfigValidator) validateSecurityPolicy(policy SecurityPolicy) error {
	// 验证加密设置
	if policy.Encryption.Enabled {
		if policy.Encryption.Algorithm == "" {
			return fmt.Errorf("encryption algorithm cannot be empty when encryption is enabled")
		}

		if policy.Encryption.KeySource == "" {
			return fmt.Errorf("key source cannot be empty when encryption is enabled")
		}
	}

	// 验证访问控制设置
	if policy.AccessControl.Enabled && policy.AccessControl.Model == "" {
		return fmt.Errorf("access control model cannot be empty when access control is enabled")
	}

	return nil
}

// 验证索引策略
func (v *DefaultConfigValidator) validateIndexPolicy(policy IndexPolicy) error {
	// 如果索引启用，验证必要字段
	if policy.Enabled {
		if len(policy.Types) == 0 {
			return fmt.Errorf("at least one index type must be specified when indexing is enabled")
		}

		// 验证索引模式
		if !v.isValidIndexMode(policy.Mode) {
			return fmt.Errorf("invalid index mode: %s", policy.Mode)
		}

		// 验证持久化模式
		if !v.isValidPersistenceMode(policy.PersistenceMode) {
			return fmt.Errorf("invalid persistence mode: %s", policy.PersistenceMode)
		}
	}

	return nil
}

// 验证系统配置
func (v *DefaultConfigValidator) validateSystemConfig(config SystemConfig) error {
	// 验证路径不为空
	if config.RootPath == "" {
		return fmt.Errorf("root path cannot be empty")
	}

	if config.TempPath == "" {
		return fmt.Errorf("temp path cannot be empty")
	}

	// 验证日志级别
	if !v.isValidLogLevel(config.LogLevel) {
		return fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	return nil
}

// 辅助方法：验证存储模式是否有效
func (v *DefaultConfigValidator) isValidStorageMode(mode StorageMode) bool {
	return mode == ContainerMode || mode == DirectoryMode || mode == HybridMode
}

// 辅助方法：验证索引模式是否有效
func (v *DefaultConfigValidator) isValidIndexMode(mode string) bool {
	validModes := []string{"sync", "async", "manual"}
	return v.isStringInSlice(mode, validModes)
}

// 辅助方法：验证持久化模式是否有效
func (v *DefaultConfigValidator) isValidPersistenceMode(mode string) bool {
	validModes := []string{"memory", "disk", "hybrid"}
	return v.isStringInSlice(mode, validModes)
}

// 辅助方法：验证日志级别是否有效
func (v *DefaultConfigValidator) isValidLogLevel(level string) bool {
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	return v.isStringInSlice(strings.ToLower(level), validLevels)
}

// 辅助方法：检查字符串是否在切片中
func (v *DefaultConfigValidator) isStringInSlice(value string, slice []string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// 辅助方法：检查整数是否为2的幂
func (v *DefaultConfigValidator) isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}
