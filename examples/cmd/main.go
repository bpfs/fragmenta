// fragmenta-examples 是FragDB存储引擎的示例程序集
package main

import (
	"fmt"
	"os"
	"path/filepath"

	advancedExample "github.com/bpfs/fragmenta/examples/advanced"
	example "github.com/bpfs/fragmenta/examples/basic"
)

func main() {
	// 显示版本信息
	fmt.Println("FragDB 示例程序 (开发版)")
	fmt.Println("这些示例展示了FragDB的主要功能和API用法")
	fmt.Println()

	// 检查命令行参数
	if len(os.Args) < 2 {
		showUsage()
		return
	}

	// 根据命令行参数选择示例
	switch os.Args[1] {
	case "basic":
		example.BasicUsage()
	case "query":
		example.AdvancedQueryExample()
	case "storage":
		example.ShowStorageModesExample()
	case "security":
		example.SecurityExample()
	case "advanced":
		advancedExample.QueryExamples()
	case "fuse":
		showFuseExampleInfo()
	case "all":
		runAllExamples()
	default:
		fmt.Printf("未知示例: %s\n", os.Args[1])
		showUsage()
	}
}

// showUsage 显示使用说明
func showUsage() {
	fmt.Println("使用方法:")
	fmt.Printf("  %s <example>\n", filepath.Base(os.Args[0]))
	fmt.Println("可用示例:")
	fmt.Println("  basic    - 基本用法示例")
	fmt.Println("  query    - 简单查询示例")
	fmt.Println("  storage  - 存储模式示例")
	fmt.Println("  security - 安全性示例")
	fmt.Println("  advanced - 高级查询示例（复杂条件组合和排序分页）")
	fmt.Println("  fuse     - FUSE挂载示例（请参见examples/experimental/fuse目录）")
	fmt.Println("  all      - 运行所有示例")
}

// runAllExamples 按顺序运行所有示例
func runAllExamples() {
	fmt.Println("======= 运行所有 FragDB 示例 =======")

	fmt.Println("\n1. 基本使用示例")
	fmt.Println("-------------------------")
	example.BasicUsage()

	fmt.Println("\n2. 高级查询示例")
	fmt.Println("-------------------------")
	example.AdvancedQueryExample()

	fmt.Println("\n3. 存储模式示例")
	fmt.Println("-------------------------")
	example.ShowStorageModesExample()

	fmt.Println("\n4. 安全功能示例")
	fmt.Println("-------------------------")
	example.SecurityExample()

	fmt.Println("\n======= 所有示例执行完毕 =======")
}

// showFuseExampleInfo 显示FUSE示例的使用说明
func showFuseExampleInfo() {
	// 获取当前工作目录
	wd, err := os.Getwd()
	if err == nil {
		// 构造FUSE示例绝对路径
		linuxPath := filepath.Join(wd, "examples", "experimental", "fuse", "linux")
		macPath := filepath.Join(wd, "examples", "experimental", "fuse", "mac")

		fmt.Println("=== FragDB FUSE挂载示例 (实验性功能) ===")
		fmt.Println("FUSE示例位于:")
		fmt.Printf("- Linux: %s\n", linuxPath)
		fmt.Printf("- macOS: %s\n", macPath)
		fmt.Println("\n请参考 examples/experimental/fuse/README.md 文件获取详细信息")
	} else {
		fmt.Println("=== FragDB FUSE挂载示例 (实验性功能) ===")
		fmt.Println("FUSE示例位于: examples/experimental/fuse/")
		fmt.Println("请参考该目录下的README.md文件获取详细信息")
	}

	fmt.Println("\n使用前提:")
	fmt.Println("1. Linux系统: 确保已安装libfuse-dev")
	fmt.Println("2. MacOS系统: 需要安装macFUSE")

	fmt.Println("\n运行示例:")
	fmt.Println("# Linux版本")
	fmt.Println("cd examples/experimental/fuse/linux")
	fmt.Println("go build -o fuse-mount && ./fuse-mount -mount /mnt/fragdb -storage data.frag -create")

	fmt.Println("\n# macOS版本")
	fmt.Println("cd examples/experimental/fuse/mac")
	fmt.Println("go build -o fuse-mount-mac && ./fuse-mount-mac -mount /Volumes/fragdb -storage data.frag -create")

	fmt.Println("\n注意: 这是一个实验性功能，优先级较低，当前某些API可能尚未实现")
}
