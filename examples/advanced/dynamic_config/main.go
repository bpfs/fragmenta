package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/bpfs/fragmenta/config"
)

// StorageConfigListener 存储配置监听器
type StorageConfigListener struct {
	Name string
}

// OnConfigChange 处理配置变更
func (l *StorageConfigListener) OnConfigChange(oldConfig, newConfig *config.Config) {
	fmt.Printf("[%s] 存储配置已变更：\n", l.Name)
	fmt.Printf("  - 存储模式: %s -> %s\n", oldConfig.Storage.Mode, newConfig.Storage.Mode)
	fmt.Printf("  - 块大小: %d -> %d\n", oldConfig.Storage.BlockStrategy.BlockSize, newConfig.Storage.BlockStrategy.BlockSize)
}

// PerformanceConfigListener 性能配置监听器
type PerformanceConfigListener struct {
	Name string
}

// OnConfigChange 处理配置变更
func (l *PerformanceConfigListener) OnConfigChange(oldConfig, newConfig *config.Config) {
	fmt.Printf("[%s] 性能配置已变更：\n", l.Name)
	fmt.Printf("  - 最大工作线程: %d -> %d\n", oldConfig.Performance.Parallelism.MaxWorkers, newConfig.Performance.Parallelism.MaxWorkers)
	fmt.Printf("  - 文件描述符缓存大小: %d -> %d\n", oldConfig.Performance.IO.FDCacheSize, newConfig.Performance.IO.FDCacheSize)
}

func main() {
	ctx := context.Background()

	// 初始化配置管理器
	fmt.Println("初始化动态配置管理器...")
	dynamicManager, err := config.NewDynamicConfigManager()
	if err != nil {
		log.Fatalf("创建动态配置管理器失败: %v", err)
	}
	defer dynamicManager.Close()

	// 创建临时目录用于配置文件和日志
	tempDir, err := os.MkdirTemp("", "config_example")
	if err != nil {
		log.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	logDir := filepath.Join(tempDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("创建日志目录失败: %v", err)
	}

	// 启用变更日志记录功能
	fmt.Println("启用配置变更日志记录...")
	if err := dynamicManager.EnableChangeLog(logDir); err != nil {
		log.Fatalf("启用变更日志记录失败: %v", err)
	}

	// 配置是否记录完整配置
	dynamicManager.SetLogFullConfig(true)

	// 获取默认配置
	config := dynamicManager.GetDefaultConfig()

	// 修改默认配置
	config.Storage.Mode = "directory"
	config.Storage.BlockStrategy.BlockSize = 4096
	config.Performance.Parallelism.MaxWorkers = 4
	config.Performance.IO.FDCacheSize = 8192

	// 保存初始配置到文件
	fmt.Printf("保存初始配置到 %s\n", configPath)
	if err := dynamicManager.SaveConfig(ctx, config, configPath); err != nil {
		log.Fatalf("保存配置失败: %v", err)
	}

	// 使用配置文件初始化并启动监控
	if err := dynamicManager.InitWithConfigFile(ctx, configPath, true); err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	// 设置防抖时间窗口
	dynamicManager.SetDebounceWindow(time.Millisecond * 500)

	// 注册配置变更监听器
	fmt.Println("注册配置监听器...")
	storageListener := &StorageConfigListener{Name: "存储监听器"}
	perfListener := &PerformanceConfigListener{Name: "性能监听器"}

	// 使用分组注册
	dynamicManager.RegisterGroupedListener("storage", storageListener)
	dynamicManager.RegisterGroupedListener("performance", perfListener)

	// 显示当前配置状态
	currentConfig := dynamicManager.GetCurrentConfig()
	fmt.Println("\n当前配置:")
	fmt.Printf("存储模式: %s, 块大小: %d\n", currentConfig.Storage.Mode, currentConfig.Storage.BlockStrategy.BlockSize)
	fmt.Printf("最大工作线程: %d, 文件描述符缓存大小: %d\n", currentConfig.Performance.Parallelism.MaxWorkers, currentConfig.Performance.IO.FDCacheSize)

	// 修改配置并保存
	fmt.Println("\n修改配置...")
	modifiedConfig := dynamicManager.GetCurrentConfig()
	modifiedConfig.Storage.Mode = "hybrid"
	modifiedConfig.Storage.BlockStrategy.BlockSize = 8192
	modifiedConfig.Performance.Parallelism.MaxWorkers = 8
	modifiedConfig.Performance.IO.FDCacheSize = 16384

	// 保存修改后的配置
	fmt.Println("保存修改后的配置...")
	if err := dynamicManager.SaveConfig(ctx, modifiedConfig, configPath); err != nil {
		log.Fatalf("保存修改后的配置失败: %v", err)
	}

	// 等待配置变更通知处理
	fmt.Println("等待配置变更通知处理...")
	time.Sleep(time.Second * 2)

	// 输出日志文件内容
	logFiles, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	if err != nil {
		log.Fatalf("获取日志文件列表失败: %v", err)
	}

	fmt.Println("\n变更日志文件列表:")
	for _, logFile := range logFiles {
		fmt.Printf(" - %s\n", filepath.Base(logFile))
		// 显示日志文件内容摘要
		logData, err := os.ReadFile(logFile)
		if err != nil {
			fmt.Printf("   读取日志失败: %v\n", err)
			continue
		}

		// 尝试解析日志条目
		var entries []interface{}
		if err := json.Unmarshal(logData, &entries); err == nil {
			fmt.Printf("   包含 %d 条日志记录\n", len(entries))
		} else {
			fmt.Printf("   日志内容大小: %d 字节\n", len(logData))
		}
	}

	// 手动修改配置并应用
	fmt.Println("\n手动应用配置变更...")
	manualConfig := dynamicManager.GetCurrentConfig()
	manualConfig.Storage.BlockStrategy.BlockSize = 16384
	manualConfig.Performance.Parallelism.MaxWorkers = 16

	if err := dynamicManager.ApplyConfig(ctx, manualConfig); err != nil {
		log.Fatalf("手动应用配置失败: %v", err)
	}

	// 禁用监听器组
	fmt.Println("\n禁用性能监听器组...")
	dynamicManager.UnregisterGroup("performance")

	// 停止监控配置文件
	fmt.Println("停止配置文件监控...")
	if err := dynamicManager.StopWatching(); err != nil {
		log.Printf("停止监控配置文件失败: %v", err)
	}

	// 禁用变更日志记录
	fmt.Println("禁用变更日志记录...")
	if err := dynamicManager.DisableChangeLog(); err != nil {
		log.Printf("禁用变更日志记录失败: %v", err)
	}

	// 强制重新加载配置
	fmt.Println("\n强制重新加载配置...")
	if err := dynamicManager.ForceReload(ctx); err != nil {
		log.Fatalf("强制重新加载配置失败: %v", err)
	}

	// 显示最终配置状态
	finalConfig := dynamicManager.GetCurrentConfig()
	fmt.Println("\n最终配置:")
	fmt.Printf("存储模式: %s, 块大小: %d\n", finalConfig.Storage.Mode, finalConfig.Storage.BlockStrategy.BlockSize)
	fmt.Printf("最大工作线程: %d, 文件描述符缓存大小: %d\n", finalConfig.Performance.Parallelism.MaxWorkers, finalConfig.Performance.IO.FDCacheSize)

	fmt.Println("\n演示完成，程序退出")
}
