package security

import (
	"time"
)

// EncryptionAlgorithm 加密算法类型
type EncryptionAlgorithm string

const (
	// 对称加密算法
	// AES256GCM AES-256-GCM加密
	AES256GCM EncryptionAlgorithm = "AES-256-GCM"

	// AES256CTR AES-256-CTR加密
	AES256CTR EncryptionAlgorithm = "AES-256-CTR"

	// ChaCha20Poly1305 ChaCha20-Poly1305加密
	ChaCha20Poly1305 EncryptionAlgorithm = "ChaCha20-Poly1305"

	// 非对称加密算法
	// RSA2048 RSA-2048加密
	RSA2048 EncryptionAlgorithm = "RSA-2048"

	// RSA4096 RSA-4096加密
	RSA4096 EncryptionAlgorithm = "RSA-4096"

	// ECIES256 使用secp256r1曲线的ECIES加密
	ECIES256 EncryptionAlgorithm = "ECIES-P256"

	// ECIES384 使用secp384r1曲线的ECIES加密
	ECIES384 EncryptionAlgorithm = "ECIES-P384"
)

// KeyType 密钥类型
type KeyType string

const (
	// SymmetricKey 对称密钥
	SymmetricKey KeyType = "SYMMETRIC"

	// AsymmetricKey 非对称密钥
	AsymmetricKey KeyType = "asymmetric"

	// RSAPrivateKey RSA私钥
	RSAPrivateKey KeyType = "RSA_PRIVATE"

	// RSAPublicKey RSA公钥
	RSAPublicKey KeyType = "RSA_PUBLIC"

	// ECPrivateKey 椭圆曲线私钥
	ECPrivateKey KeyType = "EC_PRIVATE"

	// ECPublicKey 椭圆曲线公钥
	ECPublicKey KeyType = "EC_PUBLIC"

	// MasterKey 主密钥
	MasterKey KeyType = "master"

	// DerivedKey 派生密钥
	DerivedKey KeyType = "derived"

	// ED25519PrivateKey Ed25519私钥
	ED25519PrivateKey KeyType = "ED25519_PRIVATE"

	// ED25519PublicKey Ed25519公钥
	ED25519PublicKey KeyType = "ED25519_PUBLIC"
)

// AlgorithmType 表示加密算法的类型
type AlgorithmType string

const (
	// SymmetricEncryption 表示对称加密算法
	SymmetricEncryption AlgorithmType = "symmetric"

	// AsymmetricEncryption 表示非对称加密算法
	AsymmetricEncryption AlgorithmType = "asymmetric"

	// HashFunction 表示哈希函数
	HashFunction AlgorithmType = "hash"

	// SignatureAlgorithm 表示签名算法
	SignatureAlgorithm AlgorithmType = "signature"
)

// AsymmetricKeyPair 表示非对称密钥对
type AsymmetricKeyPair struct {
	// PrivateKeyID 是私钥的唯一标识符
	PrivateKeyID string

	// PublicKeyID 是公钥的唯一标识符
	PublicKeyID string
}

// EncryptionOptions 加密选项
type EncryptionOptions struct {
	// Algorithm 加密算法
	Algorithm EncryptionAlgorithm

	// AdditionalData 附加验证数据(AAD)
	AdditionalData []byte

	// BlockID 块ID（用于确定性IV生成）
	BlockID uint32

	// Nonce 自定义随机数（如果不指定，将自动生成）
	Nonce []byte

	// Padding 填充模式（仅用于RSA加密）
	Padding string

	// Label RSA-OAEP加密的可选标签
	Label []byte
}

// KeyOptions 密钥选项
type KeyOptions struct {
	// 密钥类型
	Type KeyType

	// 密钥大小（比特）
	Size int

	// 密钥用途
	Usage []KeyUsage

	// 元数据
	Metadata map[string]string

	// 轮换策略
	RotationPolicy *RotationPolicy
}

// KeyUsage 密钥用途
type KeyUsage string

const (
	// EncryptionUsage 加密用途
	EncryptionUsage KeyUsage = "encryption"

	// SigningUsage 签名用途
	SigningUsage KeyUsage = "signing"

	// DeriveUsage 派生用途
	DeriveUsage KeyUsage = "derive"
)

// AlgorithmInfo 算法信息
type AlgorithmInfo struct {
	// 名称
	Name string

	// 类型
	Type string

	// 密钥大小（比特）
	KeySize int

	// 描述
	Description string
}

// RotationPolicy 密钥轮换策略
type RotationPolicy struct {
	// 轮换间隔（秒）
	IntervalSeconds int64

	// 是否自动轮换
	AutoRotate bool
}

// KeyEntry 密钥条目，用于存储在安全存储中
type KeyEntry struct {
	// 密钥数据
	Key []byte

	// 密钥元数据
	Metadata map[string]string

	// 创建时间
	CreatedAt time.Time

	// 过期时间
	ExpiresAt time.Time
}

// BlockEncryptionInfo 块加密信息
type BlockEncryptionInfo struct {
	// 算法
	Algorithm EncryptionAlgorithm

	// 密钥ID
	KeyID string

	// 随机数
	Nonce []byte

	// 附加数据
	AAD []byte
}

// 用于RSA加密的填充模式
const (
	// PKCS1Padding PKCS#1 v1.5填充
	PKCS1Padding = "PKCS1"

	// OAEPPadding 使用SHA-256的OAEP填充
	OAEPPadding = "OAEP-SHA256"
)
