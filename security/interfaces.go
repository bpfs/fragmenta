package security

import (
	"context"
)

// EncryptionProvider 定义了加密和解密功能的接口
type EncryptionProvider interface {
	// Encrypt 使用指定的算法和密钥加密数据
	Encrypt(ctx context.Context, algorithm string, key []byte, plaintext []byte, aad []byte) ([]byte, error)

	// Decrypt 使用指定的算法和密钥解密数据
	Decrypt(ctx context.Context, algorithm string, key []byte, ciphertext []byte, aad []byte) ([]byte, error)

	// GetAlgorithmInfo 返回指定加密算法的详细信息
	GetAlgorithmInfo(ctx context.Context, algorithm string) (AlgorithmInfo, error)

	// ListSupportedAlgorithms 列出所有支持的算法
	ListSupportedAlgorithms() []EncryptionAlgorithm

	// EncryptWithPublicKey 使用公钥加密数据（非对称加密）
	EncryptWithPublicKey(ctx context.Context, algorithm string, publicKey []byte, plaintext []byte, aad []byte) ([]byte, error)

	// DecryptWithPrivateKey 使用私钥解密数据（非对称加密）
	DecryptWithPrivateKey(ctx context.Context, algorithm string, privateKey []byte, ciphertext []byte, aad []byte) ([]byte, error)
}

// KeyManager 管理加密密钥的接口
type KeyManager interface {
	// GenerateKey 生成新密钥
	GenerateKey(ctx context.Context, keyType KeyType, options *KeyOptions) (string, error)

	// GetKey 获取密钥（可能从安全存储中获取）
	GetKey(ctx context.Context, keyID string) ([]byte, error)

	// DeleteKey 删除密钥
	DeleteKey(ctx context.Context, keyID string) error

	// RotateKey 轮换密钥
	RotateKey(ctx context.Context, oldKeyID string, options *KeyOptions) (string, error)

	// ListKeys 列出所有密钥ID
	ListKeys(ctx context.Context) ([]string, error)

	// ImportKey 导入现有密钥
	ImportKey(ctx context.Context, keyData []byte, options *KeyOptions) (string, error)

	// ExportKey 导出密钥（如果策略允许）
	ExportKey(ctx context.Context, keyID string) ([]byte, error)

	// KeyExists 检查密钥是否存在
	KeyExists(ctx context.Context, keyID string) (bool, error)

	// GenerateKeyPair 生成非对称密钥对
	GenerateKeyPair(ctx context.Context, keyType KeyType, options *KeyOptions) (*AsymmetricKeyPair, error)

	// GetPublicKey 从私钥ID获取对应的公钥ID
	GetPublicKey(ctx context.Context, privateKeyID string) (string, error)

	// ImportKeyPair 导入非对称密钥对
	ImportKeyPair(ctx context.Context, privateKeyData, publicKeyData []byte, options *KeyOptions) (*AsymmetricKeyPair, error)
}

// SecureStorage 安全存储接口，用于存储敏感数据（如密钥）
type SecureStorage interface {
	// Store 存储数据
	Store(ctx context.Context, key string, data []byte) error

	// Retrieve 获取数据
	Retrieve(ctx context.Context, key string) ([]byte, error)

	// Delete 删除数据
	Delete(ctx context.Context, key string) error

	// List 列出存储的所有键
	List(ctx context.Context) ([]string, error)
}

// SecurityManager 整合安全功能的管理器接口
type SecurityManager interface {
	// 获取加密提供者
	GetEncryptionProvider() EncryptionProvider

	// 获取密钥管理器
	GetKeyManager() KeyManager

	// 初始化安全子系统
	Initialize(ctx context.Context) error

	// 关闭安全子系统
	Shutdown(ctx context.Context) error

	// 加密数据块
	EncryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error)

	// 解密数据块
	DecryptBlock(ctx context.Context, blockID uint32, data []byte) ([]byte, error)

	// IsInitialized 检查是否已初始化
	IsInitialized() bool

	// EncryptWithKey 使用指定密钥加密数据
	EncryptWithKey(ctx context.Context, keyID string, data []byte, options *EncryptionOptions) ([]byte, error)

	// DecryptWithKey 使用指定密钥解密数据
	DecryptWithKey(ctx context.Context, keyID string, data []byte, options *EncryptionOptions) ([]byte, error)
}
