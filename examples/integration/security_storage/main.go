package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bpfs/fragmenta/security"
	"github.com/bpfs/fragmenta/storage"
)

func main() {
	// 创建临时目录作为测试路径
	tempDir, err := ioutil.TempDir("", "secure_storage_example_*")
	if err != nil {
		fmt.Printf("创建临时目录失败: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)
	fmt.Printf("使用临时目录: %s\n", tempDir)

	// 设置存储和安全路径
	storagePath := filepath.Join(tempDir, "storage")
	securityPath := filepath.Join(tempDir, "security")
	keysPath := filepath.Join(securityPath, "keys")

	// 创建所需目录
	for _, dir := range []string{storagePath, securityPath, keysPath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("创建目录失败 %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// 1. 创建存储管理器
	fmt.Println("\n=== 创建存储管理器 ===")
	storageConfig := &storage.StorageConfig{
		Type:                 storage.StorageTypeContainer,
		Path:                 filepath.Join(storagePath, "container.db"),
		AutoConvertThreshold: 0, // 禁用自动转换
		BlockSize:            1024,
		InlineThreshold:      512,
		DedupEnabled:         false,
		CacheSize:            1024 * 1024, // 1MB
		CachePolicy:          "lru",
	}

	storageManager, err := storage.NewStorageManager(storageConfig)
	if err != nil {
		fmt.Printf("创建存储管理器失败: %v\n", err)
		os.Exit(1)
	}
	defer storageManager.Close()
	fmt.Println("存储管理器创建成功")

	// 2. 创建安全管理器
	fmt.Println("\n=== 创建安全管理器 ===")
	secConfig := &security.SecurityConfig{
		EncryptionEnabled: true,
		DefaultAlgorithm:  security.AES256GCM,
		KeyStorePath:      keysPath,
		AutoGenerateKey:   true,
	}

	securityManager, err := security.NewDefaultSecurityManager(secConfig)
	if err != nil {
		fmt.Printf("创建安全管理器失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("安全管理器创建成功")

	// 3. 初始化安全管理器
	ctx := context.Background()
	if err := securityManager.Initialize(ctx); err != nil {
		fmt.Printf("初始化安全管理器失败: %v\n", err)
		os.Exit(1)
	}
	defer securityManager.Shutdown(ctx)
	fmt.Println("安全管理器已初始化")

	// 4. 生成主密钥
	keyManager := securityManager.GetKeyManager()
	if keyManager == nil {
		fmt.Println("无法获取密钥管理器")
		os.Exit(1)
	}

	masterKeyID, err := keyManager.GenerateKey(ctx, security.MasterKey, &security.KeyOptions{
		Type:     security.MasterKey,
		Size:     256,
		Usage:    []security.KeyUsage{security.EncryptionUsage},
		Metadata: map[string]string{"description": "示例主密钥"},
	})
	if err != nil {
		fmt.Printf("生成主密钥失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已生成主密钥，ID: %s\n", masterKeyID)

	// 5. 集成安全管理器与存储管理器
	fmt.Println("\n=== 集成安全与存储 ===")
	err = storageManager.SetSecurityManager(securityManager)
	if err != nil {
		fmt.Printf("设置安全管理器失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("已将安全管理器设置到存储管理器")

	// 6. 启用加密
	err = storageManager.SetEncryptionEnabled(true)
	if err != nil {
		fmt.Printf("启用加密失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("已启用加密功能")

	// 7. 存储加密数据
	fmt.Println("\n=== 使用加密存储数据 ===")
	testData := []byte("这是一段需要加密存储的敏感数据。包含用户ID、密码和其他私密信息。")
	blockID := uint32(1)

	fmt.Printf("准备写入块 ID: %d，数据大小: %d 字节\n", blockID, len(testData))
	fmt.Printf("原始数据: %s\n", testData)

	if err := storageManager.WriteBlock(blockID, testData); err != nil {
		fmt.Printf("写入块失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("数据已加密写入")

	// 8. 读取加密数据
	fmt.Println("\n=== 读取加密数据 ===")
	readData, err := storageManager.ReadBlock(blockID)
	if err != nil {
		fmt.Printf("读取块失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("读取并解密的数据: %s\n", readData)

	// 9. 直接读取存储文件（展示数据已加密）
	fmt.Println("\n=== 验证存储文件中的数据已加密 ===")
	fileData, err := os.ReadFile(storageConfig.Path)
	if err != nil {
		fmt.Printf("读取存储文件失败: %v\n", err)
		os.Exit(1)
	}

	// 检查文件中是否包含原始数据
	isPlaintext := containsSubsequence(fileData, testData)
	if isPlaintext {
		fmt.Println("警告：在存储文件中找到了未加密的原始数据！")
	} else {
		fmt.Println("验证成功：存储文件中找不到未加密的原始数据，数据已正确加密")
	}

	// 10. 禁用/启用加密的演示
	fmt.Println("\n=== 演示禁用加密 ===")
	if err := storageManager.SetEncryptionEnabled(false); err != nil {
		fmt.Printf("禁用加密失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("加密已禁用")

	// 11. 写入未加密数据
	blockID = uint32(2)
	fmt.Printf("写入未加密块，ID: %d\n", blockID)
	if err := storageManager.WriteBlock(blockID, testData); err != nil {
		fmt.Printf("写入块失败: %v\n", err)
		os.Exit(1)
	}

	// 12. 读取未加密数据
	readData, err = storageManager.ReadBlock(blockID)
	if err != nil {
		fmt.Printf("读取块失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("读取未加密数据: %s\n", readData)

	// 13. 重新启用加密
	fmt.Println("\n=== 重新启用加密 ===")
	if err := storageManager.SetEncryptionEnabled(true); err != nil {
		fmt.Printf("启用加密失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("加密已重新启用")

	// 14. 手动加解密示例
	fmt.Println("\n=== 手动加解密示例 ===")
	manualData := []byte("这是手动加密的数据")
	fmt.Printf("原始数据: %s\n", manualData)

	encrypted, err := storageManager.EncryptBlock(blockID, manualData)
	if err != nil {
		fmt.Printf("手动加密失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("加密数据长度: %d 字节\n", len(encrypted))

	decrypted, err := storageManager.DecryptBlock(blockID, encrypted)
	if err != nil {
		fmt.Printf("手动解密失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("解密结果: %s\n", decrypted)

	fmt.Println("\n=== 安全存储示例完成 ===")
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
