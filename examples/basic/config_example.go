package example

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bpfs/fragmenta/config"
)

// ConfigExample 展示配置管理功能的使用方法
func ConfigExample() {
	fmt.Println("=== Fragmenta 配置管理示例 ===")

	// 创建配置管理器
	fmt.Println("\n1. 创建配置管理器")
	manager := config.NewDefaultConfigManager()

	// 获取默认配置并展示
	fmt.Println("\n2. 获取默认配置")
	defaultConfig := manager.GetDefaultConfig()
	printConfigSummary(defaultConfig)

	// 修改配置示例
	fmt.Println("\n3. 修改配置")
	defaultConfig.Storage.Mode = config.DirectoryMode
	defaultConfig.Storage.BlockStrategy.BlockSize = 32 * 1024 // 32KB
	defaultConfig.Security.Encryption.Enabled = true
	defaultConfig.System.RootPath = "./data/fragmenta"

	// 验证配置有效性
	fmt.Println("\n4. 验证配置有效性")
	err := manager.ValidateConfig(context.Background(), defaultConfig)
	if err != nil {
		fmt.Printf("配置验证失败: %v\n", err)
	} else {
		fmt.Println("配置验证通过")
	}

	// 保存配置到文件
	configDir := "./data/config"
	configFile := filepath.Join(configDir, "fragmenta.json")

	// 确保目录存在
	os.MkdirAll(configDir, 0755)

	fmt.Printf("\n5. 保存配置到文件: %s\n", configFile)
	err = manager.SaveConfig(context.Background(), defaultConfig, configFile)
	if err != nil {
		fmt.Printf("保存配置失败: %v\n", err)
	} else {
		fmt.Println("配置保存成功")
	}

	// 从文件加载配置
	fmt.Println("\n6. 从文件加载配置")
	loadedConfig, err := manager.LoadConfig(context.Background(), configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
	} else {
		fmt.Println("配置加载成功")
		printConfigSummary(loadedConfig)
	}

	// 应用配置
	fmt.Println("\n7. 应用配置")
	err = manager.ApplyConfig(context.Background(), loadedConfig)
	if err != nil {
		fmt.Printf("应用配置失败: %v\n", err)
	} else {
		fmt.Println("配置应用成功")
	}

	// 创建一个配置变更监听器
	fmt.Println("\n8. 演示配置变更监听")
	listener := &ExampleConfigListener{}
	manager.RegisterConfigChangeListener(listener)

	// 模拟配置变更
	loadedConfig.System.LogLevel = "debug"
	err = manager.ApplyConfig(context.Background(), loadedConfig)
	if err != nil {
		fmt.Printf("应用配置失败: %v\n", err)
	}

	fmt.Println("\n=== 配置管理示例结束 ===")
}

// ExampleConfigListener 配置变更监听器示例
type ExampleConfigListener struct{}

// OnConfigChange 配置变更通知
func (l *ExampleConfigListener) OnConfigChange(oldConfig, newConfig *config.Config) {
	fmt.Println("收到配置变更通知:")
	fmt.Printf("  - 旧配置日志级别: %s\n", oldConfig.System.LogLevel)
	fmt.Printf("  - 新配置日志级别: %s\n", newConfig.System.LogLevel)
}

// 辅助函数：打印配置摘要
func printConfigSummary(cfg *config.Config) {
	fmt.Println("配置摘要:")
	fmt.Printf("  - 存储模式: %s\n", cfg.Storage.Mode)
	fmt.Printf("  - 块大小: %d KB\n", cfg.Storage.BlockStrategy.BlockSize/1024)
	fmt.Printf("  - 安全加密: %v\n", cfg.Security.Encryption.Enabled)
	fmt.Printf("  - 索引模式: %s\n", cfg.Index.Mode)
	fmt.Printf("  - 系统根路径: %s\n", cfg.System.RootPath)
	fmt.Printf("  - 日志级别: %s\n", cfg.System.LogLevel)
	fmt.Printf("  - 配置版本: %s\n", cfg.Metadata.Version)
	fmt.Printf("  - 最后更新: %s\n", cfg.Metadata.LastUpdated.Format(time.RFC3339))
}
