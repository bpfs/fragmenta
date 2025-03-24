package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bpfs/fragmenta/security"
	"github.com/bpfs/fragmenta/storage"
)

func main() {
	// 创建临时目录用于示例
	baseDir, err := os.MkdirTemp("", "security_storage_example")
	if err != nil {
		fmt.Printf("创建临时目录失败: %v\n", err)
		return
	}
	defer os.RemoveAll(baseDir)

	// 创建存储和安全配置路径
	storagePath := filepath.Join(baseDir, "storage")
	securityPath := filepath.Join(baseDir, "security")
	keyStorePath := filepath.Join(securityPath, "keys")

	// 确保目录存在
	os.MkdirAll(storagePath, 0755)
	os.MkdirAll(keyStorePath, 0755)

	fmt.Println("======= 安全存储集成示例 =======")
	fmt.Printf("存储路径: %s\n", storagePath)
	fmt.Printf("安全路径: %s\n", securityPath)

	// 创建存储配置
	storageConfig := &storage.StorageConfig{
		Path:                 storagePath,
		Type:                 storage.StorageTypeContainer,
		BlockSize:            1024,
		AutoConvertThreshold: 0, // 禁用自动转换以简化示例
	}

	// 创建存储管理器
	storageManager, err := storage.NewStorageManager(storageConfig)
	if err != nil {
		fmt.Printf("创建存储管理器失败: %v\n", err)
		return
	}

	// 创建安全管理器
	securityConfig := &security.SecurityConfig{
		EncryptionEnabled: true,
		DefaultAlgorithm:  security.AES256GCM,
		KeyStorePath:      keyStorePath,
		AutoGenerateKey:   true,
	}
	securityManager, err := security.NewDefaultSecurityManager(securityConfig)
	if err != nil {
		fmt.Printf("创建安全管理器失败: %v\n", err)
		return
	}

	// 初始化安全管理器
	ctx := context.Background()
	if err := securityManager.Initialize(ctx); err != nil {
		fmt.Printf("初始化安全管理器失败: %v\n", err)
		return
	}

	// 创建安全存储配置
	securityAdapterConfig := &storage.StorageSecurityConfig{
		EncryptionEnabled:  true,
		SecurityConfigPath: securityPath,
		KeyStorePath:       keyStorePath,
		AutoGenerateKey:    true,
	}

	fmt.Println("\n1. 创建安全存储适配器")
	adapter, err := storage.NewStorageSecurityAdapter(storageManager, securityManager, securityAdapterConfig)
	if err != nil {
		fmt.Printf("创建安全存储适配器失败: %v\n", err)
		return
	}
	defer adapter.Close()

	fmt.Println("2. 获取安全存储管理器")
	secureStorage := adapter.GetStorageManager()

	// 确认加密已启用
	fmt.Printf("加密状态: %v\n", secureStorage.IsEncryptionEnabled())

	// 写入和读取加密数据
	testData := []byte("这是一些敏感数据，需要加密存储。" + time.Now().String())
	blockID := uint32(1)

	fmt.Println("\n3. 写入加密数据")
	err = secureStorage.WriteBlock(blockID, testData)
	if err != nil {
		fmt.Printf("写入数据失败: %v\n", err)
		return
	}
	fmt.Printf("成功写入 %d 字节数据到块 #%d\n", len(testData), blockID)

	// 检查是否加密存储
	fmt.Println("\n4. 验证数据是否已加密")
	// 获取原始存储管理器查看加密数据
	rawStorage, _ := storage.NewStorageManager(storageConfig)
	encryptedData, err := rawStorage.ReadBlock(blockID)
	if err != nil {
		fmt.Printf("读取原始数据失败: %v\n", err)
		return
	}

	// 检查数据是否与原始数据不同（已加密）
	isEncrypted := !compareData(encryptedData, testData)
	fmt.Printf("数据已加密: %v\n", isEncrypted)
	fmt.Printf("加密数据前10字节: %v\n", encryptedData[:min(10, len(encryptedData))])

	fmt.Println("\n5. 读取并自动解密数据")
	decryptedData, err := secureStorage.ReadBlock(blockID)
	if err != nil {
		fmt.Printf("读取解密数据失败: %v\n", err)
		return
	}

	// 验证解密数据与原始数据一致
	isMatch := compareData(decryptedData, testData)
	fmt.Printf("解密数据与原始数据匹配: %v\n", isMatch)
	fmt.Printf("解密数据 (字符串): %s\n", string(decryptedData))

	// 禁用加密并测试
	fmt.Println("\n6. 测试禁用加密功能")
	err = secureStorage.SetEncryptionEnabled(false)
	if err != nil {
		fmt.Printf("禁用加密失败: %v\n", err)
		return
	}

	// 写入非加密数据
	newData := []byte("这是未加密数据")
	newBlockID := uint32(2)
	err = secureStorage.WriteBlock(newBlockID, newData)
	if err != nil {
		fmt.Printf("写入未加密数据失败: %v\n", err)
		return
	}

	// 读取非加密数据
	plainData, err := secureStorage.ReadBlock(newBlockID)
	if err != nil {
		fmt.Printf("读取未加密数据失败: %v\n", err)
		return
	}

	fmt.Printf("未加密数据读取成功: %s\n", string(plainData))

	// 手动加密/解密示例
	fmt.Println("\n7. 手动加密/解密示例")
	manualData := []byte("手动加密的数据")
	manualBlockID := uint32(3)

	// 启用加密
	secureStorage.SetEncryptionEnabled(true)

	// 手动加密
	encData, err := secureStorage.EncryptBlock(manualBlockID, manualData)
	if err != nil {
		fmt.Printf("手动加密失败: %v\n", err)
		return
	}

	// 禁用自动加密并存储已加密数据
	secureStorage.SetEncryptionEnabled(false)
	err = secureStorage.WriteBlock(manualBlockID, encData)
	if err != nil {
		fmt.Printf("写入手动加密数据失败: %v\n", err)
		return
	}

	// 读取加密数据并手动解密
	readEncData, err := secureStorage.ReadBlock(manualBlockID)
	if err != nil {
		fmt.Printf("读取手动加密数据失败: %v\n", err)
		return
	}

	// 手动解密
	decData, err := secureStorage.DecryptBlock(manualBlockID, readEncData)
	if err != nil {
		fmt.Printf("手动解密失败: %v\n", err)
		return
	}

	fmt.Printf("手动解密数据: %s\n", string(decData))
	fmt.Printf("数据匹配: %v\n", compareData(decData, manualData))

	fmt.Println("\n======= 示例完成 =======")
	fmt.Println("已成功演示安全存储集成功能，包括自动加密/解密、切换加密状态和手动加密/解密。")
}

// 比较两个字节数组是否相同
func compareData(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
