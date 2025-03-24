// package example 提供FragDB存储引擎的存储模式示例
package example

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bpfs/fragmenta"
)

// 存储模式类型常量 - 这些应该在fragmenta包中定义
// 在这里临时声明以便示例使用
const (
	// FragDB存储模式常量
	FragDBContainerMode uint8 = iota
	FragDBDirectoryMode
	FragDBMemoryMode
	FragDBHybridMode
)

// ShowStorageModesExample 演示FragDB的不同存储模式
// 本示例展示如何使用不同的存储模式并进行相互转换
func ShowStorageModesExample() {
	fmt.Println("=== FragDB 存储模式示例 ===")
	fmt.Println("本示例演示不同的存储模式及其特点")

	// 创建临时目录
	tempDir := "storage_modes_example"
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// 1. 容器模式示例
	containerFile := filepath.Join(tempDir, "container_mode.frag")
	demoContainerMode(containerFile)

	// 2. 目录模式示例
	directoryPath := filepath.Join(tempDir, "directory_mode")
	demoDirectoryMode(directoryPath)

	// 3. 内存模式示例
	demoMemoryMode()

	// 4. 混合模式示例
	demoHybridMode(tempDir)

	// 5. 存储模式转换示例
	demoStorageModeConversion(
		filepath.Join(tempDir, "convert_source.frag"),
		filepath.Join(tempDir, "convert_target"),
	)

	fmt.Println("\n=== 存储模式示例完成 ===")
}

// demoContainerMode 演示容器模式
func demoContainerMode(filePath string) {
	fmt.Println("\n1. 容器模式示例:")
	fmt.Println("   容器模式将所有数据存储在单个文件中")

	// 创建容器模式文件
	db, err := fragmenta.CreateFragmenta(filePath, &fragmenta.FragmentaOptions{
		StorageMode: FragDBContainerMode,
	})
	if err != nil {
		fmt.Printf("   创建容器模式文件失败: %v\n", err)
		return
	}
	defer func() {
		db.Close()
	}()

	// 设置一些元数据
	db.SetMetadata(fragmenta.UserTag(0x1000), []byte("容器模式示例"))
	db.SetMetadata(fragmenta.UserTag(0x1001), []byte(time.Now().Format(time.RFC3339)))

	// 写入一些数据块
	data1 := []byte("这是容器模式的第一个数据块")
	blockID1, err := db.WriteBlock(data1, nil)
	if err != nil {
		fmt.Printf("   写入数据块失败: %v\n", err)
		return
	}

	data2 := []byte("这是容器模式的第二个数据块，包含更多数据")
	blockID2, err := db.WriteBlock(data2, nil)
	if err != nil {
		fmt.Printf("   写入数据块失败: %v\n", err)
		return
	}

	// 提交更改
	err = db.Commit()
	if err != nil {
		fmt.Printf("   提交更改失败: %v\n", err)
		return
	}

	// 文件信息
	info, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("   获取文件信息失败: %v\n", err)
		return
	}

	fmt.Printf("   已创建容器模式文件: %s\n", filePath)
	fmt.Printf("   文件大小: %d 字节\n", info.Size())
	fmt.Printf("   包含 %d 个数据块 (ID: %d, %d)\n", 2, blockID1, blockID2)

	fmt.Println("   容器模式特点:")
	fmt.Println("   - 所有数据存储在单个文件中")
	fmt.Println("   - 便于备份和传输")
	fmt.Println("   - 适合存储中小型数据集")
	fmt.Println("   - 支持原子提交操作")
}

// demoDirectoryMode 演示目录模式
func demoDirectoryMode(dirPath string) {
	fmt.Println("\n2. 目录模式示例:")
	fmt.Println("   目录模式将数据分散存储在多个文件中")

	// 创建必要的目录
	os.MkdirAll(dirPath, 0755)

	// 创建目录模式存储
	dirFile := filepath.Join(dirPath, "index.frag")
	db, err := fragmenta.CreateFragmenta(dirFile, &fragmenta.FragmentaOptions{
		StorageMode: FragDBDirectoryMode,
	})
	if err != nil {
		fmt.Printf("   创建目录模式存储失败: %v\n", err)
		return
	}
	defer func() {
		db.Close()
	}()

	// 设置一些元数据
	db.SetMetadata(fragmenta.UserTag(0x1000), []byte("目录模式示例"))
	db.SetMetadata(fragmenta.UserTag(0x1001), []byte(time.Now().Format(time.RFC3339)))

	// 写入一些大数据块
	largeData1 := make([]byte, 50*1024) // 50KB
	for i := range largeData1 {
		largeData1[i] = byte(i % 256)
	}
	blockID1, err := db.WriteBlock(largeData1, nil)
	if err != nil {
		fmt.Printf("   写入大数据块失败: %v\n", err)
		return
	}

	largeData2 := make([]byte, 100*1024) // 100KB
	for i := range largeData2 {
		largeData2[i] = byte((i + 128) % 256)
	}
	blockID2, err := db.WriteBlock(largeData2, nil)
	if err != nil {
		fmt.Printf("   写入大数据块失败: %v\n", err)
		return
	}

	// 提交更改
	err = db.Commit()
	if err != nil {
		fmt.Printf("   提交更改失败: %v\n", err)
		return
	}

	// 统计目录中的文件数量
	var fileCount int
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})

	fmt.Printf("   已创建目录模式存储: %s\n", dirPath)
	fmt.Printf("   目录中包含 %d 个文件\n", fileCount)
	fmt.Printf("   包含 %d 个数据块 (ID: %d, %d)\n", 2, blockID1, blockID2)

	fmt.Println("   目录模式特点:")
	fmt.Println("   - 数据分散存储在多个文件中")
	fmt.Println("   - 适合大型数据集和大文件")
	fmt.Println("   - 便于增量备份")
	fmt.Println("   - 便于并行读写操作")
}

// demoMemoryMode 演示内存模式
func demoMemoryMode() {
	fmt.Println("\n3. 内存模式示例:")
	fmt.Println("   内存模式将所有数据存储在内存中，不写入磁盘")

	// 创建内存模式存储
	db, err := fragmenta.CreateFragmenta("", &fragmenta.FragmentaOptions{
		StorageMode: FragDBMemoryMode,
	})
	if err != nil {
		fmt.Printf("   创建内存模式存储失败: %v\n", err)
		return
	}
	defer db.Close()

	// 设置一些元数据
	startTime := time.Now()

	const numBlocks = 1000
	for i := 0; i < numBlocks; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		db.SetMetadata(fragmenta.UserTag(uint16(0x1000+i)), []byte(key+":"+value))
	}

	// 写入一些数据块
	const dataSize = 1024 // 1KB
	for i := 0; i < numBlocks; i++ {
		data := make([]byte, dataSize)
		for j := range data {
			data[j] = byte((i + j) % 256)
		}
		_, err := db.WriteBlock(data, nil)
		if err != nil {
			fmt.Printf("   写入数据块失败: %v\n", err)
			return
		}
	}

	elapsedTime := time.Since(startTime)
	opsPerSecond := float64(numBlocks*2) / elapsedTime.Seconds()

	fmt.Printf("   已创建内存模式存储\n")
	fmt.Printf("   存储了 %d 个元数据项和 %d 个数据块\n", numBlocks, numBlocks)
	fmt.Printf("   操作耗时: %v (%.2f 操作/秒)\n", elapsedTime, opsPerSecond)

	fmt.Println("   内存模式特点:")
	fmt.Println("   - 所有数据仅存储在内存中")
	fmt.Println("   - 极高的读写性能")
	fmt.Println("   - 适合临时数据和缓存")
	fmt.Println("   - 进程结束后数据丢失")
	fmt.Println("   - 可以通过导出功能持久化数据")
}

// demoHybridMode 演示混合模式
func demoHybridMode(baseDir string) {
	fmt.Println("\n4. 混合模式示例:")
	fmt.Println("   混合模式结合了容器模式和目录模式的优点")

	hybridPath := filepath.Join(baseDir, "hybrid_mode")
	os.MkdirAll(hybridPath, 0755)

	hybridFile := filepath.Join(hybridPath, "hybrid.frag")
	db, err := fragmenta.CreateFragmenta(hybridFile, &fragmenta.FragmentaOptions{
		StorageMode: FragDBHybridMode,
	})
	if err != nil {
		fmt.Printf("   创建混合模式存储失败: %v\n", err)
		return
	}
	defer db.Close()

	// 设置一些元数据
	db.SetMetadata(fragmenta.UserTag(0x1000), []byte("混合模式示例"))
	db.SetMetadata(fragmenta.UserTag(0x1001), []byte(time.Now().Format(time.RFC3339)))

	// 写入一些小数据块
	for i := 0; i < 5; i++ {
		smallData := []byte(fmt.Sprintf("这是小数据块 #%d", i+1))
		_, err := db.WriteBlock(smallData, nil)
		if err != nil {
			fmt.Printf("   写入小数据块失败: %v\n", err)
			return
		}
	}

	// 写入一些大数据块
	for i := 0; i < 3; i++ {
		largeData := make([]byte, 200*1024) // 200KB
		for j := range largeData {
			largeData[j] = byte((i + j) % 256)
		}
		_, err := db.WriteBlock(largeData, nil)
		if err != nil {
			fmt.Printf("   写入大数据块失败: %v\n", err)
			return
		}
	}

	// 提交更改
	err = db.Commit()
	if err != nil {
		fmt.Printf("   提交更改失败: %v\n", err)
		return
	}

	// 统计目录中的文件数量
	var fileCount int
	filepath.Walk(hybridPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})

	fmt.Printf("   已创建混合模式存储: %s\n", hybridPath)
	fmt.Printf("   目录中包含 %d 个文件\n", fileCount)
	fmt.Printf("   存储了 5 个小数据块和 3 个大数据块\n")

	fmt.Println("   混合模式特点:")
	fmt.Println("   - 小数据存储在主文件中")
	fmt.Println("   - 大数据存储在单独的文件中")
	fmt.Println("   - 结合了容器模式和目录模式的优点")
	fmt.Println("   - 适合混合大小的数据集")
	fmt.Println("   - 典型阈值大小: 10KB (示例值)")
}

// demoStorageModeConversion 演示存储模式转换
// 从容器模式转换为目录模式
func demoStorageModeConversion(sourceFile, targetDir string) {
	fmt.Println("\n5. 存储模式转换示例:")
	fmt.Println("   演示如何在不同存储模式之间转换")

	// 准备目录
	os.MkdirAll(filepath.Dir(targetDir), 0755)

	// 创建源数据文件(内存模式)
	fmt.Println("   5.1 创建源文件(内存模式)...")
	src, err := fragmenta.CreateFragmenta(sourceFile, &fragmenta.FragmentaOptions{
		StorageMode: FragDBMemoryMode,
	})
	if err != nil {
		fmt.Printf("   创建源文件失败: %v\n", err)
		return
	}

	// 写入一些数据
	src.SetMetadata(fragmenta.UserTag(0x1000), []byte("存储模式转换示例"))
	src.SetMetadata(fragmenta.UserTag(0x1001), []byte(time.Now().Format(time.RFC3339)))

	for i := 1; i <= 5; i++ {
		data := []byte(fmt.Sprintf("这是数据块 #%d，用于演示存储模式转换", i))
		_, err := src.WriteBlock(data, nil)
		if err != nil {
			fmt.Printf("   写入数据块失败: %v\n", err)
			src.Close()
			return
		}
	}

	err = src.Commit()
	if err != nil {
		fmt.Printf("   提交更改失败: %v\n", err)
		src.Close()
		return
	}

	// 获取并记录元数据标签，稍后验证
	titleData, err := src.GetMetadata(fragmenta.UserTag(0x1000))
	if err != nil {
		fmt.Printf("   读取元数据失败: %v\n", err)
		src.Close()
		return
	}
	titleStr := string(titleData)

	// 关闭源文件
	src.Close()

	// 打开源文件以便转换
	fmt.Println("   5.2 转换为容器模式...")
	dstContainerPath := filepath.Join(filepath.Dir(targetDir), "converted_container.frag")

	// 转换存储模式
	err = demoConvertStorage(sourceFile, dstContainerPath, FragDBContainerMode)
	if err != nil {
		fmt.Printf("   转换为容器模式失败: %v\n", err)
		return
	}

	// 打开转换后的容器模式文件
	containerFile, err := fragmenta.OpenFragmenta(dstContainerPath)
	if err != nil {
		fmt.Printf("   打开容器模式文件失败: %v\n", err)
		return
	}

	// 验证数据
	title, err := containerFile.GetMetadata(fragmenta.UserTag(0x1000))
	if err != nil {
		fmt.Printf("   读取元数据失败: %v\n", err)
		containerFile.Close()
		return
	}

	fmt.Printf("   验证容器模式转换: 标题=%s\n", string(title))
	containerFile.Close()

	// 将容器模式转换为目录模式
	fmt.Println("   5.3 转换为目录模式...")
	dstDirPath := filepath.Join(targetDir, "dir_mode")
	os.MkdirAll(dstDirPath, 0755)
	dstDirFile := filepath.Join(dstDirPath, "index.frag")

	// 进行转换
	err = demoConvertStorage(dstContainerPath, dstDirFile, FragDBDirectoryMode)
	if err != nil {
		fmt.Printf("   转换为目录模式失败: %v\n", err)
		return
	}

	// 打开转换后的目录模式存储
	dirFile, err := fragmenta.OpenFragmenta(dstDirFile)
	if err != nil {
		fmt.Printf("   打开目录模式存储失败: %v\n", err)
		return
	}

	// 验证数据
	title, err = dirFile.GetMetadata(fragmenta.UserTag(0x1000))
	if err != nil {
		fmt.Printf("   读取元数据失败: %v\n", err)
		dirFile.Close()
		return
	}

	fmt.Printf("   验证目录模式转换: 标题=%s\n", string(title))
	dirFile.Close()

	// 确认元数据匹配原始值
	if string(title) == titleStr {
		fmt.Println("   元数据完全匹配，转换成功")
	} else {
		fmt.Printf("   警告：元数据不匹配。原始：%s，转换后：%s\n", titleStr, string(title))
	}

	fmt.Println("   存储模式转换成功完成")
}

// demoConvertStorage 在不同存储模式之间转换
// 模拟实现，实际使用时应该使用fragmenta包提供的方法
func demoConvertStorage(sourcePath, targetPath string, targetMode uint8) error {
	// 打开源文件
	src, err := fragmenta.OpenFragmenta(sourcePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer src.Close()

	// 创建目标文件
	opts := &fragmenta.FragmentaOptions{
		StorageMode: targetMode,
	}

	dst, err := fragmenta.CreateFragmenta(targetPath, opts)
	if err != nil {
		return fmt.Errorf("创建目标存储失败: %w", err)
	}
	defer dst.Close()

	// 复制所有元数据
	// 注意：实际API可能提供更便捷的方法来复制所有元数据
	// 这里我们仅复制我们知道的键
	metadata := []uint16{
		fragmenta.UserTag(0x1000),
		fragmenta.UserTag(0x1001),
	}

	for _, tag := range metadata {
		value, err := src.GetMetadata(tag)
		if err != nil {
			continue
		}

		err = dst.SetMetadata(tag, value)
		if err != nil {
			return fmt.Errorf("复制元数据失败: %w", err)
		}
	}

	// 复制所有数据块
	// 实际API可能提供更便捷的方法或块迭代器
	for i := uint32(1); i <= 5; i++ {
		data, err := src.ReadBlock(i)
		if err != nil {
			continue
		}

		_, err = dst.WriteBlock(data, nil)
		if err != nil {
			return fmt.Errorf("复制数据块失败: %w", err)
		}
	}

	// 提交更改
	err = dst.Commit()
	if err != nil {
		return fmt.Errorf("提交更改失败: %w", err)
	}

	return nil
}
