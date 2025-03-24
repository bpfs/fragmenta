package example

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/bpfs/fragmenta/security"
)

// 演示签名子系统的使用
func SignatureExample() {
	// 创建一个密钥管理器
	storage, err := security.NewFileSecureStorage("./keys")
	if err != nil {
		log.Fatalf("创建安全存储失败: %v", err)
	}
	keyManager := security.NewDefaultKeyManager(storage)

	// 创建签名提供者
	signatureProvider := security.NewDefaultSignatureProvider(keyManager)

	// 列出支持的签名算法
	fmt.Println("支持的签名算法:")
	for _, algo := range signatureProvider.ListSupportedAlgorithms() {
		info, _ := signatureProvider.GetAlgorithmInfo(context.Background(), string(algo))
		fmt.Printf("- %s (%s): %s\n", algo, info.KeyType, info.Description)
	}

	// 使用不同算法进行签名和验证
	exampleData := []byte("这是一条需要签名的重要消息")

	// 1. HMAC签名
	fmt.Println("\n===== HMAC签名示例 =====")
	hmacDemo(signatureProvider, exampleData)

	// 2. RSA签名
	fmt.Println("\n===== RSA签名示例 =====")
	rsaDemo(signatureProvider, exampleData)

	// 3. ECDSA签名
	fmt.Println("\n===== ECDSA签名示例 =====")
	ecdsaDemo(signatureProvider, exampleData)

	// 4. Ed25519签名
	fmt.Println("\n===== Ed25519签名示例 =====")
	ed25519Demo(signatureProvider, exampleData)
}

// HMAC签名示例
func hmacDemo(provider security.SignatureProvider, data []byte) {
	// 生成HMAC密钥
	hmacKey := make([]byte, 32)
	_, err := rand.Read(hmacKey)
	if err != nil {
		log.Fatalf("生成HMAC密钥失败: %v", err)
	}

	// 使用HMAC-SHA256签名
	signature, err := provider.Sign(context.Background(), string(security.HMAC_SHA256), hmacKey, data)
	if err != nil {
		log.Fatalf("HMAC签名失败: %v", err)
	}
	fmt.Printf("HMAC签名结果: %s\n", hex.EncodeToString(signature[:20])+"..."+hex.EncodeToString(signature[len(signature)-20:]))

	// 验证签名
	valid, err := provider.Verify(context.Background(), string(security.HMAC_SHA256), hmacKey, data, signature)
	if err != nil {
		log.Fatalf("HMAC验证失败: %v", err)
	}
	fmt.Printf("签名验证结果: %v\n", valid)

	// 使用错误的密钥验证
	wrongKey := make([]byte, 32)
	_, _ = rand.Read(wrongKey)
	valid, _ = provider.Verify(context.Background(), string(security.HMAC_SHA256), wrongKey, data, signature)
	fmt.Printf("使用错误密钥验证结果: %v\n", valid)
}

// RSA签名示例
func rsaDemo(provider security.SignatureProvider, data []byte) {
	// 生成RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 将私钥转换为PKCS8格式
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		log.Fatalf("序列化RSA私钥失败: %v", err)
	}

	// 将公钥转换为PKIX格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		log.Fatalf("序列化RSA公钥失败: %v", err)
	}

	// 使用RSA-PSS-SHA256签名
	signature, err := provider.Sign(context.Background(), string(security.RSA_PSS_SHA256), privateKeyBytes, data)
	if err != nil {
		log.Fatalf("RSA签名失败: %v", err)
	}
	fmt.Printf("RSA-PSS签名结果: %s\n", hex.EncodeToString(signature[:20])+"..."+hex.EncodeToString(signature[len(signature)-20:]))

	// 验证签名
	valid, err := provider.Verify(context.Background(), string(security.RSA_PSS_SHA256), publicKeyBytes, data, signature)
	if err != nil {
		log.Fatalf("RSA验证失败: %v", err)
	}
	fmt.Printf("签名验证结果: %v\n", valid)

	// 使用错误的数据验证
	wrongData := []byte("被篡改的数据")
	valid, _ = provider.Verify(context.Background(), string(security.RSA_PSS_SHA256), publicKeyBytes, wrongData, signature)
	fmt.Printf("篡改数据后验证结果: %v\n", valid)
}

// ECDSA签名示例
func ecdsaDemo(provider security.SignatureProvider, data []byte) {
	// 生成ECDSA密钥对
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("生成ECDSA密钥对失败: %v", err)
	}

	// 将私钥转换为PKCS8格式
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		log.Fatalf("序列化ECDSA私钥失败: %v", err)
	}

	// 将公钥转换为PKIX格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		log.Fatalf("序列化ECDSA公钥失败: %v", err)
	}

	// 使用ECDSA-P256-SHA256签名
	signature, err := provider.Sign(context.Background(), string(security.ECDSA_P256_SHA256), privateKeyBytes, data)
	if err != nil {
		log.Fatalf("ECDSA签名失败: %v", err)
	}
	fmt.Printf("ECDSA签名结果: %s\n", hex.EncodeToString(signature[:20])+"..."+hex.EncodeToString(signature[len(signature)-20:]))

	// 验证签名
	valid, err := provider.Verify(context.Background(), string(security.ECDSA_P256_SHA256), publicKeyBytes, data, signature)
	if err != nil {
		log.Fatalf("ECDSA验证失败: %v", err)
	}
	fmt.Printf("签名验证结果: %v\n", valid)
}

// Ed25519签名示例
func ed25519Demo(provider security.SignatureProvider, data []byte) {
	// 生成Ed25519密钥对
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("生成Ed25519密钥对失败: %v", err)
	}

	// 使用Ed25519签名
	signature, err := provider.Sign(context.Background(), string(security.ED25519), privateKey, data)
	if err != nil {
		log.Fatalf("Ed25519签名失败: %v", err)
	}
	fmt.Printf("Ed25519签名结果: %s\n", hex.EncodeToString(signature[:20])+"..."+hex.EncodeToString(signature[len(signature)-20:]))

	// 验证签名
	valid, err := provider.Verify(context.Background(), string(security.ED25519), publicKey, data, signature)
	if err != nil {
		log.Fatalf("Ed25519验证失败: %v", err)
	}
	fmt.Printf("签名验证结果: %v\n", valid)
}
