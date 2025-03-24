package example

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/bpfs/fragmenta/security"
)

// SecurityExample 展示安全模块的使用方法
func SecurityExample() {
	fmt.Println("=== Fragmenta 安全模块示例 ===")

	// 创建安全配置
	fmt.Println("\n1. 创建安全管理器")
	securityConfig := &security.SecurityConfig{
		EncryptionEnabled: true,
		DefaultAlgorithm:  security.AES256GCM,
		KeyStorePath:      "./data/keys",
		AutoGenerateKey:   true,
	}

	// 确保目录存在
	os.MkdirAll(securityConfig.KeyStorePath, 0755)

	// 创建安全管理器
	securityManager, err := security.NewDefaultSecurityManager(securityConfig)
	if err != nil {
		fmt.Printf("创建安全管理器失败: %v\n", err)
		return
	}

	// 初始化安全子系统
	fmt.Println("\n2. 初始化安全子系统")
	err = securityManager.Initialize(context.Background())
	if err != nil {
		fmt.Printf("初始化安全子系统失败: %v\n", err)
		return
	}

	// 获取加密提供者和密钥管理器
	fmt.Println("\n3. 获取加密提供者和密钥管理器")
	encryptionProvider := securityManager.GetEncryptionProvider()
	keyManager := securityManager.GetKeyManager()

	// 列出支持的加密算法
	fmt.Println("\n4. 列出支持的加密算法")
	algorithms := encryptionProvider.ListSupportedAlgorithms()
	for i, algo := range algorithms {
		algoInfo, err := encryptionProvider.GetAlgorithmInfo(context.Background(), string(algo))
		if err != nil {
			continue
		}
		fmt.Printf("  %d. %s (密钥大小: %d 位)\n", i+1, algoInfo.Name, algoInfo.KeySize)
	}

	// 列出所有密钥
	fmt.Println("\n5. 列出所有密钥")
	keys, err := keyManager.ListKeys(context.Background())
	if err != nil {
		fmt.Printf("列出密钥失败: %v\n", err)
	} else {
		if len(keys) == 0 {
			fmt.Println("  没有可用的密钥")
		} else {
			for i, keyID := range keys {
				fmt.Printf("  %d. %s\n", i+1, keyID)
			}
		}
	}

	// 使用默认密钥ID
	var keyID string
	if len(keys) > 0 {
		keyID = keys[0]
	} else {
		// 生成新密钥
		fmt.Println("\n6. 生成新密钥")
		keyID, err = keyManager.GenerateKey(context.Background(), security.SymmetricKey, &security.KeyOptions{
			Type: security.SymmetricKey,
			Size: 256,
			Metadata: map[string]string{
				"description": "示例密钥",
				"usage":       "example",
			},
		})
		if err != nil {
			fmt.Printf("生成密钥失败: %v\n", err)
			return
		}
		fmt.Printf("  新密钥ID: %s\n", keyID)
	}

	// 加密和解密数据块
	fmt.Println("\n7. 加密和解密数据块")
	plaintext := []byte("这是一个需要加密的敏感数据块")
	blockID := uint32(12345)

	// 加密数据
	fmt.Println("  加密数据")
	ciphertext, err := securityManager.EncryptBlock(context.Background(), blockID, plaintext)
	if err != nil {
		fmt.Printf("加密数据失败: %v\n", err)
		return
	}
	fmt.Printf("  原始数据大小: %d 字节\n", len(plaintext))
	fmt.Printf("  加密后大小: %d 字节\n", len(ciphertext))

	// 解密数据
	fmt.Println("  解密数据")
	decrypted, err := securityManager.DecryptBlock(context.Background(), blockID, ciphertext)
	if err != nil {
		fmt.Printf("解密数据失败: %v\n", err)
		return
	}
	fmt.Printf("  解密后大小: %d 字节\n", len(decrypted))

	// 验证数据一致性
	if bytes.Equal(plaintext, decrypted) {
		fmt.Println("  解密成功: 数据一致")
	} else {
		fmt.Println("  解密失败: 数据不一致")
	}

	// 注意：流式加密功能已被移除
	fmt.Println("\n8. 流式加密功能已被移除")

	// 关闭安全子系统
	fmt.Println("\n9. 关闭安全子系统")
	err = securityManager.Shutdown(context.Background())
	if err != nil {
		fmt.Printf("关闭安全子系统失败: %v\n", err)
	} else {
		fmt.Println("  安全子系统已关闭")
	}

	fmt.Println("\n=== 安全模块示例结束 ===")
}

// RunSecurityExample 运行安全子系统的使用示例
func RunSecurityExample() {
	fmt.Println("=== 运行安全子系统示例 ===")

	// 创建安全存储
	secureStorage, err := security.NewFileSecureStorage("/tmp/fragmenta-security")
	if err != nil {
		fmt.Printf("创建安全存储失败: %v\n", err)
		os.Exit(1)
	}

	// 创建密钥管理器
	keyManager := security.NewDefaultKeyManager(secureStorage)

	// 创建加密提供者
	encryptionProvider := security.NewDefaultEncryptionProvider(keyManager)

	// 创建上下文
	ctx := context.Background()

	// 演示对称加密
	demoSymmetricEncryption(ctx, keyManager, encryptionProvider)

	// 演示非对称加密
	demoAsymmetricEncryption(ctx, keyManager, encryptionProvider)
}

// demoSymmetricEncryption 演示对称加密操作
func demoSymmetricEncryption(ctx context.Context, km *security.DefaultKeyManager, ep *security.DefaultEncryptionProvider) {
	fmt.Println("\n=== 对称加密演示 ===")

	// 生成对称密钥
	keyID, err := km.GenerateKey(ctx, security.SymmetricKey, &security.KeyOptions{
		Size: 32, // 256位/8 = 32字节
		Metadata: map[string]string{
			"purpose": "data-encryption",
			"app":     "fragmenta-example",
		},
	})
	if err != nil {
		fmt.Printf("生成对称密钥失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("生成对称密钥: %s\n", keyID)

	// 获取密钥数据
	keyData, err := km.GetKey(ctx, keyID)
	if err != nil {
		fmt.Printf("获取密钥失败: %v\n", err)
		os.Exit(1)
	}

	// 加密数据
	plaintext := []byte("这是一个需要加密的敏感数据示例")
	ciphertext, err := ep.Encrypt(ctx, "AES-256-GCM", keyData, plaintext, nil)
	if err != nil {
		fmt.Printf("加密数据失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("加密数据 (Hex): %s\n", hex.EncodeToString(ciphertext))

	// 解密数据
	decrypted, err := ep.Decrypt(ctx, "AES-256-GCM", keyData, ciphertext, nil)
	if err != nil {
		fmt.Printf("解密数据失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("解密数据: %s\n", string(decrypted))
}

// demoAsymmetricEncryption 演示非对称加密操作
func demoAsymmetricEncryption(ctx context.Context, km *security.DefaultKeyManager, ep *security.DefaultEncryptionProvider) {
	fmt.Println("\n=== 非对称加密演示 ===")

	// 生成RSA密钥对
	keyPair, err := km.GenerateKeyPair(ctx, security.RSAPrivateKey, &security.KeyOptions{
		Size: 2048,
		Metadata: map[string]string{
			"purpose": "asymmetric-encryption",
			"app":     "fragmenta-example",
		},
	})
	if err != nil {
		fmt.Printf("生成RSA密钥对失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("生成RSA密钥对:\n私钥ID: %s\n公钥ID: %s\n", keyPair.PrivateKeyID, keyPair.PublicKeyID)

	// 获取公钥
	publicKeyData, err := km.GetKey(ctx, keyPair.PublicKeyID)
	if err != nil {
		fmt.Printf("获取公钥失败: %v\n", err)
		os.Exit(1)
	}

	// 获取私钥
	privateKeyData, err := km.GetKey(ctx, keyPair.PrivateKeyID)
	if err != nil {
		fmt.Printf("获取私钥失败: %v\n", err)
		os.Exit(1)
	}

	// 使用公钥加密数据
	plaintext := []byte("这是一个使用非对称加密的敏感数据示例")
	ciphertext, err := ep.EncryptWithPublicKey(ctx, "RSA-2048-OAEP-SHA256", publicKeyData, plaintext, nil)
	if err != nil {
		fmt.Printf("使用公钥加密数据失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("公钥加密数据 (Hex): %s\n", hex.EncodeToString(ciphertext))

	// 使用私钥解密数据
	decrypted, err := ep.DecryptWithPrivateKey(ctx, "RSA-2048-OAEP-SHA256", privateKeyData, ciphertext, nil)
	if err != nil {
		fmt.Printf("使用私钥解密数据失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("私钥解密数据: %s\n", string(decrypted))

	// 展示密钥元数据
	fmt.Println("\n=== 密钥元数据示例 ===")
	entry, err := km.RetrieveKeyEntry(ctx, keyPair.PrivateKeyID)
	if err != nil {
		fmt.Printf("获取密钥条目失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("密钥元数据:")
	for k, v := range entry.Metadata {
		fmt.Printf("  %s: %s\n", k, v)
	}

	// 列出支持的算法
	fmt.Println("\n=== 支持的加密算法 ===")
	algorithms := ep.ListSupportedAlgorithms()
	fmt.Println("支持的算法:")
	for _, algo := range algorithms {
		fmt.Printf("  %s\n", algo)
	}
}
