package security

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math/big"
)

// SignatureProvider 提供数字签名功能的接口
type SignatureProvider interface {
	// Sign 使用指定的算法和密钥对数据进行签名
	Sign(ctx context.Context, algorithm string, privateKey []byte, data []byte) ([]byte, error)

	// Verify 验证数据签名
	Verify(ctx context.Context, algorithm string, publicKey []byte, data []byte, signature []byte) (bool, error)

	// GetAlgorithmInfo 获取签名算法的信息
	GetAlgorithmInfo(ctx context.Context, algorithm string) (SignatureAlgorithmInfo, error)

	// ListSupportedAlgorithms 列出所有支持的签名算法
	ListSupportedAlgorithms() []SignatureAlgorithmName
}

// SignatureAlgorithmName 签名算法名称
type SignatureAlgorithmName string

const (
	// 基于HMAC的签名算法
	// HMAC_SHA256 使用HMAC-SHA256算法
	HMAC_SHA256 SignatureAlgorithmName = "HMAC-SHA256"

	// HMAC_SHA512 使用HMAC-SHA512算法
	HMAC_SHA512 SignatureAlgorithmName = "HMAC-SHA512"

	// 基于RSA的签名算法
	// RSA_PKCS1_SHA256 使用RSA-PKCS1-SHA256算法
	RSA_PKCS1_SHA256 SignatureAlgorithmName = "RSA-PKCS1-SHA256"

	// RSA_PSS_SHA256 使用RSA-PSS-SHA256算法
	RSA_PSS_SHA256 SignatureAlgorithmName = "RSA-PSS-SHA256"

	// RSA_PKCS1_SHA512 使用RSA-PKCS1-SHA512算法
	RSA_PKCS1_SHA512 SignatureAlgorithmName = "RSA-PKCS1-SHA512"

	// RSA_PSS_SHA512 使用RSA-PSS-SHA512算法
	RSA_PSS_SHA512 SignatureAlgorithmName = "RSA-PSS-SHA512"

	// 基于ECDSA的签名算法
	// ECDSA_P256_SHA256 使用ECDSA-P256-SHA256算法
	ECDSA_P256_SHA256 SignatureAlgorithmName = "ECDSA-P256-SHA256"

	// ECDSA_P384_SHA384 使用ECDSA-P384-SHA384算法
	ECDSA_P384_SHA384 SignatureAlgorithmName = "ECDSA-P384-SHA384"

	// 基于EdDSA的签名算法
	// ED25519 使用Ed25519算法
	ED25519 SignatureAlgorithmName = "ED25519"
)

// SignatureAlgorithmInfo 签名算法信息
type SignatureAlgorithmInfo struct {
	// 算法名称
	Name string

	// An instance of AlgorithmType
	Type string

	// 签名长度（字节）
	SignatureLength int

	// 密钥类型
	KeyType string

	// 描述
	Description string
}

// SignOptions 签名选项
type SignOptions struct {
	// 算法名称
	Algorithm SignatureAlgorithmName

	// 附加数据（可选）
	AdditionalData []byte
}

// SignedData 签名数据结构
type SignedData struct {
	// 算法名称
	Algorithm string `json:"algorithm"`

	// 原始数据哈希
	DataHash []byte `json:"data_hash,omitempty"`

	// 签名数据
	Signature []byte `json:"signature"`

	// 附加数据
	AdditionalData []byte `json:"additional_data,omitempty"`
}

// signFunc 签名函数类型
type signFunc func(ctx context.Context, algorithm string, privateKey []byte, data []byte) ([]byte, error)

// verifyFunc 验证函数类型
type verifyFunc func(ctx context.Context, algorithm string, publicKey []byte, data []byte, signature []byte) (bool, error)

// signatureAlgorithmInfo 算法信息
type signatureAlgorithmInfo struct {
	Type            AlgorithmType
	SignatureLength int
	KeyType         KeyType
	Description     string
	Sign            signFunc
	Verify          verifyFunc
}

// ECDSASignature ECDSA签名结构
type ECDSASignature struct {
	R, S *big.Int
}

// DefaultSignatureProvider 默认签名提供者实现
type DefaultSignatureProvider struct {
	keyManager KeyManager
	algorithms map[string]*signatureAlgorithmInfo
}

// NewDefaultSignatureProvider 创建默认签名提供者
func NewDefaultSignatureProvider(keyManager KeyManager) *DefaultSignatureProvider {
	provider := &DefaultSignatureProvider{
		keyManager: keyManager,
		algorithms: make(map[string]*signatureAlgorithmInfo),
	}

	// 注册支持的算法
	provider.registerAlgorithms()
	return provider
}

// registerAlgorithms 注册支持的算法
func (p *DefaultSignatureProvider) registerAlgorithms() {
	// HMAC算法
	p.algorithms[string(HMAC_SHA256)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 32, // SHA-256 输出长度为32字节
		KeyType:         SymmetricKey,
		Description:     "HMAC using SHA-256",
		Sign:            p.signHMAC,
		Verify:          p.verifyHMAC,
	}

	p.algorithms[string(HMAC_SHA512)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 64, // SHA-512 输出长度为64字节
		KeyType:         SymmetricKey,
		Description:     "HMAC using SHA-512",
		Sign:            p.signHMAC,
		Verify:          p.verifyHMAC,
	}

	// RSA算法
	p.algorithms[string(RSA_PKCS1_SHA256)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 256, // RSA-2048 签名长度为256字节
		KeyType:         RSAPrivateKey,
		Description:     "RSA PKCS#1 v1.5 with SHA-256",
		Sign:            p.signRSA,
		Verify:          p.verifyRSA,
	}

	p.algorithms[string(RSA_PSS_SHA256)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 256, // RSA-2048 签名长度为256字节
		KeyType:         RSAPrivateKey,
		Description:     "RSA-PSS with SHA-256",
		Sign:            p.signRSA,
		Verify:          p.verifyRSA,
	}

	p.algorithms[string(RSA_PKCS1_SHA512)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 256, // RSA-2048 签名长度为256字节
		KeyType:         RSAPrivateKey,
		Description:     "RSA PKCS#1 v1.5 with SHA-512",
		Sign:            p.signRSA,
		Verify:          p.verifyRSA,
	}

	p.algorithms[string(RSA_PSS_SHA512)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 256, // RSA-2048 签名长度为256字节
		KeyType:         RSAPrivateKey,
		Description:     "RSA-PSS with SHA-512",
		Sign:            p.signRSA,
		Verify:          p.verifyRSA,
	}

	// ECDSA算法
	p.algorithms[string(ECDSA_P256_SHA256)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 64, // ECDSA P-256 签名长度为64字节(r,s各32字节)
		KeyType:         ECPrivateKey,
		Description:     "ECDSA using P-256 curve with SHA-256",
		Sign:            p.signECDSA,
		Verify:          p.verifyECDSA,
	}

	p.algorithms[string(ECDSA_P384_SHA384)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 96, // ECDSA P-384 签名长度为96字节(r,s各48字节)
		KeyType:         ECPrivateKey,
		Description:     "ECDSA using P-384 curve with SHA-384",
		Sign:            p.signECDSA,
		Verify:          p.verifyECDSA,
	}

	// EdDSA算法
	p.algorithms[string(ED25519)] = &signatureAlgorithmInfo{
		Type:            SignatureAlgorithm,
		SignatureLength: 64, // Ed25519 签名长度为64字节
		KeyType:         ED25519PrivateKey,
		Description:     "Edwards-curve Digital Signature Algorithm using Curve25519",
		Sign:            p.signEdDSA,
		Verify:          p.verifyEdDSA,
	}
}

// Sign 使用指定的算法和密钥对数据进行签名
func (p *DefaultSignatureProvider) Sign(ctx context.Context, algorithm string, privateKey []byte, data []byte) ([]byte, error) {
	// 获取算法信息
	info, exists := p.algorithms[algorithm]
	if !exists {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// 调用算法特定的签名函数
	return info.Sign(ctx, algorithm, privateKey, data)
}

// Verify 验证数据签名
func (p *DefaultSignatureProvider) Verify(ctx context.Context, algorithm string, publicKey []byte, data []byte, signature []byte) (bool, error) {
	// 获取算法信息
	info, exists := p.algorithms[algorithm]
	if !exists {
		return false, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// 调用算法特定的验证函数
	return info.Verify(ctx, algorithm, publicKey, data, signature)
}

// GetAlgorithmInfo 获取签名算法的信息
func (p *DefaultSignatureProvider) GetAlgorithmInfo(ctx context.Context, algorithm string) (SignatureAlgorithmInfo, error) {
	info, exists := p.algorithms[algorithm]
	if !exists {
		return SignatureAlgorithmInfo{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	return SignatureAlgorithmInfo{
		Name:            algorithm,
		Type:            string(info.Type),
		SignatureLength: info.SignatureLength,
		KeyType:         string(info.KeyType),
		Description:     info.Description,
	}, nil
}

// ListSupportedAlgorithms 列出所有支持的签名算法
func (p *DefaultSignatureProvider) ListSupportedAlgorithms() []SignatureAlgorithmName {
	algorithms := make([]SignatureAlgorithmName, 0, len(p.algorithms))
	for name := range p.algorithms {
		algorithms = append(algorithms, SignatureAlgorithmName(name))
	}
	return algorithms
}

// signHMAC 使用HMAC算法进行签名
func (p *DefaultSignatureProvider) signHMAC(ctx context.Context, algorithm string, key []byte, data []byte) ([]byte, error) {
	var h func() hash.Hash
	switch algorithm {
	case string(HMAC_SHA256):
		h = sha256.New
	case string(HMAC_SHA512):
		h = sha512.New
	default:
		return nil, fmt.Errorf("unsupported HMAC algorithm: %s", algorithm)
	}

	// 创建HMAC哈希实例
	mac := hmac.New(h, key)
	mac.Write(data)
	signature := mac.Sum(nil)

	// 构造签名数据
	signedData := &SignedData{
		Algorithm: algorithm,
		Signature: signature,
	}

	// 序列化签名数据
	serialized, err := json.Marshal(signedData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize signed data: %w", err)
	}

	return serialized, nil
}

// verifyHMAC 验证HMAC签名
func (p *DefaultSignatureProvider) verifyHMAC(ctx context.Context, algorithm string, key []byte, data []byte, signatureBytes []byte) (bool, error) {
	// 反序列化签名数据
	var signedData SignedData
	if err := json.Unmarshal(signatureBytes, &signedData); err != nil {
		return false, fmt.Errorf("failed to deserialize signed data: %w", err)
	}

	// 验证算法一致性
	if signedData.Algorithm != algorithm {
		return false, fmt.Errorf("algorithm mismatch: expected %s, got %s", algorithm, signedData.Algorithm)
	}

	// 使用相同参数重新计算HMAC
	var h func() hash.Hash
	switch algorithm {
	case string(HMAC_SHA256):
		h = sha256.New
	case string(HMAC_SHA512):
		h = sha512.New
	default:
		return false, fmt.Errorf("unsupported HMAC algorithm: %s", algorithm)
	}

	mac := hmac.New(h, key)
	mac.Write(data)
	expectedSignature := mac.Sum(nil)

	// 比较签名
	return hmac.Equal(signedData.Signature, expectedSignature), nil
}

// signRSA 使用RSA算法进行签名
func (p *DefaultSignatureProvider) signRSA(ctx context.Context, algorithm string, privateKey []byte, data []byte) ([]byte, error) {
	// 解析私钥
	key, err := x509.ParsePKCS8PrivateKey(privateKey)
	if err != nil {
		// 尝试PKCS1格式
		key, err = x509.ParsePKCS1PrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
		}
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an RSA private key")
	}

	// 确定哈希函数和签名类型
	var h crypto.Hash
	var isPSS bool
	switch algorithm {
	case string(RSA_PKCS1_SHA256):
		h = crypto.SHA256
		isPSS = false
	case string(RSA_PSS_SHA256):
		h = crypto.SHA256
		isPSS = true
	case string(RSA_PKCS1_SHA512):
		h = crypto.SHA512
		isPSS = false
	case string(RSA_PSS_SHA512):
		h = crypto.SHA512
		isPSS = true
	default:
		return nil, fmt.Errorf("unsupported RSA algorithm: %s", algorithm)
	}

	// 计算数据哈希
	hasher := h.New()
	hasher.Write(data)
	hashed := hasher.Sum(nil)

	// 签名
	var signature []byte
	if isPSS {
		// 使用PSS填充
		opts := &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
			Hash:       h,
		}
		signature, err = rsa.SignPSS(rand.Reader, rsaKey, h, hashed, opts)
	} else {
		// 使用PKCS1v15填充
		signature, err = rsa.SignPKCS1v15(rand.Reader, rsaKey, h, hashed)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	// 构造签名数据
	signedData := &SignedData{
		Algorithm: algorithm,
		DataHash:  hashed,
		Signature: signature,
	}

	// 序列化签名数据
	serialized, err := json.Marshal(signedData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize signed data: %w", err)
	}

	return serialized, nil
}

// verifyRSA 验证RSA签名
func (p *DefaultSignatureProvider) verifyRSA(ctx context.Context, algorithm string, publicKey []byte, data []byte, signatureBytes []byte) (bool, error) {
	// 反序列化签名数据
	var signedData SignedData
	if err := json.Unmarshal(signatureBytes, &signedData); err != nil {
		return false, fmt.Errorf("failed to deserialize signed data: %w", err)
	}

	// 验证算法一致性
	if signedData.Algorithm != algorithm {
		return false, fmt.Errorf("algorithm mismatch: expected %s, got %s", algorithm, signedData.Algorithm)
	}

	// 解析公钥
	key, err := x509.ParsePKIXPublicKey(publicKey)
	if err != nil {
		return false, fmt.Errorf("failed to parse RSA public key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return false, errors.New("key is not an RSA public key")
	}

	// 确定哈希函数和签名类型
	var h crypto.Hash
	var isPSS bool
	switch algorithm {
	case string(RSA_PKCS1_SHA256):
		h = crypto.SHA256
		isPSS = false
	case string(RSA_PSS_SHA256):
		h = crypto.SHA256
		isPSS = true
	case string(RSA_PKCS1_SHA512):
		h = crypto.SHA512
		isPSS = false
	case string(RSA_PSS_SHA512):
		h = crypto.SHA512
		isPSS = true
	default:
		return false, fmt.Errorf("unsupported RSA algorithm: %s", algorithm)
	}

	// 计算数据哈希（如果未提供）
	var hashed []byte
	if signedData.DataHash != nil {
		hashed = signedData.DataHash
	} else {
		hasher := h.New()
		hasher.Write(data)
		hashed = hasher.Sum(nil)
	}

	// 验证签名
	var err2 error
	if isPSS {
		// 使用PSS填充
		opts := &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
			Hash:       h,
		}
		err2 = rsa.VerifyPSS(rsaKey, h, hashed, signedData.Signature, opts)
	} else {
		// 使用PKCS1v15填充
		err2 = rsa.VerifyPKCS1v15(rsaKey, h, hashed, signedData.Signature)
	}

	return err2 == nil, nil
}

// signECDSA 使用ECDSA算法进行签名
func (p *DefaultSignatureProvider) signECDSA(ctx context.Context, algorithm string, privateKey []byte, data []byte) ([]byte, error) {
	// 解析私钥
	key, err := x509.ParsePKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ECDSA private key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an ECDSA private key")
	}

	// 确定哈希函数
	var h crypto.Hash
	switch algorithm {
	case string(ECDSA_P256_SHA256):
		h = crypto.SHA256
	case string(ECDSA_P384_SHA384):
		h = crypto.SHA384
	default:
		return nil, fmt.Errorf("unsupported ECDSA algorithm: %s", algorithm)
	}

	// 计算数据哈希
	hasher := h.New()
	hasher.Write(data)
	hashed := hasher.Sum(nil)

	// 签名
	r, s, err := ecdsa.Sign(rand.Reader, ecdsaKey, hashed)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	// 序列化r和s为一个签名
	signature, err := marshalECDSASignature(r, s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ECDSA signature: %w", err)
	}

	// 构造签名数据
	signedData := &SignedData{
		Algorithm: algorithm,
		DataHash:  hashed,
		Signature: signature,
	}

	// 序列化签名数据
	serialized, err := json.Marshal(signedData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize signed data: %w", err)
	}

	return serialized, nil
}

// verifyECDSA 验证ECDSA签名
func (p *DefaultSignatureProvider) verifyECDSA(ctx context.Context, algorithm string, publicKey []byte, data []byte, signatureBytes []byte) (bool, error) {
	// 反序列化签名数据
	var signedData SignedData
	if err := json.Unmarshal(signatureBytes, &signedData); err != nil {
		return false, fmt.Errorf("failed to deserialize signed data: %w", err)
	}

	// 验证算法一致性
	if signedData.Algorithm != algorithm {
		return false, fmt.Errorf("algorithm mismatch: expected %s, got %s", algorithm, signedData.Algorithm)
	}

	// 解析公钥
	key, err := x509.ParsePKIXPublicKey(publicKey)
	if err != nil {
		return false, fmt.Errorf("failed to parse ECDSA public key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return false, errors.New("key is not an ECDSA public key")
	}

	// 确定哈希函数
	var h crypto.Hash
	switch algorithm {
	case string(ECDSA_P256_SHA256):
		h = crypto.SHA256
	case string(ECDSA_P384_SHA384):
		h = crypto.SHA384
	default:
		return false, fmt.Errorf("unsupported ECDSA algorithm: %s", algorithm)
	}

	// 计算数据哈希（如果未提供）
	var hashed []byte
	if signedData.DataHash != nil {
		hashed = signedData.DataHash
	} else {
		hasher := h.New()
		hasher.Write(data)
		hashed = hasher.Sum(nil)
	}

	// 解析签名为r, s
	r, s, err := unmarshalECDSASignature(signedData.Signature)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal ECDSA signature: %w", err)
	}

	// 验证签名
	valid := ecdsa.Verify(ecdsaKey, hashed, r, s)
	return valid, nil
}

// signEdDSA 使用EdDSA算法进行签名
func (p *DefaultSignatureProvider) signEdDSA(ctx context.Context, algorithm string, privateKey []byte, data []byte) ([]byte, error) {
	// 验证算法
	if algorithm != string(ED25519) {
		return nil, fmt.Errorf("unsupported EdDSA algorithm: %s", algorithm)
	}

	// 解析私钥
	edKey, err := ParseEd25519PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Ed25519 private key: %w", err)
	}

	// 直接签名
	signature := ed25519.Sign(edKey, data)

	// 构造签名数据
	signedData := &SignedData{
		Algorithm: algorithm,
		Signature: signature,
	}

	// 序列化签名数据
	serialized, err := json.Marshal(signedData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize signed data: %w", err)
	}

	return serialized, nil
}

// verifyEdDSA 验证EdDSA签名
func (p *DefaultSignatureProvider) verifyEdDSA(ctx context.Context, algorithm string, publicKey []byte, data []byte, signatureBytes []byte) (bool, error) {
	// 验证算法
	if algorithm != string(ED25519) {
		return false, fmt.Errorf("unsupported EdDSA algorithm: %s", algorithm)
	}

	// 反序列化签名数据
	var signedData SignedData
	if err := json.Unmarshal(signatureBytes, &signedData); err != nil {
		return false, fmt.Errorf("failed to deserialize signed data: %w", err)
	}

	// 验证算法一致性
	if signedData.Algorithm != algorithm {
		return false, fmt.Errorf("algorithm mismatch: expected %s, got %s", algorithm, signedData.Algorithm)
	}

	// 解析公钥
	edKey, err := ParseEd25519PublicKey(publicKey)
	if err != nil {
		return false, fmt.Errorf("failed to parse Ed25519 public key: %w", err)
	}

	// 验证签名
	valid := ed25519.Verify(edKey, data, signedData.Signature)
	return valid, nil
}

// ParseEd25519PrivateKey 解析Ed25519私钥
func ParseEd25519PrivateKey(key []byte) (ed25519.PrivateKey, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid Ed25519 private key size: got %d, want %d", len(key), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(key), nil
}

// ParseEd25519PublicKey 解析Ed25519公钥
func ParseEd25519PublicKey(key []byte) (ed25519.PublicKey, error) {
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 public key size: got %d, want %d", len(key), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(key), nil
}

// marshalECDSASignature 将ECDSA签名的r和s序列化为一个字节数组
func marshalECDSASignature(r, s *big.Int) ([]byte, error) {
	// 使用ASN.1编码
	signature := ECDSASignature{
		R: r,
		S: s,
	}
	return asn1.Marshal(signature)
}

// unmarshalECDSASignature 从字节数组中解析r和s
func unmarshalECDSASignature(signature []byte) (r, s *big.Int, err error) {
	var ecdsaSignature ECDSASignature
	_, err = asn1.Unmarshal(signature, &ecdsaSignature)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal ECDSA signature: %w", err)
	}
	return ecdsaSignature.R, ecdsaSignature.S, nil
}
