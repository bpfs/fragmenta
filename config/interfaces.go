package config

import (
	"context"
)

// ConfigManager 配置管理器接口
type ConfigManager interface {
	// LoadConfig 从文件加载配置
	LoadConfig(ctx context.Context, path string) (*Config, error)

	// SaveConfig 保存配置到文件
	SaveConfig(ctx context.Context, config *Config, path string) error

	// GetDefaultConfig 获取默认配置
	GetDefaultConfig() *Config

	// ValidateConfig 验证配置有效性
	ValidateConfig(ctx context.Context, config *Config) error

	// ApplyConfig 应用配置到系统
	ApplyConfig(ctx context.Context, config *Config) error

	// GetCurrentConfig 获取当前配置
	GetCurrentConfig() *Config

	// RegisterConfigChangeListener 注册配置变更监听器
	RegisterConfigChangeListener(listener ConfigChangeListener)
}

// ConfigChangeListener 配置变更监听器
type ConfigChangeListener interface {
	// OnConfigChange 配置变更通知
	OnConfigChange(oldConfig, newConfig *Config)
}

// ConfigValidator 配置验证器接口
type ConfigValidator interface {
	// Validate 验证配置
	Validate(config *Config) error

	// ValidateSection 验证特定配置段
	ValidateSection(sectionName string, section interface{}) error
}
