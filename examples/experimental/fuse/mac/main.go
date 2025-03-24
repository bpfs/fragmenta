// FragDB macOS FUSE挂载示例
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bpfs/fragmenta"
	"github.com/bpfs/fragmenta/examples/experimental/fuse/common"
)

var (
	mountPoint  = flag.String("mount", "/Volumes/fragdb", "挂载点路径")
	storagePath = flag.String("storage", "fuse-example-mac.frag", "存储文件路径")
	createMode  = flag.Bool("create", false, "创建新存储文件")
	debugMode   = flag.Bool("debug", false, "启用调试模式")
	volName     = flag.String("volname", "FragDB存储", "卷名称")
)

func main() {
	flag.Parse()

	fmt.Println("=== FragDB macOS FUSE挂载示例 (实验性) ===")
	fmt.Printf("存储文件：%s\n", *storagePath)
	fmt.Printf("挂载点：%s\n", *mountPoint)

	// 检查macFUSE是否已安装
	if !checkMacFUSE() {
		fmt.Println("错误: 未检测到macFUSE")
		fmt.Println("请安装macFUSE后再运行此示例:")
		fmt.Println("1. 下载地址: https://github.com/osxfuse/osxfuse/releases")
		fmt.Println("2. 或使用Homebrew安装: brew install --cask macfuse")
		os.Exit(1)
	}

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

	// 配置macOS FUSE选项
	macFuseOptions := &common.MacFuseOptions{
		AllowRoot:       false,      // 是否允许root访问
		AllowOther:      false,      // 是否允许其他用户访问
		VolumeName:      *volName,   // 卷名称
		NoAppleDouble:   true,       // 禁止创建.AppleDouble文件
		NoBrowse:        false,      // 是否允许在Finder中浏览
		FilePermissions: 0644,       // 文件默认权限
		DirPermissions:  0755,       // 目录默认权限
		Debug:           *debugMode, // 调试模式
	}

	fmt.Println("正在将FragDB挂载为macOS FUSE文件系统...")

	// 创建macOS特定的挂载处理器
	mounter, err := common.NewMacFuseMounter(storage, macFuseOptions)
	if err != nil {
		fmt.Printf("创建macOS FUSE挂载器失败: %v\n", err)
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
	fmt.Println("提示: 此卷应当出现在Finder的设备列表中")
	fmt.Println("      您也可以使用Terminal访问:")
	fmt.Printf("      ls -la %s\n", *mountPoint)
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
		fmt.Printf("  或在Finder中右键点击卷图标选择'推出'选项\n")
		os.Exit(1)
	}

	fmt.Println("FUSE文件系统已成功卸载")
}

// 检查macFUSE是否已安装
func checkMacFUSE() bool {
	// 方法1：检查macFUSE内核扩展
	if _, err := os.Stat("/Library/Filesystems/macfuse.fs"); err == nil {
		return true
	}

	// 方法2：检查包信息
	cmd := exec.Command("pkgutil", "--pkg-info", "com.github.osxfuse.pkg.MacFUSE")
	if err := cmd.Run(); err == nil {
		return true
	}

	// 方法3：检查macFUSE工具
	cmd = exec.Command("which", "mount_macfuse")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// 创建示例文件
func createExampleFiles(storage fragmenta.Fragmenta) {
	fmt.Println("正在创建示例文件...")

	// 创建目录结构
	dirs := []string{
		"/文档",
		"/图片",
		"/视频",
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
		filePath := fmt.Sprintf("%s/说明.txt", dir)
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
	welcomeContent := []byte(`欢迎使用FragDB macOS FUSE挂载功能！

这是一个实验性功能，允许将FragDB存储引擎挂载为macOS文件系统。
您可以像操作普通文件一样在Finder中浏览和修改FragDB存储中的内容。

注意事项：
1. 这是一个实验性功能，可能存在限制和性能问题
2. 大文件的读写可能会较慢
3. 某些特殊文件操作可能不被支持
4. 此功能依赖macFUSE，可能需要授予额外的系统权限

了解更多信息，请访问项目文档。`)

	// 写入欢迎文件内容
	welcomeBlockID, err := storage.WriteBlock(welcomeContent, nil)
	if err != nil {
		fmt.Printf("写入欢迎文件内容失败: %v\n", err)
	} else {
		// 设置欢迎文件元数据
		storage.SetMetadata(fragmenta.UserTag(0x2100), []byte("/欢迎使用.txt"))
		storage.SetMetadata(fragmenta.UserTag(0x3100), fragmenta.EncodeInt64(int64(welcomeBlockID)))
		fmt.Printf("创建欢迎文件: /欢迎使用.txt (块ID: %d)\n", welcomeBlockID)
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
	case "/文档":
		return "文档、文本文件和其他文档资料"
	case "/图片":
		return "图片、图像文件和图表"
	case "/视频":
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
