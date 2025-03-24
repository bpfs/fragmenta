package security

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestEnvironment 设置测试环境
func setupTestEnvironment(t *testing.T) (string, *DefaultSecurityManager) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "fragmenta-security-test-*")
	if err != nil {
		t.Fatalf("创建临时测试目录失败: %v", err)
	}

	// 创建安全配置
	securityConfig := &SecurityConfig{
		EncryptionEnabled: true,
		DefaultAlgorithm:  AES256GCM,
		KeyStorePath:      filepath.Join(tempDir, "keys"),
		AutoGenerateKey:   true,
	}

	// 确保密钥目录存在
	os.MkdirAll(securityConfig.KeyStorePath, 0755)

	// 创建安全管理器
	securityManager, err := NewDefaultSecurityManager(securityConfig)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("创建安全管理器失败: %v", err)
	}

	// 初始化安全管理器
	err = securityManager.Initialize(context.Background())
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("初始化安全管理器失败: %v", err)
	}

	return tempDir, securityManager
}

// teardownTestEnvironment 清理测试环境
func teardownTestEnvironment(tempDir string, securityManager *DefaultSecurityManager) {
	if securityManager != nil {
		securityManager.Shutdown(context.Background())
	}
	os.RemoveAll(tempDir)
}

// generateRandomData 生成随机数据用于测试
func generateRandomData(size int) []byte {
	data := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, data)
	if err != nil {
		panic(fmt.Sprintf("生成随机数据失败: %v", err))
	}
	return data
}

// TestSecurityManagerInitialization 测试安全管理器初始化
func TestSecurityManagerInitialization(t *testing.T) {
	tempDir, securityManager := setupTestEnvironment(t)
	defer teardownTestEnvironment(tempDir, securityManager)

	// 验证组件状态
	if securityManager.encryptionProvider == nil {
		t.Error("加密提供者未初始化")
	}

	if securityManager.keyManager == nil {
		t.Error("密钥管理器未初始化")
	}

	// 验证安全管理器是否正常运行
	if !securityManager.IsInitialized() {
		t.Error("安全管理器未完成初始化")
	}
}

// TestKeyManagement 测试密钥管理功能
func TestKeyManagement(t *testing.T) {
	tempDir, securityManager := setupTestEnvironment(t)
	defer teardownTestEnvironment(tempDir, securityManager)

	keyManager := securityManager.GetKeyManager()

	// 测试密钥生成
	keyID, err := keyManager.GenerateKey(context.Background(), SymmetricKey, &KeyOptions{
		Type: SymmetricKey,
		Size: 256,
		Metadata: map[string]string{
			"description": "测试密钥",
			"created":     time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	if keyID == "" {
		t.Fatal("生成的密钥ID为空")
	}

	// 测试密钥列表
	keys, err := keyManager.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("列出密钥失败: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("密钥列表为空")
	}

	// 确认新生成的密钥存在于列表中
	keyFound := false
	for _, id := range keys {
		if id == keyID {
			keyFound = true
			break
		}
	}
	if !keyFound {
		t.Fatalf("在密钥列表中未找到生成的密钥: %s", keyID)
	}

	// 测试获取密钥信息
	// 注意：安全原因，我们无法直接获取密钥元数据，这里只检查密钥是否存在
	exists, err := keyManager.KeyExists(context.Background(), keyID)
	if err != nil {
		t.Fatalf("检查密钥存在性失败: %v", err)
	}
	if !exists {
		t.Fatalf("密钥 %s 应该存在，但检查结果为不存在", keyID)
	}
}

// TestEncryptionDecryption 测试加密和解密功能
func TestEncryptionDecryption(t *testing.T) {
	tempDir, securityManager := setupTestEnvironment(t)
	defer teardownTestEnvironment(tempDir, securityManager)

	// 准备测试数据
	testSizes := []int{
		64,      // 小块数据
		4096,    // 标准块大小
		1048576, // 1MB
	}

	blockID := uint32(12345)

	// 对不同大小的数据进行测试
	for _, size := range testSizes {
		t.Run(fmt.Sprintf("Size_%d", size), func(t *testing.T) {
			plaintext := generateRandomData(size)

			// 加密数据
			ciphertext, err := securityManager.EncryptBlock(context.Background(), blockID, plaintext)
			if err != nil {
				t.Fatalf("加密数据失败: %v", err)
			}

			// 验证密文与明文不同
			if bytes.Equal(plaintext, ciphertext) {
				t.Error("加密后的数据与原始数据相同，这可能表明数据未被加密")
			}

			// 解密数据
			decrypted, err := securityManager.DecryptBlock(context.Background(), blockID, ciphertext)
			if err != nil {
				t.Fatalf("解密数据失败: %v", err)
			}

			// 验证解密后的数据与原始数据相同
			if !bytes.Equal(plaintext, decrypted) {
				t.Error("解密后的数据与原始数据不匹配")
			}
		})
	}
}

// TestStreamEncryptionDecryption 测试流式加密和解密
func TestStreamEncryptionDecryption(t *testing.T) {
	t.Skip("流式加密测试已被跳过，因为EncryptStream/DecryptStream方法已被移除")

	/*
		tempDir, securityManager := setupTestEnvironment(t)
		defer teardownTestEnvironment(tempDir, securityManager)

		encryptionProvider := securityManager.GetEncryptionProvider()
		keyManager := securityManager.GetKeyManager()

		// 列出所有密钥
		keys, err := keyManager.ListKeys(context.Background())
		if err != nil {
			t.Fatalf("列出密钥失败: %v", err)
		}

		// 使用默认密钥ID
		var keyID string
		if len(keys) > 0 {
			keyID = keys[0]
		} else {
			t.Skip("没有可用的密钥，跳过测试")
		}

		// 测试不同大小的数据块
		dataSizes := []int{1024, 8192, 65536} // 1KB, 8KB, 64KB
		blockID := uint32(12345)

		for _, size := range dataSizes {
			t.Run(fmt.Sprintf("DataSize_%d", size), func(t *testing.T) {
				// 生成随机数据
				plaintext := generateRandomData(size)
				srcBuf := bytes.NewReader(plaintext)
				encryptedBuf := &bytes.Buffer{}
				decryptedBuf := &bytes.Buffer{}

				// 流式加密
				err := encryptionProvider.EncryptStream(
					context.Background(),
					srcBuf,
					encryptedBuf,
					keyID,
					&EncryptionOptions{
						Algorithm: AES256GCM,
						BlockID:   blockID,
					},
				)
				if err != nil {
					t.Fatalf("流式加密失败: %v", err)
				}

				// 流式解密
				err = encryptionProvider.DecryptStream(
					context.Background(),
					bytes.NewReader(encryptedBuf.Bytes()),
					decryptedBuf,
					keyID,
					&EncryptionOptions{
						Algorithm: AES256GCM,
						BlockID:   blockID,
					},
				)
				if err != nil {
					t.Fatalf("流式解密失败: %v", err)
				}

				// 验证数据一致性
				if !bytes.Equal(plaintext, decryptedBuf.Bytes()) {
					t.Fatalf("解密数据与原始数据不匹配")
				}
			})
		}
	*/
}

// TestAlgorithmSupport 测试支持的加密算法
func TestAlgorithmSupport(t *testing.T) {
	tempDir, securityManager := setupTestEnvironment(t)
	defer teardownTestEnvironment(tempDir, securityManager)

	encryptionProvider := securityManager.GetEncryptionProvider()

	// 列出支持的算法并验证
	algorithms := encryptionProvider.ListSupportedAlgorithms()
	if len(algorithms) == 0 {
		t.Fatal("没有支持的加密算法")
	}

	// 确认至少支持AES-GCM算法
	aesGCMSupported := false
	for _, algo := range algorithms {
		if algo == AES256GCM {
			aesGCMSupported = true
			break
		}
	}
	if !aesGCMSupported {
		t.Fatal("不支持AES-GCM算法")
	}

	// 获取并验证算法信息
	algoInfo, err := encryptionProvider.GetAlgorithmInfo(context.Background(), string(AES256GCM))
	if err != nil {
		t.Fatalf("获取算法信息失败: %v", err)
	}
	if algoInfo.Name == "" {
		t.Fatal("获取的算法信息无效")
	}
}

// TestSecureStorage 测试安全存储功能
func TestSecureStorage(t *testing.T) {
	tempDir, securityManager := setupTestEnvironment(t)
	defer teardownTestEnvironment(tempDir, securityManager)

	// 创建文件安全存储
	secureStoragePath := filepath.Join(tempDir, "secure_storage")
	os.MkdirAll(secureStoragePath, 0755)

	secureStorage, err := NewFileSecureStorage(secureStoragePath)
	if err != nil {
		t.Fatalf("创建文件安全存储失败: %v", err)
	}

	// 测试数据
	storageKey := "test_key"
	storageValue := []byte("这是一个安全存储的测试数据")

	// 存储数据
	err = secureStorage.Store(context.Background(), storageKey, storageValue)
	if err != nil {
		t.Fatalf("存储数据失败: %v", err)
	}

	// 检查数据是否存在 - 通过尝试检索来检查存在性
	// 使用Retrieve方法检查存在性
	_, err = secureStorage.Retrieve(context.Background(), storageKey)
	if err != nil {
		t.Fatalf("检索数据失败，可能表示存储操作未成功: %v", err)
	}

	// 检索数据
	retrievedValue, err := secureStorage.Retrieve(context.Background(), storageKey)
	if err != nil {
		t.Fatalf("检索数据失败: %v", err)
	}
	if !bytes.Equal(storageValue, retrievedValue) {
		t.Fatal("检索到的数据与存储的数据不匹配")
	}

	// 删除数据
	err = secureStorage.Delete(context.Background(), storageKey)
	if err != nil {
		t.Fatalf("删除数据失败: %v", err)
	}

	// 确认数据已被删除 - 尝试检索应该失败
	_, err = secureStorage.Retrieve(context.Background(), storageKey)
	if err == nil {
		t.Fatal("数据应该已被删除，但仍然可以检索")
	}
}
