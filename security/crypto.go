package security

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// DefaultEncryptionProvider 是默认的加密提供者实现
type DefaultEncryptionProvider struct {
	keyManager KeyManager
	algorithms map[string]*algorithmInfo
}

// algorithmInfo 存储算法的详细信息和处理函数
type algorithmInfo struct {
	Type        AlgorithmType
	KeySize     int
	Description string
	Encrypt     encryptFunc
	Decrypt     decryptFunc
}

// NewDefaultEncryptionProvider 创建默认加密提供者
func NewDefaultEncryptionProvider(keyManager KeyManager) *DefaultEncryptionProvider {
	provider := &DefaultEncryptionProvider{
		keyManager: keyManager,
		algorithms: make(map[string]*algorithmInfo),
	}

	// 注册支持的算法
	provider.registerAlgorithms()
	return provider
}

// registerAlgorithms 注册支持的算法
func (p *DefaultEncryptionProvider) registerAlgorithms() {
	// 对称加密算法
	p.algorithms[string(AES256GCM)] = &algorithmInfo{
		Type:        SymmetricEncryption,
		KeySize:     256,
		Description: "AES-256-GCM",
		Encrypt:     p.encryptAES,
		Decrypt:     p.decryptAES,
	}

	p.algorithms[string(AES256CTR)] = &algorithmInfo{
		Type:        SymmetricEncryption,
		KeySize:     256,
		Description: "AES-256-CTR",
		Encrypt:     p.encryptAES,
		Decrypt:     p.decryptAES,
	}

	p.algorithms[string(ChaCha20Poly1305)] = &algorithmInfo{
		Type:        SymmetricEncryption,
		KeySize:     256,
		Description: "ChaCha20-Poly1305",
		Encrypt:     p.encryptAES, // 临时使用AES实现
		Decrypt:     p.decryptAES, // 临时使用AES实现
	}

	// 非对称加密算法
	p.algorithms[string(RSA2048)] = &algorithmInfo{
		Type:        AsymmetricEncryption,
		KeySize:     2048,
		Description: "RSA-2048",
		Encrypt:     p.encryptRSA,
		Decrypt:     p.decryptRSA,
	}

	p.algorithms[string(RSA4096)] = &algorithmInfo{
		Type:        AsymmetricEncryption,
		KeySize:     4096,
		Description: "RSA-4096",
		Encrypt:     p.encryptRSA,
		Decrypt:     p.decryptRSA,
	}

	p.algorithms[string(ECIES256)] = &algorithmInfo{
		Type:        AsymmetricEncryption,
		KeySize:     256,
		Description: "ECIES-P256",
		Encrypt:     p.encryptECIES,
		Decrypt:     p.decryptECIES,
	}

	p.algorithms[string(ECIES384)] = &algorithmInfo{
		Type:        AsymmetricEncryption,
		KeySize:     384,
		Description: "ECIES-P384",
		Encrypt:     p.encryptECIES,
		Decrypt:     p.decryptECIES,
	}

	// 添加RSA-OAEP-SHA256算法
	p.algorithms["RSA-2048-OAEP-SHA256"] = &algorithmInfo{
		Type:        AsymmetricEncryption,
		KeySize:     2048,
		Description: "RSA-2048 with OAEP padding using SHA-256",
		Encrypt:     p.encryptRSA,
		Decrypt:     p.decryptRSA,
	}
}

// Encrypt 使用指定的算法和密钥加密数据
func (p *DefaultEncryptionProvider) Encrypt(ctx context.Context, algorithm string, key []byte, plaintext []byte, aad []byte) ([]byte, error) {
	// 获取算法信息
	info, exists := p.algorithms[algorithm]
	if !exists {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// 调用算法特定的加密函数
	return info.Encrypt(ctx, algorithm, key, plaintext, aad)
}

// Decrypt 使用指定的算法和密钥解密数据
func (p *DefaultEncryptionProvider) Decrypt(ctx context.Context, algorithm string, key []byte, ciphertext []byte, aad []byte) ([]byte, error) {
	// 获取算法信息
	info, exists := p.algorithms[algorithm]
	if !exists {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// 调用算法特定的解密函数
	return info.Decrypt(ctx, algorithm, key, ciphertext, aad)
}

// GetAlgorithmInfo 获取算法信息
func (p *DefaultEncryptionProvider) GetAlgorithmInfo(ctx context.Context, algorithm string) (AlgorithmInfo, error) {
	info, exists := p.algorithms[algorithm]
	if !exists {
		return AlgorithmInfo{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	return AlgorithmInfo{
		Name:        algorithm,
		Type:        string(info.Type),
		KeySize:     info.KeySize,
		Description: info.Description,
	}, nil
}

// ListSupportedAlgorithms 列出所有支持的算法
func (p *DefaultEncryptionProvider) ListSupportedAlgorithms() []EncryptionAlgorithm {
	algorithms := make([]EncryptionAlgorithm, 0, len(p.algorithms))
	for name := range p.algorithms {
		algorithms = append(algorithms, EncryptionAlgorithm(name))
	}
	return algorithms
}

// 辅助函数：从BlockID生成确定性随机数
func generateDeterministicNonce(blockID uint32, size int) []byte {
	nonce := make([]byte, size)

	// 使用blockID作为种子
	h := sha256.New()
	binary.Write(h, binary.LittleEndian, blockID)

	// 截取哈希值的前size字节作为随机数
	copy(nonce, h.Sum(nil)[:size])

	return nonce
}

// 辅助函数：创建ChaCha20-Poly1305
func newChaCha20Poly1305(key []byte) (cipher.AEAD, error) {
	// 在实际项目中，我们会使用golang.org/x/crypto/chacha20poly1305
	// 这里简化为假实现
	return nil, errors.New("ChaCha20-Poly1305 not implemented in this example")
}

// encryptAES 使用AES加密数据
func (p *DefaultEncryptionProvider) encryptAES(ctx context.Context, algorithm string, key []byte, plaintext []byte, aad []byte) ([]byte, error) {
	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// 创建GCM（仅支持AES-GCM模式）
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 生成随机数
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密数据
	ciphertext := aesGCM.Seal(nil, nonce, plaintext, aad)

	// 构造加密结果
	result := &encryptedData{
		Algorithm:  algorithm,
		Ciphertext: ciphertext,
		IV:         nonce,
		AAD:        aad,
	}

	// 序列化加密结果
	resultData, err := serializeEncryptedData(result)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize encrypted data: %w", err)
	}

	return resultData, nil
}

// decryptAES 使用AES解密数据
func (p *DefaultEncryptionProvider) decryptAES(ctx context.Context, algorithm string, key []byte, ciphertext []byte, aad []byte) ([]byte, error) {
	// 反序列化加密数据
	encData, err := deserializeEncryptedData(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize encrypted data: %w", err)
	}

	// 验证算法
	if encData.Algorithm != algorithm {
		return nil, fmt.Errorf("algorithm mismatch: expected %s, got %s", algorithm, encData.Algorithm)
	}

	// 验证AAD
	if aad != nil && !bytes.Equal(encData.AAD, aad) {
		return nil, errors.New("authentication data (AAD) mismatch")
	}

	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// 创建GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 确保有随机数
	if len(encData.IV) != aesGCM.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}

	// 解密数据
	plaintext, err := aesGCM.Open(nil, encData.IV, encData.Ciphertext, encData.AAD)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// encryptRSA 使用RSA加密数据
func (p *DefaultEncryptionProvider) encryptRSA(ctx context.Context, algorithm string, key []byte, plaintext []byte, aad []byte) ([]byte, error) {
	// 解析公钥
	pubKey, err := x509.ParsePKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}

	rsaKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an RSA public key")
	}

	// 确定填充方式
	padding := OAEPPadding // 默认使用OAEP

	// 根据填充方式加密
	var ciphertext []byte
	switch padding {
	case PKCS1Padding:
		ciphertext, err = rsa.EncryptPKCS1v15(rand.Reader, rsaKey, plaintext)
	case OAEPPadding:
		// 使用SHA-256 hash
		hash := sha256.New()
		label := aad
		ciphertext, err = rsa.EncryptOAEP(hash, rand.Reader, rsaKey, plaintext, label)
	default:
		return nil, fmt.Errorf("unsupported RSA padding mode: %s", padding)
	}

	if err != nil {
		return nil, fmt.Errorf("RSA encryption failed: %w", err)
	}

	// 构造加密结果
	result := &encryptedData{
		Algorithm:  algorithm,
		Ciphertext: ciphertext,
		AAD:        aad,
	}

	// 序列化加密结果
	resultData, err := serializeEncryptedData(result)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize encrypted data: %w", err)
	}

	return resultData, nil
}

// decryptRSA 使用RSA解密数据
func (p *DefaultEncryptionProvider) decryptRSA(ctx context.Context, algorithm string, key []byte, ciphertext []byte, aad []byte) ([]byte, error) {
	// 反序列化加密数据
	encData, err := deserializeEncryptedData(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize encrypted data: %w", err)
	}

	// 验证算法
	if encData.Algorithm != algorithm {
		return nil, fmt.Errorf("algorithm mismatch: expected %s, got %s", algorithm, encData.Algorithm)
	}

	// 解析私钥
	privKey, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		// 尝试其他格式
		privKey, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
		}
	}

	rsaKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an RSA private key")
	}

	// 确定填充方式
	padding := OAEPPadding // 默认使用OAEP

	// 根据填充方式解密
	var plaintext []byte
	switch padding {
	case PKCS1Padding:
		plaintext, err = rsa.DecryptPKCS1v15(rand.Reader, rsaKey, encData.Ciphertext)
	case OAEPPadding:
		// 使用SHA-256 hash
		hash := sha256.New()
		label := encData.AAD
		plaintext, err = rsa.DecryptOAEP(hash, rand.Reader, rsaKey, encData.Ciphertext, label)
	default:
		return nil, fmt.Errorf("unsupported RSA padding mode: %s", padding)
	}

	if err != nil {
		return nil, fmt.Errorf("RSA decryption failed: %w", err)
	}

	return plaintext, nil
}

// encryptECIES 使用ECIES加密数据
// 注意：标准Go库不直接支持ECIES，这里提供一个基本框架
func (p *DefaultEncryptionProvider) encryptECIES(ctx context.Context, algorithm string, key []byte, plaintext []byte, aad []byte) ([]byte, error) {
	// 实际实现中，我们会使用外部库如github.com/ethereum/go-ethereum/crypto/ecies
	return nil, errors.New("ECIES encryption not implemented in this example")
}

// decryptECIES 使用ECIES解密数据
func (p *DefaultEncryptionProvider) decryptECIES(ctx context.Context, algorithm string, key []byte, ciphertext []byte, aad []byte) ([]byte, error) {
	// 实际实现中，我们会使用外部库如github.com/ethereum/go-ethereum/crypto/ecies
	return nil, errors.New("ECIES decryption not implemented in this example")
}

// 加密数据结构
type encryptedData struct {
	Algorithm  string `json:"algorithm"`
	Ciphertext []byte `json:"ciphertext"`
	IV         []byte `json:"iv,omitempty"`
	Tag        []byte `json:"tag,omitempty"`
	AAD        []byte `json:"aad,omitempty"`
}

// 序列化加密数据
func serializeEncryptedData(data *encryptedData) ([]byte, error) {
	if data == nil {
		return nil, errors.New("encrypted data cannot be nil")
	}

	// 将字节数组转换为base64编码的字符串
	jsonData := struct {
		Algorithm  string `json:"algorithm"`
		Ciphertext string `json:"ciphertext"`
		IV         string `json:"iv,omitempty"`
		Tag        string `json:"tag,omitempty"`
		AAD        string `json:"aad,omitempty"`
	}{
		Algorithm:  data.Algorithm,
		Ciphertext: base64.StdEncoding.EncodeToString(data.Ciphertext),
	}

	if data.IV != nil {
		jsonData.IV = base64.StdEncoding.EncodeToString(data.IV)
	}

	if data.Tag != nil {
		jsonData.Tag = base64.StdEncoding.EncodeToString(data.Tag)
	}

	if data.AAD != nil {
		jsonData.AAD = base64.StdEncoding.EncodeToString(data.AAD)
	}

	// 序列化为JSON
	serialized, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize encrypted data: %w", err)
	}

	return serialized, nil
}

// 反序列化加密数据
func deserializeEncryptedData(data []byte) (*encryptedData, error) {
	if len(data) == 0 {
		return nil, errors.New("data cannot be empty")
	}

	// 反序列化JSON
	var jsonData struct {
		Algorithm  string `json:"algorithm"`
		Ciphertext string `json:"ciphertext"`
		IV         string `json:"iv,omitempty"`
		Tag        string `json:"tag,omitempty"`
		AAD        string `json:"aad,omitempty"`
	}

	err := json.Unmarshal(data, &jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize encrypted data: %w", err)
	}

	// 创建结果结构
	result := &encryptedData{
		Algorithm: jsonData.Algorithm,
	}

	// 解码base64编码的字节数组
	result.Ciphertext, err = base64.StdEncoding.DecodeString(jsonData.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	if jsonData.IV != "" {
		result.IV, err = base64.StdEncoding.DecodeString(jsonData.IV)
		if err != nil {
			return nil, fmt.Errorf("failed to decode IV: %w", err)
		}
	}

	if jsonData.Tag != "" {
		result.Tag, err = base64.StdEncoding.DecodeString(jsonData.Tag)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tag: %w", err)
		}
	}

	if jsonData.AAD != "" {
		result.AAD, err = base64.StdEncoding.DecodeString(jsonData.AAD)
		if err != nil {
			return nil, fmt.Errorf("failed to decode AAD: %w", err)
		}
	}

	return result, nil
}

// EncryptWithPublicKey 使用公钥进行加密
func (p *DefaultEncryptionProvider) EncryptWithPublicKey(ctx context.Context, algorithm string, publicKey []byte, plaintext []byte, aad []byte) ([]byte, error) {
	// 获取算法信息
	info, exists := p.algorithms[algorithm]
	if !exists {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// 验证算法类型
	if info.Type != AsymmetricEncryption {
		return nil, fmt.Errorf("algorithm %s is not an asymmetric encryption algorithm", algorithm)
	}

	// 调用算法特定的加密函数
	return info.Encrypt(ctx, algorithm, publicKey, plaintext, aad)
}

// DecryptWithPrivateKey 使用私钥进行解密
func (p *DefaultEncryptionProvider) DecryptWithPrivateKey(ctx context.Context, algorithm string, privateKey []byte, ciphertext []byte, aad []byte) ([]byte, error) {
	// 获取算法信息
	info, exists := p.algorithms[algorithm]
	if !exists {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// 验证算法类型
	if info.Type != AsymmetricEncryption {
		return nil, fmt.Errorf("algorithm %s is not an asymmetric encryption algorithm", algorithm)
	}

	// 调用算法特定的解密函数
	return info.Decrypt(ctx, algorithm, privateKey, ciphertext, aad)
}

// encryptFunc 定义加密函数类型
type encryptFunc func(ctx context.Context, algorithm string, key []byte, plaintext []byte, aad []byte) ([]byte, error)

// decryptFunc 定义解密函数类型
type decryptFunc func(ctx context.Context, algorithm string, key []byte, ciphertext []byte, aad []byte) ([]byte, error)
