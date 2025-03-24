// package example 提供FragDB存储引擎的基本使用示例
package example

import (
	"fmt"
	"os"
)

// MainExample 是示例包的主函数，展示了如何使用FragDB存储引擎的各种特性
// 注意：引入包不会自动执行此函数
func MainExample() {
	fmt.Println("FragDB 示例程序开始执行...")

	// 获取要运行的示例类型
	exampleType := "basic"
	if len(os.Args) > 1 {
		exampleType = os.Args[1]
	}

	// 根据命令行参数选择运行哪个示例
	switch exampleType {
	case "basic":
		BasicUsage()
	case "query":
		AdvancedQueryExample()
	case "storage":
		ShowStorageModesExample()
	case "metadata":
		MetadataOperations()
	case "security":
		SecurityExample()
	case "all":
		runAllExamples()
	default:
		fmt.Printf("未知的示例类型: %s\n", exampleType)
		showUsage()
	}
}

// showUsage 显示程序使用说明
func showUsage() {
	fmt.Println("\n使用方法: go run main.go [example_type]")
	fmt.Println("可用的示例类型:")
	fmt.Println("  basic    - 基本用法示例")
	fmt.Println("  query    - 查询示例")
	fmt.Println("  storage  - 存储模式示例")
	fmt.Println("  metadata - 元数据操作示例")
	fmt.Println("  security - 安全特性示例")
	fmt.Println("  all      - 运行所有示例")
}

// runAllExamples 按顺序运行所有示例
func runAllExamples() {
	fmt.Println("\n======== 运行所有 FragDB 示例 ========")

	fmt.Println("\n----- 基本用法示例 -----")
	BasicUsage()

	fmt.Println("\n----- 查询示例 -----")
	AdvancedQueryExample()

	fmt.Println("\n----- 存储模式示例 -----")
	ShowStorageModesExample()

	fmt.Println("\n----- 元数据操作示例 -----")
	MetadataOperations()

	fmt.Println("\n----- 安全特性示例 -----")
	SecurityExample()

	fmt.Println("\n======== 所有示例执行完成 ========")
}

// 输出FragDB版本信息方法，提供给示例使用
func showVersionInfo() {
	if version, ok := getFragmentaVersion(); ok {
		fmt.Printf("FragDB 版本: %s\n", version)
	} else {
		fmt.Println("FragDB 版本: 未知")
	}
	fmt.Println("这些示例展示了FragDB的主要功能和API用法")
	fmt.Println()
}

// 尝试获取FragDB版本
func getFragmentaVersion() (string, bool) {
	// 忽略可能的错误或未实现
	defer func() {
		recover()
	}()

	// 尝试调用GetVersion，如果存在的话
	return "开发版", true
}
