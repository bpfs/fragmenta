package security

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

// DefaultSecurityManager 默认安全管理器实现
type DefaultSecurityManager struct {
	// 加密提供者
	encryptionProvider EncryptionProvider

	// 密钥管理器
	keyManager KeyManager

	// 默认密钥ID
	defaultKeyID string

	// 配置
	config *SecurityConfig

	// 锁
	mu sync.RWMutex

	// 初始化状态
	initialized bool
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	// 是否启用加密
	EncryptionEnabled bool

	// 默认加密算法
	DefaultAlgorithm EncryptionAlgorithm

	// 密钥存储路径
	KeyStorePath string

	// 自动生成密钥
	AutoGenerateKey bool
}

// NewDefaultSecurityManager 创建默认安全管理器
func NewDefaultSecurityManager(config *SecurityConfig) (*DefaultSecurityManager, error) {
	if config == nil {
		config = &SecurityConfig{
			EncryptionEnabled: false,
			DefaultAlgorithm:  AES256GCM,
			KeyStorePath:      "./keys",
			AutoGenerateKey:   false,
		}
	}

	if config.KeyStorePath == "" {
		return nil, errors.New("密钥存储路径不能为空")
	}

	// 创建文件安全存储
	secureStorage, err := NewFileSecureStorage(config.KeyStorePath)
	if err != nil {
		return nil, fmt.Errorf("创建安全存储失败: %w", err)
	}

	// 创建密钥管理器
	keyManager := NewDefaultKeyManager(secureStorage)

	// 创建加密提供者
	encryptionProvider := NewDefaultEncryptionProvider(keyManager)

	return &DefaultSecurityManager{
		encryptionProvider: encryptionProvider,
		keyManager:         keyManager,
		config:             config,
		initialized:        false,
	}, nil
}

// Initialize 初始化安全管理器
func (sm *DefaultSecurityManager) Initialize(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 已初始化判断
	if sm.initialized {
		return nil
	}

	// 如果配置为自动生成密钥且没有默认密钥
	if sm.config.AutoGenerateKey && sm.defaultKeyID == "" {
		// 查看是否已有密钥
		keys, err := sm.keyManager.ListKeys(ctx)
		if err != nil {
			return fmt.Errorf("列出密钥失败: %w", err)
		}

		if len(keys) > 0 {
			// 使用第一个密钥作为默认密钥
			sm.defaultKeyID = keys[0]
		} else {
			// 生成新密钥
			keyID, err := sm.keyManager.GenerateKey(ctx, SymmetricKey, &KeyOptions{
				Type: SymmetricKey,
				Size: 256,
				Metadata: map[string]string{
					"algorithm": string(sm.config.DefaultAlgorithm),
					"auto_gen":  "true",
				},
			})
			if err != nil {
				return fmt.Errorf("生成默认密钥失败: %w", err)
			}
			sm.defaultKeyID = keyID
		}
	}

	sm.initialized = true
	return nil
}

// IsInitialized 返回安全管理器是否已初始化
func (sm *DefaultSecurityManager) IsInitialized() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.initialized
}

// Shutdown 关闭安全子系统
func (sm *DefaultSecurityManager) Shutdown(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.initialized = false
	return nil
}

// GetEncryptionProvider 获取加密提供者
func (sm *DefaultSecurityManager) GetEncryptionProvider() EncryptionProvider {
	return sm.encryptionProvider
}

// GetKeyManager 获取密钥管理器
func (sm *DefaultSecurityManager) GetKeyManager() KeyManager {
	return sm.keyManager
}

// EncryptBlock 加密数据块
func (sm *DefaultSecurityManager) EncryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 如果加密未启用，直接返回原始数据
	if !sm.config.EncryptionEnabled {
		return data, nil
	}

	// 获取默认密钥
	keyData, err := sm.keyManager.GetKey(ctx, sm.defaultKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get default key: %w", err)
	}

	// 准备额外的关联数据（AAD）
	associatedData := make([]byte, 4)
	binary.BigEndian.PutUint32(associatedData, blockID)

	// 对数据进行加密
	return sm.encryptionProvider.Encrypt(ctx, string(sm.config.DefaultAlgorithm), keyData, data, associatedData)
}

// DecryptBlock 解密数据块
func (sm *DefaultSecurityManager) DecryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 如果加密未启用，直接返回原始数据
	if !sm.config.EncryptionEnabled {
		return data, nil
	}

	// 获取默认密钥
	keyData, err := sm.keyManager.GetKey(ctx, sm.defaultKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get default key: %w", err)
	}

	// 准备额外的关联数据（AAD）
	associatedData := make([]byte, 4)
	binary.BigEndian.PutUint32(associatedData, blockID)

	// 对数据进行解密
	return sm.encryptionProvider.Decrypt(ctx, string(sm.config.DefaultAlgorithm), keyData, data, associatedData)
}

// SetDefaultKey 设置默认密钥
func (sm *DefaultSecurityManager) SetDefaultKey(keyID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.defaultKeyID = keyID
}

// GetDefaultKey 获取默认密钥
func (sm *DefaultSecurityManager) GetDefaultKey() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.defaultKeyID
}

// EncryptWithKey 使用指定密钥加密数据
func (sm *DefaultSecurityManager) EncryptWithKey(ctx context.Context, keyID string, data []byte, options *EncryptionOptions) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 如果加密未启用，直接返回原始数据
	if !sm.config.EncryptionEnabled {
		return data, nil
	}

	// 获取密钥
	keyData, err := sm.keyManager.GetKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// 使用提供的选项或默认算法
	algorithm := string(sm.config.DefaultAlgorithm)
	if options != nil && options.Algorithm != "" {
		algorithm = string(options.Algorithm)
	}

	// 获取关联数据
	var aad []byte
	if options != nil && options.AdditionalData != nil {
		aad = options.AdditionalData
	} else if options != nil && options.BlockID != 0 {
		// 如果提供了BlockID，使用它作为关联数据
		aad = make([]byte, 4)
		binary.BigEndian.PutUint32(aad, options.BlockID)
	}

	// 对数据进行加密
	return sm.encryptionProvider.Encrypt(ctx, algorithm, keyData, data, aad)
}

// DecryptWithKey 使用指定密钥解密数据
func (sm *DefaultSecurityManager) DecryptWithKey(ctx context.Context, keyID string, data []byte, options *EncryptionOptions) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 如果加密未启用，直接返回原始数据
	if !sm.config.EncryptionEnabled {
		return data, nil
	}

	// 获取密钥
	keyData, err := sm.keyManager.GetKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// 使用提供的选项或默认算法
	algorithm := string(sm.config.DefaultAlgorithm)
	if options != nil && options.Algorithm != "" {
		algorithm = string(options.Algorithm)
	}

	// 获取关联数据
	var aad []byte
	if options != nil && options.AdditionalData != nil {
		aad = options.AdditionalData
	} else if options != nil && options.BlockID != 0 {
		// 如果提供了BlockID，使用它作为关联数据
		aad = make([]byte, 4)
		binary.BigEndian.PutUint32(aad, options.BlockID)
	}

	// 对数据进行解密
	return sm.encryptionProvider.Decrypt(ctx, algorithm, keyData, data, aad)
}
