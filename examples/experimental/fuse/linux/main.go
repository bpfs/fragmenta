// FragDB FUSE挂载示例
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bpfs/fragmenta"
	"github.com/bpfs/fragmenta/examples/experimental/fuse/common"
)

var (
	mountPoint  = flag.String("mount", "/tmp/fragmenta-mount", "挂载点路径")
	storagePath = flag.String("storage", "fuse-example.frag", "存储文件路径")
	createMode  = flag.Bool("create", false, "创建新存储文件")
	debugMode   = flag.Bool("debug", false, "启用调试模式")
)

func main() {
	flag.Parse()

	fmt.Println("=== FragDB FUSE挂载示例 (实验性) ===")
	fmt.Printf("存储文件：%s\n", *storagePath)
	fmt.Printf("挂载点：%s\n", *mountPoint)

	// 检查挂载点是否存在，如不存在则创建
	if _, err := os.Stat(*mountPoint); os.IsNotExist(err) {
		fmt.Printf("挂载点 %s 不存在，正在创建...\n", *mountPoint)
		if err := os.MkdirAll(*mountPoint, 0755); err != nil {
			fmt.Printf("创建挂载点失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 创建或打开FragDB存储
	var storage fragmenta.Fragmenta
	var err error

	if *createMode || !fileExists(*storagePath) {
		fmt.Println("创建新的FragDB存储文件...")

		// 创建存储选项
		options := &fragmenta.FragmentaOptions{
			StorageMode: 0, // ContainerMode
		}

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(*storagePath), 0755); err != nil {
			fmt.Printf("创建存储目录失败: %v\n", err)
			os.Exit(1)
		}

		// 创建存储
		storage, err = fragmenta.CreateFragmenta(*storagePath, options)
		if err != nil {
			fmt.Printf("创建FragDB存储失败: %v\n", err)
			os.Exit(1)
		}

		// 添加一些示例文件
		createExampleFiles(storage)
	} else {
		fmt.Println("打开现有FragDB存储文件...")
		storage, err = fragmenta.OpenFragmenta(*storagePath)
		if err != nil {
			fmt.Printf("打开FragDB存储失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 确保关闭存储
	defer storage.Close()

	// 配置FUSE选项
	fuseOptions := &common.FuseMountOptions{
		AllowOther:  false,           // 是否允许其他用户访问
		ReadOnly:    false,           // 是否以只读模式挂载
		Debug:       *debugMode,      // 调试模式
		FSName:      "FragDB",        // 文件系统名称
		VolumeName:  "FragDB Volume", // 卷名称
		Permissions: 0755,            // 默认权限
	}

	fmt.Println("正在将FragDB挂载为FUSE文件系统...")

	// 创建挂载处理器
	mounter, err := common.NewFuseMounter(storage, fuseOptions)
	if err != nil {
		fmt.Printf("创建FUSE挂载器失败: %v\n", err)
		os.Exit(1)
	}

	// 设置信号处理，以便在程序终止时卸载
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 后台执行挂载
	errCh := make(chan error, 1)
	go func() {
		errCh <- mounter.Mount(*mountPoint)
	}()

	fmt.Printf("FUSE文件系统已挂载到 %s\n", *mountPoint)
	fmt.Println("提示: 可以使用以下命令查看挂载的文件系统:")
	fmt.Printf("  ls -la %s\n", *mountPoint)
	fmt.Println("按 Ctrl+C 停止挂载")

	// 等待信号或错误
	select {
	case sig := <-sigCh:
		fmt.Printf("收到信号 %s，准备卸载...\n", sig)
	case err := <-errCh:
		if err != nil {
			fmt.Printf("挂载过程中出错: %v\n", err)
		}
	}

	// 卸载
	fmt.Println("正在卸载FUSE文件系统...")
	if err := mounter.Unmount(); err != nil {
		fmt.Printf("卸载失败: %v\n", err)
		fmt.Println("尝试手动卸载命令:")
		fmt.Printf("  umount %s\n", *mountPoint)
		os.Exit(1)
	}

	fmt.Println("FUSE文件系统已成功卸载")
}

// 创建示例文件
func createExampleFiles(storage fragmenta.Fragmenta) {
	fmt.Println("正在创建示例文件...")

	// 创建目录结构
	dirs := []string{
		"/docs",
		"/images",
		"/videos",
	}

	for i, dir := range dirs {
		// 使用元数据标记目录
		err := storage.SetMetadata(fragmenta.UserTag(0x1000+uint16(i)), []byte(dir))
		if err != nil {
			fmt.Printf("创建目录 %s 时出错: %v\n", dir, err)
			continue
		}

		// 为每个目录创建README文件
		content := []byte(fmt.Sprintf("这是 %s 目录的说明文件\n\n此目录用于存储%s。",
			dir, getDirDescription(dir)))

		// 写入文件内容
		blockID, err := storage.WriteBlock(content, nil)
		if err != nil {
			fmt.Printf("写入README内容失败: %v\n", err)
			continue
		}

		// 设置文件元数据
		filePath := fmt.Sprintf("%s/README.txt", dir)
		err = storage.SetMetadata(fragmenta.UserTag(0x2000+uint16(i)), []byte(filePath))
		if err != nil {
			fmt.Printf("设置文件路径失败: %v\n", err)
			continue
		}

		// 将文件与数据块关联
		err = storage.SetMetadata(fragmenta.UserTag(0x3000+uint16(i)), fragmenta.EncodeInt64(int64(blockID)))
		if err != nil {
			fmt.Printf("关联文件与数据块失败: %v\n", err)
			continue
		}

		fmt.Printf("创建示例文件: %s (块ID: %d)\n", filePath, blockID)
	}

	// 创建根目录的欢迎文件
	welcomeContent := []byte(`欢迎使用FragDB FUSE挂载功能！

这是一个实验性功能，允许将FragDB存储引擎挂载为本地文件系统。
你可以像操作普通文件系统一样浏览和修改FragDB中的内容。

注意事项：
1. 这是一个实验性功能，可能存在限制和性能问题
2. 大文件的读写可能会较慢
3. 某些特殊文件操作可能不被支持

了解更多信息，请访问项目文档。`)

	// 写入欢迎文件内容
	welcomeBlockID, err := storage.WriteBlock(welcomeContent, nil)
	if err != nil {
		fmt.Printf("写入欢迎文件内容失败: %v\n", err)
	} else {
		// 设置欢迎文件元数据
		storage.SetMetadata(fragmenta.UserTag(0x2100), []byte("/welcome.txt"))
		storage.SetMetadata(fragmenta.UserTag(0x3100), fragmenta.EncodeInt64(int64(welcomeBlockID)))
		fmt.Printf("创建欢迎文件: /welcome.txt (块ID: %d)\n", welcomeBlockID)
	}

	// 提交更改
	if err := storage.Commit(); err != nil {
		fmt.Printf("提交更改失败: %v\n", err)
	} else {
		fmt.Println("示例文件创建完成并已提交")
	}
}

// 根据目录名获取描述
func getDirDescription(dir string) string {
	switch dir {
	case "/docs":
		return "文档、文本文件和其他文档资料"
	case "/images":
		return "图片、图像文件和图表"
	case "/videos":
		return "视频文件和动画内容"
	default:
		return "各类文件"
	}
}

// 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
