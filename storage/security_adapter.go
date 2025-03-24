// security_adapter.go 存储安全适配器，集成安全与存储功能
package storage

import (
	"context"
	"fmt"

	"github.com/bpfs/fragmenta/security"
)

// StorageSecurityAdapter 存储安全适配器
type StorageSecurityAdapter struct {
	// 存储管理器
	storageManager StorageManager

	// 安全管理器
	securityManager security.SecurityManager

	// 配置
	config *StorageSecurityConfig
}

// StorageSecurityConfig 存储安全配置
type StorageSecurityConfig struct {
	// 是否启用加密
	EncryptionEnabled bool

	// 是否启用访问控制
	AccessControlEnabled bool

	// 安全配置路径
	SecurityConfigPath string

	// 密钥存储路径
	KeyStorePath string

	// 自动生成密钥
	AutoGenerateKey bool
}

// DefaultStorageSecurityConfig 返回默认存储安全配置
func DefaultStorageSecurityConfig() *StorageSecurityConfig {
	return &StorageSecurityConfig{
		EncryptionEnabled:    false,
		AccessControlEnabled: false,
		SecurityConfigPath:   "./security",
		KeyStorePath:         "./security/keys",
		AutoGenerateKey:      true,
	}
}

// NewStorageSecurityAdapter 创建存储安全适配器
func NewStorageSecurityAdapter(
	storageManager StorageManager,
	securityManager security.SecurityManager,
	config *StorageSecurityConfig,
) (*StorageSecurityAdapter, error) {
	if storageManager == nil {
		return nil, fmt.Errorf("存储管理器不能为空")
	}

	if securityManager == nil {
		return nil, fmt.Errorf("安全管理器不能为空")
	}

	if config == nil {
		config = DefaultStorageSecurityConfig()
	}

	// 创建适配器
	adapter := &StorageSecurityAdapter{
		storageManager:  storageManager,
		securityManager: securityManager,
		config:          config,
	}

	// 初始化
	if err := adapter.initialize(); err != nil {
		return nil, err
	}

	return adapter, nil
}

// initialize 初始化适配器
func (a *StorageSecurityAdapter) initialize() error {
	// 设置安全管理器
	if err := a.storageManager.SetSecurityManager(a.securityManager); err != nil {
		return fmt.Errorf("设置安全管理器失败: %w", err)
	}

	// 初始化安全管理器
	ctx := context.Background()
	if err := a.securityManager.Initialize(ctx); err != nil {
		return fmt.Errorf("初始化安全管理器失败: %w", err)
	}

	// 设置加密状态
	if err := a.storageManager.SetEncryptionEnabled(a.config.EncryptionEnabled); err != nil {
		return fmt.Errorf("设置加密状态失败: %w", err)
	}

	return nil
}

// CreateSecureStorageManager 创建安全增强的存储管理器
func CreateSecureStorageManager(storageConfig *StorageConfig, securityConfig *StorageSecurityConfig) (StorageManager, error) {
	// 创建存储管理器
	storageManager, err := NewStorageManager(storageConfig)
	if err != nil {
		return nil, fmt.Errorf("创建存储管理器失败: %w", err)
	}

	// 如果不需要安全功能，直接返回普通存储管理器
	if securityConfig == nil || (!securityConfig.EncryptionEnabled && !securityConfig.AccessControlEnabled) {
		return storageManager, nil
	}

	// 创建安全管理器
	secConfig := &security.SecurityConfig{
		EncryptionEnabled: securityConfig.EncryptionEnabled,
		DefaultAlgorithm:  security.AES256GCM,
		KeyStorePath:      securityConfig.KeyStorePath,
		AutoGenerateKey:   securityConfig.AutoGenerateKey,
	}

	securityManager, err := security.NewDefaultSecurityManager(secConfig)
	if err != nil {
		return nil, fmt.Errorf("创建安全管理器失败: %w", err)
	}

	// 创建适配器
	_, err = NewStorageSecurityAdapter(storageManager, securityManager, securityConfig)
	if err != nil {
		return nil, fmt.Errorf("创建存储安全适配器失败: %w", err)
	}

	// 返回配置好的存储管理器
	return storageManager, nil
}

// GetStorageSecurityAdapter 从已有的存储管理器中获取安全适配器
func GetStorageSecurityAdapter(storageManager StorageManager, securityConfig *StorageSecurityConfig) (*StorageSecurityAdapter, error) {
	if storageManager == nil {
		return nil, fmt.Errorf("存储管理器不能为空")
	}

	if securityConfig == nil {
		securityConfig = DefaultStorageSecurityConfig()
	}

	// 创建安全管理器
	secConfig := &security.SecurityConfig{
		EncryptionEnabled: securityConfig.EncryptionEnabled,
		DefaultAlgorithm:  security.AES256GCM,
		KeyStorePath:      securityConfig.KeyStorePath,
		AutoGenerateKey:   securityConfig.AutoGenerateKey,
	}

	securityManager, err := security.NewDefaultSecurityManager(secConfig)
	if err != nil {
		return nil, fmt.Errorf("创建安全管理器失败: %w", err)
	}

	// 创建并返回适配器
	return NewStorageSecurityAdapter(storageManager, securityManager, securityConfig)
}

// EnableEncryption 启用加密
func (a *StorageSecurityAdapter) EnableEncryption() error {
	// 更新配置
	a.config.EncryptionEnabled = true

	// 设置存储加密状态
	return a.storageManager.SetEncryptionEnabled(true)
}

// DisableEncryption 禁用加密
func (a *StorageSecurityAdapter) DisableEncryption() error {
	// 更新配置
	a.config.EncryptionEnabled = false

	// 设置存储加密状态
	return a.storageManager.SetEncryptionEnabled(false)
}

// GetSecurityManager 获取安全管理器
func (a *StorageSecurityAdapter) GetSecurityManager() security.SecurityManager {
	return a.securityManager
}

// GetStorageManager 获取存储管理器
func (a *StorageSecurityAdapter) GetStorageManager() StorageManager {
	return a.storageManager
}

// IsEncryptionEnabled 检查加密是否启用
func (a *StorageSecurityAdapter) IsEncryptionEnabled() bool {
	return a.config.EncryptionEnabled && a.storageManager.IsEncryptionEnabled()
}

// Close 关闭适配器
func (a *StorageSecurityAdapter) Close() error {
	// 关闭安全管理器
	ctx := context.Background()
	if err := a.securityManager.Shutdown(ctx); err != nil {
		return fmt.Errorf("关闭安全管理器失败: %w", err)
	}

	// 关闭存储管理器
	return a.storageManager.Close()
}
