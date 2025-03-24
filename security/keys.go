package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// DefaultKeyManager 默认密钥管理器实现
type DefaultKeyManager struct {
	storage SecureStorage
}

// NewDefaultKeyManager 创建默认密钥管理器
func NewDefaultKeyManager(storage SecureStorage) *DefaultKeyManager {
	return &DefaultKeyManager{
		storage: storage,
	}
}

// GenerateKey 生成新密钥
func (km *DefaultKeyManager) GenerateKey(ctx context.Context, keyType KeyType, options *KeyOptions) (string, error) {
	if options == nil {
		options = &KeyOptions{
			Type: keyType,
			Size: 256, // 默认256位
		}
	}

	// 确定密钥大小
	keySize := options.Size / 8 // 转换为字节
	if keySize <= 0 {
		return "", errors.New("invalid key size")
	}

	// 生成随机密钥
	key := make([]byte, keySize)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}

	// 生成密钥ID
	timestamp := time.Now().UnixNano()
	keyID := fmt.Sprintf("%s-%d-%s", keyType, timestamp, generateRandomString(8))

	// 准备密钥元数据
	metadata := map[string]string{
		"type":      string(keyType),
		"size":      fmt.Sprintf("%d", options.Size),
		"timestamp": fmt.Sprintf("%d", timestamp),
	}

	// 将自定义元数据添加到密钥元数据
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			metadata[k] = v
		}
	}

	// 创建密钥条目
	keyEntry := &KeyEntry{
		Key:       key,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	// 如果有设置轮换策略，添加过期时间
	if options.RotationPolicy != nil && options.RotationPolicy.IntervalSeconds > 0 {
		keyEntry.ExpiresAt = keyEntry.CreatedAt.Add(time.Duration(options.RotationPolicy.IntervalSeconds) * time.Second)
	}

	// 序列化密钥条目
	keyData, err := serializeKeyEntry(keyEntry)
	if err != nil {
		return "", err
	}

	// 存储密钥
	err = km.storage.Store(ctx, keyID, keyData)
	if err != nil {
		return "", err
	}

	return keyID, nil
}

// GetKey 获取密钥
func (km *DefaultKeyManager) GetKey(ctx context.Context, keyID string) ([]byte, error) {
	if keyID == "" {
		return nil, errors.New("keyID cannot be empty")
	}

	// 从存储中检索密钥数据
	keyData, err := km.storage.Retrieve(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key: %w", err)
	}

	// 反序列化密钥条目
	keyEntry, err := deserializeKeyEntry(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize key entry: %w", err)
	}

	// 检查密钥是否过期
	if !keyEntry.ExpiresAt.IsZero() && time.Now().After(keyEntry.ExpiresAt) {
		return nil, errors.New("key has expired")
	}

	return keyEntry.Key, nil
}

// DeleteKey 删除密钥
func (km *DefaultKeyManager) DeleteKey(ctx context.Context, keyID string) error {
	if keyID == "" {
		return errors.New("keyID cannot be empty")
	}

	return km.storage.Delete(ctx, keyID)
}

// RotateKey 轮换密钥
func (km *DefaultKeyManager) RotateKey(ctx context.Context, oldKeyID string, options *KeyOptions) (string, error) {
	if oldKeyID == "" {
		return "", errors.New("oldKeyID cannot be empty")
	}

	// 获取旧密钥数据
	oldKeyData, err := km.storage.Retrieve(ctx, oldKeyID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve old key: %w", err)
	}

	// 反序列化旧密钥条目
	oldKeyEntry, err := deserializeKeyEntry(oldKeyData)
	if err != nil {
		return "", fmt.Errorf("failed to deserialize old key entry: %w", err)
	}

	// 如果没有提供选项，使用旧密钥的元数据
	if options == nil {
		keyType := KeyType(oldKeyEntry.Metadata["type"])
		size := 256 // 默认值
		if sizeStr, ok := oldKeyEntry.Metadata["size"]; ok {
			fmt.Sscanf(sizeStr, "%d", &size)
		}

		options = &KeyOptions{
			Type:     keyType,
			Size:     size,
			Metadata: oldKeyEntry.Metadata,
		}
	}

	// 生成新密钥
	newKeyID, err := km.GenerateKey(ctx, options.Type, options)
	if err != nil {
		return "", fmt.Errorf("failed to generate new key: %w", err)
	}

	return newKeyID, nil
}

// ListKeys 列出所有密钥ID
func (km *DefaultKeyManager) ListKeys(ctx context.Context) ([]string, error) {
	return km.storage.List(ctx)
}

// ImportKey 导入现有密钥
func (km *DefaultKeyManager) ImportKey(ctx context.Context, keyData []byte, options *KeyOptions) (string, error) {
	if len(keyData) == 0 {
		return "", errors.New("keyData cannot be empty")
	}

	if options == nil {
		return "", errors.New("options cannot be nil for importing keys")
	}

	// 生成密钥ID
	timestamp := time.Now().UnixNano()
	keyID := fmt.Sprintf("%s-%d-%s", options.Type, timestamp, generateRandomString(8))

	// 准备密钥元数据
	metadata := map[string]string{
		"type":      string(options.Type),
		"size":      fmt.Sprintf("%d", len(keyData)*8),
		"timestamp": fmt.Sprintf("%d", timestamp),
		"imported":  "true",
	}

	// 将自定义元数据添加到密钥元数据
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			metadata[k] = v
		}
	}

	// 创建密钥条目
	keyEntry := &KeyEntry{
		Key:       keyData,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	// 如果有设置轮换策略，添加过期时间
	if options.RotationPolicy != nil && options.RotationPolicy.IntervalSeconds > 0 {
		keyEntry.ExpiresAt = keyEntry.CreatedAt.Add(time.Duration(options.RotationPolicy.IntervalSeconds) * time.Second)
	}

	// 序列化密钥条目
	serializedKeyEntry, err := serializeKeyEntry(keyEntry)
	if err != nil {
		return "", err
	}

	// 存储密钥
	err = km.storage.Store(ctx, keyID, serializedKeyEntry)
	if err != nil {
		return "", err
	}

	return keyID, nil
}

// ExportKey 导出密钥
func (km *DefaultKeyManager) ExportKey(ctx context.Context, keyID string) ([]byte, error) {
	if keyID == "" {
		return nil, errors.New("keyID cannot be empty")
	}

	// 简单实现，直接返回密钥
	// 注意：在实际应用中，应该有额外的安全检查和策略控制
	return km.GetKey(ctx, keyID)
}

// KeyExists 检查密钥是否存在
func (km *DefaultKeyManager) KeyExists(ctx context.Context, keyID string) (bool, error) {
	if keyID == "" {
		return false, errors.New("keyID cannot be empty")
	}

	// 获取所有密钥ID
	keys, err := km.ListKeys(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to list keys: %w", err)
	}

	// 检查是否包含指定的密钥ID
	for _, id := range keys {
		if id == keyID {
			return true, nil
		}
	}

	return false, nil
}

// GenerateKeyPair 生成非对称密钥对
func (km *DefaultKeyManager) GenerateKeyPair(ctx context.Context, keyType KeyType, options *KeyOptions) (*AsymmetricKeyPair, error) {
	if options == nil {
		options = &KeyOptions{
			Type: keyType,
			Size: 2048, // 默认RSA 2048位
		}
	}

	var privateKeyBytes, publicKeyBytes []byte
	var err error

	switch keyType {
	case RSAPrivateKey:
		// 生成RSA密钥对
		privateKeyBytes, publicKeyBytes, err = generateRSAKeyPair(options.Size)
	case ECPrivateKey:
		// 生成EC密钥对
		privateKeyBytes, publicKeyBytes, err = generateECKeyPair(options.Size)
	default:
		return nil, fmt.Errorf("unsupported key type for key pair generation: %s", keyType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// 生成密钥ID的基础
	timestamp := time.Now().UnixNano()
	randomStr := generateRandomString(8)

	// 创建私钥元数据
	privateKeyMetadata := map[string]string{
		"type":      string(keyType),
		"size":      fmt.Sprintf("%d", options.Size),
		"timestamp": fmt.Sprintf("%d", timestamp),
		"has_pair":  "true",
	}

	// 复制用户提供的元数据
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			privateKeyMetadata[k] = v
		}
	}

	// 创建公钥元数据
	var publicKeyType KeyType
	if keyType == RSAPrivateKey {
		publicKeyType = RSAPublicKey
	} else {
		publicKeyType = ECPublicKey
	}

	publicKeyMetadata := map[string]string{
		"type":      string(publicKeyType),
		"size":      fmt.Sprintf("%d", options.Size),
		"timestamp": fmt.Sprintf("%d", timestamp),
		"has_pair":  "true",
	}

	// 复制用户提供的元数据到公钥
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			publicKeyMetadata[k] = v
		}
	}

	// 生成密钥ID
	privateKeyID := fmt.Sprintf("%s-%d-%s", keyType, timestamp, randomStr)
	publicKeyID := fmt.Sprintf("%s-%d-%s", publicKeyType, timestamp, randomStr)

	// 在元数据中记录对应的公钥/私钥ID
	privateKeyMetadata["public_key_id"] = publicKeyID
	publicKeyMetadata["private_key_id"] = privateKeyID

	// 创建密钥条目
	privateKeyEntry := &KeyEntry{
		Key:       privateKeyBytes,
		Metadata:  privateKeyMetadata,
		CreatedAt: time.Now(),
	}

	publicKeyEntry := &KeyEntry{
		Key:       publicKeyBytes,
		Metadata:  publicKeyMetadata,
		CreatedAt: time.Now(),
	}

	// 如果有设置轮换策略，添加过期时间
	if options.RotationPolicy != nil && options.RotationPolicy.IntervalSeconds > 0 {
		expireTime := privateKeyEntry.CreatedAt.Add(time.Duration(options.RotationPolicy.IntervalSeconds) * time.Second)
		privateKeyEntry.ExpiresAt = expireTime
		publicKeyEntry.ExpiresAt = expireTime
	}

	// 序列化密钥条目
	serializedPrivateKey, err := serializeKeyEntry(privateKeyEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize private key: %w", err)
	}

	serializedPublicKey, err := serializeKeyEntry(publicKeyEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize public key: %w", err)
	}

	// 存储私钥
	err = km.storage.Store(ctx, privateKeyID, serializedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to store private key: %w", err)
	}

	// 存储公钥
	err = km.storage.Store(ctx, publicKeyID, serializedPublicKey)
	if err != nil {
		// 如果存储公钥失败，尝试删除已存储的私钥
		km.storage.Delete(ctx, privateKeyID)
		return nil, fmt.Errorf("failed to store public key: %w", err)
	}

	return &AsymmetricKeyPair{
		PrivateKeyID: privateKeyID,
		PublicKeyID:  publicKeyID,
	}, nil
}

// GetPublicKey 从私钥ID获取对应的公钥ID
func (km *DefaultKeyManager) GetPublicKey(ctx context.Context, privateKeyID string) (string, error) {
	if privateKeyID == "" {
		return "", errors.New("private key ID cannot be empty")
	}

	// 获取私钥数据
	keyData, err := km.storage.Retrieve(ctx, privateKeyID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve private key: %w", err)
	}

	// 反序列化私钥条目
	keyEntry, err := deserializeKeyEntry(keyData)
	if err != nil {
		return "", fmt.Errorf("failed to deserialize key entry: %w", err)
	}

	// 从元数据获取公钥ID
	publicKeyID, ok := keyEntry.Metadata["public_key_id"]
	if !ok {
		return "", errors.New("no public key associated with this private key")
	}

	return publicKeyID, nil
}

// ImportKeyPair 导入非对称密钥对
func (km *DefaultKeyManager) ImportKeyPair(ctx context.Context, privateKeyData, publicKeyData []byte, options *KeyOptions) (*AsymmetricKeyPair, error) {
	if options == nil {
		return nil, errors.New("options cannot be nil for importing key pair")
	}

	if len(privateKeyData) == 0 || len(publicKeyData) == 0 {
		return nil, errors.New("private key and public key data cannot be empty")
	}

	// 验证密钥对类型
	var privateKeyType, publicKeyType KeyType
	switch options.Type {
	case RSAPrivateKey:
		privateKeyType = RSAPrivateKey
		publicKeyType = RSAPublicKey
	case ECPrivateKey:
		privateKeyType = ECPrivateKey
		publicKeyType = ECPublicKey
	default:
		return nil, fmt.Errorf("unsupported key type for key pair import: %s", options.Type)
	}

	// 验证密钥数据格式
	keySize, err := validateKeyPair(privateKeyData, publicKeyData, privateKeyType)
	if err != nil {
		return nil, fmt.Errorf("invalid key pair: %w", err)
	}

	// 如果用户没有指定密钥大小，使用验证时获得的大小
	if options.Size == 0 {
		options.Size = keySize
	}

	// 生成密钥ID的基础
	timestamp := time.Now().UnixNano()
	randomStr := generateRandomString(8)

	// 创建私钥元数据
	privateKeyMetadata := map[string]string{
		"type":      string(privateKeyType),
		"size":      fmt.Sprintf("%d", options.Size),
		"timestamp": fmt.Sprintf("%d", timestamp),
		"has_pair":  "true",
		"imported":  "true",
	}

	// 复制用户提供的元数据
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			privateKeyMetadata[k] = v
		}
	}

	// 创建公钥元数据
	publicKeyMetadata := map[string]string{
		"type":      string(publicKeyType),
		"size":      fmt.Sprintf("%d", options.Size),
		"timestamp": fmt.Sprintf("%d", timestamp),
		"has_pair":  "true",
		"imported":  "true",
	}

	// 复制用户提供的元数据到公钥
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			publicKeyMetadata[k] = v
		}
	}

	// 生成密钥ID
	privateKeyID := fmt.Sprintf("%s-%d-%s", privateKeyType, timestamp, randomStr)
	publicKeyID := fmt.Sprintf("%s-%d-%s", publicKeyType, timestamp, randomStr)

	// 在元数据中记录对应的公钥/私钥ID
	privateKeyMetadata["public_key_id"] = publicKeyID
	publicKeyMetadata["private_key_id"] = privateKeyID

	// 创建密钥条目
	privateKeyEntry := &KeyEntry{
		Key:       privateKeyData,
		Metadata:  privateKeyMetadata,
		CreatedAt: time.Now(),
	}

	publicKeyEntry := &KeyEntry{
		Key:       publicKeyData,
		Metadata:  publicKeyMetadata,
		CreatedAt: time.Now(),
	}

	// 如果有设置轮换策略，添加过期时间
	if options.RotationPolicy != nil && options.RotationPolicy.IntervalSeconds > 0 {
		expireTime := privateKeyEntry.CreatedAt.Add(time.Duration(options.RotationPolicy.IntervalSeconds) * time.Second)
		privateKeyEntry.ExpiresAt = expireTime
		publicKeyEntry.ExpiresAt = expireTime
	}

	// 序列化密钥条目
	serializedPrivateKey, err := serializeKeyEntry(privateKeyEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize private key: %w", err)
	}

	serializedPublicKey, err := serializeKeyEntry(publicKeyEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize public key: %w", err)
	}

	// 存储私钥
	err = km.storage.Store(ctx, privateKeyID, serializedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to store private key: %w", err)
	}

	// 存储公钥
	err = km.storage.Store(ctx, publicKeyID, serializedPublicKey)
	if err != nil {
		// 如果存储公钥失败，尝试删除已存储的私钥
		km.storage.Delete(ctx, privateKeyID)
		return nil, fmt.Errorf("failed to store public key: %w", err)
	}

	return &AsymmetricKeyPair{
		PrivateKeyID: privateKeyID,
		PublicKeyID:  publicKeyID,
	}, nil
}

// RetrieveKeyEntry 获取完整的密钥条目（包括元数据）
func (km *DefaultKeyManager) RetrieveKeyEntry(ctx context.Context, keyID string) (*KeyEntry, error) {
	if keyID == "" {
		return nil, errors.New("key ID cannot be empty")
	}

	// 从存储中获取序列化的密钥数据
	serializedData, err := km.storage.Retrieve(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key: %w", err)
	}

	// 反序列化密钥条目
	keyEntry, err := deserializeKeyEntry(serializedData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize key entry: %w", err)
	}

	// 检查密钥是否已过期
	if keyEntry.ExpiresAt.After(time.Time{}) && time.Now().After(keyEntry.ExpiresAt) {
		// 标记过期，但仍然返回
		keyEntry.Metadata["expired"] = "true"
	}

	return keyEntry, nil
}

// 辅助函数：生成随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length/2+1)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// 辅助函数：序列化密钥条目
func serializeKeyEntry(entry *KeyEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// 辅助函数：反序列化密钥条目
func deserializeKeyEntry(data []byte) (*KeyEntry, error) {
	var entry KeyEntry
	err := json.Unmarshal(data, &entry)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// 辅助函数：生成RSA密钥对
func generateRSAKeyPair(bits int) ([]byte, []byte, error) {
	// 验证密钥大小
	if bits < 2048 {
		return nil, nil, errors.New("RSA key size must be at least 2048 bits")
	}

	// 生成RSA私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// 编码私钥为PKCS8格式
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// 编码公钥为PKIX格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	return privateKeyBytes, publicKeyBytes, nil
}

// 辅助函数：生成EC密钥对
func generateECKeyPair(bits int) ([]byte, []byte, error) {
	// 注意：标准库中没有直接支持ECIES，这里我们只提供函数框架
	return nil, nil, errors.New("EC key pair generation not implemented in this example")
}

// 辅助函数：验证密钥对
func validateKeyPair(privateKeyData, publicKeyData []byte, keyType KeyType) (int, error) {
	var keySize int

	switch keyType {
	case RSAPrivateKey:
		// 解析RSA私钥
		privateKey, err := x509.ParsePKCS8PrivateKey(privateKeyData)
		if err != nil {
			// 尝试其他格式
			privateKey, err = x509.ParsePKCS1PrivateKey(privateKeyData)
			if err != nil {
				return 0, fmt.Errorf("invalid RSA private key: %w", err)
			}
		}

		rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			return 0, errors.New("private key is not an RSA key")
		}

		// 解析RSA公钥
		publicKey, err := x509.ParsePKIXPublicKey(publicKeyData)
		if err != nil {
			return 0, fmt.Errorf("invalid RSA public key: %w", err)
		}

		rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return 0, errors.New("public key is not an RSA key")
		}

		// 验证密钥对是否匹配
		if rsaPrivateKey.N.Cmp(rsaPublicKey.N) != 0 || rsaPrivateKey.E != rsaPublicKey.E {
			return 0, errors.New("RSA public key does not match private key")
		}

		keySize = rsaPrivateKey.Size() * 8

	case ECPrivateKey:
		// 标准库中没有直接支持ECIES，这里需要实现EC密钥对验证
		return 0, errors.New("EC key pair validation not implemented in this example")

	default:
		return 0, fmt.Errorf("unsupported key type: %s", keyType)
	}

	return keySize, nil
}
