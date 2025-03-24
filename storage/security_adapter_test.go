package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bpfs/fragmenta/security"
)

// TestStorageSecurityIntegration 测试存储安全集成
func TestStorageSecurityIntegration(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "storage_security_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 设置存储和安全路径
	storagePath := filepath.Join(tempDir, "storage")
	securityPath := filepath.Join(tempDir, "security")
	keysPath := filepath.Join(securityPath, "keys")

	// 创建所需目录
	for _, dir := range []string{storagePath, securityPath, keysPath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("创建目录失败 %s: %v", dir, err)
		}
	}

	// 设置存储配置
	storageConfig := &StorageConfig{
		Type:                 StorageTypeContainer,
		Path:                 filepath.Join(storagePath, "container.db"),
		AutoConvertThreshold: 0, // 禁用自动转换
		BlockSize:            1024,
		InlineThreshold:      512,
		DedupEnabled:         false,
		CacheSize:            1024 * 1024, // 1MB
		CachePolicy:          "lru",
	}

	// 直接创建存储管理器
	storageManager, err := NewStorageManager(storageConfig)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}
	defer storageManager.Close()

	// 创建安全配置
	secConfig := &security.SecurityConfig{
		EncryptionEnabled: true,
		DefaultAlgorithm:  security.AES256GCM,
		KeyStorePath:      keysPath,
		AutoGenerateKey:   true,
	}

	// 创建安全管理器
	securityManager, err := security.NewDefaultSecurityManager(secConfig)
	if err != nil {
		t.Fatalf("创建安全管理器失败: %v", err)
	}

	// 初始化安全管理器
	ctx := context.Background()
	if err := securityManager.Initialize(ctx); err != nil {
		t.Fatalf("初始化安全管理器失败: %v", err)
	}
	defer securityManager.Shutdown(ctx)

	// 确保生成了默认密钥
	keyManager := securityManager.GetKeyManager()
	if keyManager == nil {
		t.Fatalf("无法获取密钥管理器")
	}

	// 生成主密钥
	masterKeyID, err := keyManager.GenerateKey(ctx, security.MasterKey, &security.KeyOptions{
		Type:     security.MasterKey,
		Size:     256,
		Usage:    []security.KeyUsage{security.EncryptionUsage},
		Metadata: map[string]string{"description": "测试主密钥"},
	})
	if err != nil {
		t.Fatalf("生成主密钥失败: %v", err)
	}
	t.Logf("已生成主密钥，ID: %s", masterKeyID)

	// 手动设置安全管理器
	err = storageManager.SetSecurityManager(securityManager)
	if err != nil {
		t.Fatalf("设置安全管理器失败: %v", err)
	}

	// 启用加密
	err = storageManager.SetEncryptionEnabled(true)
	if err != nil {
		t.Fatalf("启用加密失败: %v", err)
	}

	// 验证加密是否已启用
	if !storageManager.IsEncryptionEnabled() {
		t.Fatalf("加密应该被启用，但实际状态为禁用")
	}

	// 写入测试数据
	testData := []byte("这是一段用于测试加密功能的敏感数据")
	blockID := uint32(1)

	t.Logf("写入加密块，ID: %d, 大小: %d 字节", blockID, len(testData))
	if err := storageManager.WriteBlock(blockID, testData); err != nil {
		t.Fatalf("写入块失败: %v", err)
	}

	// 直接读取存储文件，验证数据是否已加密
	// 仅作为测试目的，实际应用不应绕过存储管理器直接访问
	t.Log("验证存储文件中的数据已加密")
	fileData, err := os.ReadFile(storageConfig.Path)
	if err != nil {
		t.Fatalf("读取存储文件失败: %v", err)
	}

	// 查找原始数据的匹配，如果找到匹配，则表明数据未加密
	isPlaintext := containsSubsequence(fileData, testData)
	if isPlaintext {
		t.Errorf("在存储文件中找到未加密的数据，加密可能未正常工作")
	} else {
		t.Log("没有在存储文件中找到未加密的数据，加密正常工作")
	}

	// 通过存储管理器读取数据
	t.Log("通过存储管理器读取加密数据")
	readData, err := storageManager.ReadBlock(blockID)
	if err != nil {
		t.Fatalf("读取块失败: %v", err)
	}

	// 验证解密后的数据是否与原始数据一致
	if string(readData) != string(testData) {
		t.Fatalf("读取的数据与原始数据不匹配\n预期: %s\n实际: %s", testData, readData)
	}
	t.Log("加解密验证成功，读取的数据与原始数据一致")

	// 测试禁用加密
	t.Log("测试禁用加密功能")
	if err := storageManager.SetEncryptionEnabled(false); err != nil {
		t.Fatalf("禁用加密失败: %v", err)
	}

	// 验证加密状态
	if storageManager.IsEncryptionEnabled() {
		t.Fatalf("加密应该被禁用，但实际状态为启用")
	}

	// 写入未加密数据
	blockID = uint32(2)
	t.Logf("写入未加密块，ID: %d, 大小: %d 字节", blockID, len(testData))
	if err := storageManager.WriteBlock(blockID, testData); err != nil {
		t.Fatalf("写入块失败: %v", err)
	}

	// 读取数据并验证
	t.Log("读取未加密数据")
	readData, err = storageManager.ReadBlock(blockID)
	if err != nil {
		t.Fatalf("读取块失败: %v", err)
	}

	if string(readData) != string(testData) {
		t.Fatalf("读取的数据与原始数据不匹配\n预期: %s\n实际: %s", testData, readData)
	}
	t.Log("未加密数据验证成功")

	// 重新启用加密
	t.Log("重新启用加密")
	if err := storageManager.SetEncryptionEnabled(true); err != nil {
		t.Fatalf("启用加密失败: %v", err)
	}

	// 验证加密状态
	if !storageManager.IsEncryptionEnabled() {
		t.Fatalf("加密应该被启用，但实际状态为禁用")
	}

	// 写入第三个块
	blockID = uint32(3)
	t.Logf("加密重新启用后写入块，ID: %d, 大小: %d 字节", blockID, len(testData))
	if err := storageManager.WriteBlock(blockID, testData); err != nil {
		t.Fatalf("写入块失败: %v", err)
	}

	// 读取并验证所有三个块
	t.Log("验证所有块的数据完整性")
	for id := uint32(1); id <= 3; id++ {
		readData, err = storageManager.ReadBlock(id)
		if err != nil {
			t.Fatalf("读取块 %d 失败: %v", id, err)
		}

		if string(readData) != string(testData) {
			t.Fatalf("块 %d 的数据与原始数据不匹配", id)
		}
		t.Logf("块 %d 验证成功", id)
	}

	// 测试手动加密/解密功能
	t.Log("测试手动加密/解密功能")
	plaintext := []byte("手动加密测试数据")

	// 手动加密
	encrypted, err := storageManager.EncryptBlock(blockID, plaintext)
	if err != nil {
		t.Fatalf("手动加密失败: %v", err)
	}

	// 手动解密
	decrypted, err := storageManager.DecryptBlock(blockID, encrypted)
	if err != nil {
		t.Fatalf("手动解密失败: %v", err)
	}

	// 验证手动加解密结果
	if string(decrypted) != string(plaintext) {
		t.Fatalf("手动加解密结果与原始数据不匹配\n预期: %s\n实际: %s", plaintext, decrypted)
	}
	t.Log("手动加解密验证成功")

	t.Log("存储安全集成测试完成")
}

// 辅助函数，检查切片是否包含子序列
func containsSubsequence(data, subseq []byte) bool {
	if len(subseq) > len(data) {
		return false
	}

	for i := 0; i <= len(data)-len(subseq); i++ {
		matched := true
		for j := 0; j < len(subseq); j++ {
			if data[i+j] != subseq[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}
