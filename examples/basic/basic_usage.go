// package example 提供FragDB存储引擎的基础使用示例
package example

import (
	"fmt"
	"os"
	"time"

	"github.com/bpfs/fragmenta"
)

// 临时定义一些标签常量，实际应由fragmenta包提供
const (
	TagFileName     uint16 = 0x0001
	TagCreationTime uint16 = 0x0002
	TagFileType     uint16 = 0x0003
)

// BasicUsage 演示FragDB的基本使用方法
func BasicUsage() {
	fmt.Println("=== FragDB 基本使用示例 ===")
	fmt.Println("本示例展示FragDB存储引擎的基本操作")

	// 创建示例文件
	testPath := "basic_example.frag"
	os.Remove(testPath)

	// 1. 创建FragDB文件
	fmt.Println("\n1. 创建FragDB文件...")
	db, err := fragmenta.CreateFragmenta(testPath, &fragmenta.FragmentaOptions{
		StorageMode: 0, // ContainerMode
	})
	if err != nil {
		fmt.Printf("创建文件失败: %v\n", err)
		return
	}
	defer func() {
		db.Close()
		os.Remove(testPath)
	}()

	// 2. 设置元数据
	fmt.Println("2. 设置文件元数据...")
	// 使用内置标签设置基本文件信息
	db.SetMetadata(TagFileName, []byte("示例文件"))
	db.SetMetadata(TagCreationTime, []byte(time.Now().Format(time.RFC3339)))
	db.SetMetadata(TagFileType, []byte("示例/基本使用"))

	// 使用自定义标签设置应用特定信息
	db.SetMetadata(fragmenta.UserTag(0x1000), []byte("这是一个FragDB基本使用示例"))
	db.SetMetadata(fragmenta.UserTag(0x1001), []byte("版本1.0"))
	db.SetMetadata(fragmenta.UserTag(0x1002), []byte("张三"))

	// 3. 写入数据块
	fmt.Println("3. 写入数据块...")
	blockData1 := []byte("这是第一个数据块的内容。FragDB支持存储任意二进制数据。")
	blockID1, err := db.WriteBlock(blockData1, nil)
	if err != nil {
		fmt.Printf("写入第一个数据块失败: %v\n", err)
		return
	}
	fmt.Printf("   数据块1写入成功，ID: %d\n", blockID1)

	// 写入第二个数据块
	blockData2 := []byte("这是第二个数据块的内容。每个数据块可以存储不同的信息。")
	blockID2, err := db.WriteBlock(blockData2, nil)
	if err != nil {
		fmt.Printf("写入第二个数据块失败: %v\n", err)
		return
	}
	fmt.Printf("   数据块2写入成功，ID: %d\n", blockID2)

	// 写入第三个数据块
	blockData3 := []byte("这是第三个数据块的内容。我们将把这个数据块链接到第一个数据块。")
	blockID3, err := db.WriteBlock(blockData3, nil)
	if err != nil {
		fmt.Printf("写入第三个数据块失败: %v\n", err)
		return
	}
	fmt.Printf("   数据块3写入成功，ID: %d\n", blockID3)

	// 4. 建立数据块之间的链接关系
	fmt.Println("4. 建立数据块链接...")
	// 模拟链接数据块(实际API可能提供专门的方法)
	// 这里使用自定义元数据来模拟链接
	linkKey := fmt.Sprintf("block_%d_links", blockID1)
	db.SetMetadata(fragmenta.UserTag(0x2000), []byte(linkKey))
	db.SetMetadata(fragmenta.UserTag(0x2001), fragmenta.EncodeInt64(int64(blockID3)))
	fmt.Printf("   数据块1 (#%d) 和数据块3 (#%d) 已链接(通过元数据模拟)\n", blockID1, blockID3)

	// 5. 提交更改
	fmt.Println("5. 提交更改...")
	err = db.Commit()
	if err != nil {
		fmt.Printf("提交更改失败: %v\n", err)
		return
	}
	fmt.Println("   更改已提交到磁盘")

	// 6. 获取文件信息
	fmt.Println("6. 获取文件信息...")
	fileName, _ := db.GetMetadata(TagFileName)
	creationTime, _ := db.GetMetadata(TagCreationTime)
	fileType, _ := db.GetMetadata(TagFileType)
	description, _ := db.GetMetadata(fragmenta.UserTag(0x1000))
	version, _ := db.GetMetadata(fragmenta.UserTag(0x1001))
	author, _ := db.GetMetadata(fragmenta.UserTag(0x1002))

	fmt.Printf("   文件名: %s\n", string(fileName))
	fmt.Printf("   创建时间: %s\n", string(creationTime))
	fmt.Printf("   文件类型: %s\n", string(fileType))
	fmt.Printf("   描述: %s\n", string(description))
	fmt.Printf("   版本: %s\n", string(version))
	fmt.Printf("   作者: %s\n", string(author))

	// 7. 读取数据块
	fmt.Println("7. 读取数据块...")
	// 读取第一个数据块
	data1, err := db.ReadBlock(blockID1)
	if err != nil {
		fmt.Printf("读取数据块1失败: %v\n", err)
		return
	}
	fmt.Printf("   数据块1内容: %s\n", string(data1))

	// 读取第二个数据块
	data2, err := db.ReadBlock(blockID2)
	if err != nil {
		fmt.Printf("读取数据块2失败: %v\n", err)
		return
	}
	fmt.Printf("   数据块2内容: %s\n", string(data2))

	// 8. 获取链接的数据块
	fmt.Println("8. 获取链接的数据块...")
	// 模拟获取链接数据块(通过读取我们之前设置的元数据)
	linkData, err := db.GetMetadata(fragmenta.UserTag(0x2001))
	if err != nil {
		fmt.Printf("获取链接数据块失败: %v\n", err)
		return
	}

	linkedBlockID := uint32(fragmenta.DecodeInt64(linkData))
	fmt.Printf("   数据块1链接到的数据块: %d\n", linkedBlockID)

	// 读取链接的数据块
	linkedData, err := db.ReadBlock(linkedBlockID)
	if err != nil {
		fmt.Printf("   读取链接数据块失败: %v\n", err)
	} else {
		fmt.Printf("   链接数据块(ID: %d)内容: %s\n", linkedBlockID, string(linkedData))
	}

	// 9. 关闭并重新打开文件
	fmt.Println("9. 关闭并重新打开文件...")
	err = db.Close()
	if err != nil {
		fmt.Printf("关闭文件失败: %v\n", err)
		return
	}
	fmt.Println("   文件已关闭")

	// 重新打开文件
	fmt.Println("\n7. 重新打开文件...")
	db2, err := fragmenta.OpenFragmenta(testPath)
	if err != nil {
		fmt.Printf("打开文件失败: %v\n", err)
		return
	}
	defer db2.Close()
	fmt.Println("   文件已重新打开")

	// 10. 列出所有元数据
	fmt.Println("10. 列出所有元数据...")
	// 注意：实际API可能提供更便捷的方法来列出所有元数据
	// 这里我们仅枚举一些已知的键
	metadataTags := []struct {
		tag  uint16
		name string
	}{
		{TagFileName, "文件名"},
		{TagCreationTime, "创建时间"},
		{TagFileType, "文件类型"},
		{fragmenta.UserTag(0x1000), "描述"},
		{fragmenta.UserTag(0x1001), "版本"},
		{fragmenta.UserTag(0x1002), "作者"},
	}

	for _, tag := range metadataTags {
		value, err := db2.GetMetadata(tag.tag)
		if err != nil {
			fmt.Printf("   %s: <获取失败: %v>\n", tag.name, err)
			continue
		}
		fmt.Printf("   %s: %s\n", tag.name, string(value))
	}

	fmt.Println("\n=== 基本使用示例完成 ===")
}
