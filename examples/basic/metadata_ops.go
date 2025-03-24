// package example 提供FragDB存储引擎元数据操作示例
package example

import (
	"fmt"
	"os"
	"time"

	"github.com/bpfs/fragmenta"
)

// MetadataOperations 演示FragDB元数据操作
func MetadataOperations() {
	fmt.Println("=== FragDB 元数据操作示例 ===")
	fmt.Println("本示例展示如何使用FragDB的元数据功能")

	// 创建临时文件
	testFile := "metadata_example.frag"
	os.Remove(testFile)

	// 1. 创建文件
	fmt.Println("\n1. 创建FragDB文件...")
	db, err := fragmenta.CreateFragmenta(testFile, &fragmenta.FragmentaOptions{
		StorageMode: 0, // ContainerMode
	})
	if err != nil {
		fmt.Printf("创建文件失败: %v\n", err)
		return
	}
	defer func() {
		db.Close()
		os.Remove(testFile)
	}()

	// 2. 设置基本元数据
	fmt.Println("2. 设置基本元数据...")

	// 设置内置元数据
	db.SetMetadata(0x0001, []byte("元数据操作示例"))                       // 文件名
	db.SetMetadata(0x0002, []byte(time.Now().Format(time.RFC3339))) // 创建时间
	db.SetMetadata(0x0003, []byte("示例/元数据"))                        // 文件类型

	// 3. 设置用户自定义元数据
	fmt.Println("3. 设置用户自定义元数据...")

	// 设置文本类型元数据
	db.SetMetadata(fragmenta.UserTag(0x1000), []byte("张三"))
	db.SetMetadata(fragmenta.UserTag(0x1001), []byte("北京市海淀区中关村"))
	db.SetMetadata(fragmenta.UserTag(0x1002), []byte("research@example.com"))

	// 设置数值类型元数据
	priority := int64(10)
	db.SetMetadata(fragmenta.UserTag(0x1003), fragmenta.EncodeInt64(priority))

	size := int64(1024 * 1024) // 1MB
	db.SetMetadata(fragmenta.UserTag(0x1004), fragmenta.EncodeInt64(size))

	rating := 4.5
	encodedRating := fragmenta.EncodeFloat64(rating)
	db.SetMetadata(fragmenta.UserTag(0x1005), encodedRating)

	// 设置日期类型元数据
	expiryDate := time.Now().Add(time.Hour * 24 * 30) // 30天后过期
	db.SetMetadata(fragmenta.UserTag(0x1006), []byte(expiryDate.Format(time.RFC3339)))

	// 设置复杂结构
	// 在实际应用中，可以使用JSON或其他序列化方法
	tagsList := []string{"研究", "科技", "示例"}
	tagsData := []byte(fmt.Sprintf("%v", tagsList))
	db.SetMetadata(fragmenta.UserTag(0x1007), tagsData)

	// 4. 提交更改
	fmt.Println("4. 提交更改...")
	err = db.Commit()
	if err != nil {
		fmt.Printf("提交更改失败: %v\n", err)
		return
	}

	// 5. 读取元数据
	fmt.Println("5. 读取元数据...")

	// 读取基本元数据
	fileName, _ := db.GetMetadata(0x0001)
	createTime, _ := db.GetMetadata(0x0002)
	fileType, _ := db.GetMetadata(0x0003)

	fmt.Printf("   文件名: %s\n", string(fileName))
	fmt.Printf("   创建时间: %s\n", string(createTime))
	fmt.Printf("   文件类型: %s\n", string(fileType))

	// 读取用户自定义元数据
	name, _ := db.GetMetadata(fragmenta.UserTag(0x1000))
	address, _ := db.GetMetadata(fragmenta.UserTag(0x1001))
	email, _ := db.GetMetadata(fragmenta.UserTag(0x1002))

	fmt.Printf("   姓名: %s\n", string(name))
	fmt.Printf("   地址: %s\n", string(address))
	fmt.Printf("   邮箱: %s\n", string(email))

	// 读取并解码数值类型元数据
	priorityData, _ := db.GetMetadata(fragmenta.UserTag(0x1003))
	decodedPriority := fragmenta.DecodeInt64(priorityData)

	sizeData, _ := db.GetMetadata(fragmenta.UserTag(0x1004))
	decodedSize := fragmenta.DecodeInt64(sizeData)

	ratingData, _ := db.GetMetadata(fragmenta.UserTag(0x1005))
	decodedRating := fragmenta.DecodeFloat64(ratingData)

	fmt.Printf("   优先级: %d\n", decodedPriority)
	fmt.Printf("   大小: %d 字节 (%.2f MB)\n", decodedSize, float64(decodedSize)/(1024*1024))
	fmt.Printf("   评分: %.1f 星\n", decodedRating)

	// 读取日期
	expiryDateData, _ := db.GetMetadata(fragmenta.UserTag(0x1006))
	fmt.Printf("   过期时间: %s\n", string(expiryDateData))

	// 读取标签列表
	tagsData, _ = db.GetMetadata(fragmenta.UserTag(0x1007))
	fmt.Printf("   标签: %s\n", string(tagsData))

	// 6. 更新元数据
	fmt.Println("6. 更新元数据...")
	db.SetMetadata(fragmenta.UserTag(0x1000), []byte("李四"))
	db.SetMetadata(fragmenta.UserTag(0x1003), fragmenta.EncodeInt64(20)) // 优先级提高

	err = db.Commit()
	if err != nil {
		fmt.Printf("提交更新失败: %v\n", err)
		return
	}

	// 读取更新后的值
	updatedName, _ := db.GetMetadata(fragmenta.UserTag(0x1000))
	updatedPriorityData, _ := db.GetMetadata(fragmenta.UserTag(0x1003))
	updatedPriority := fragmenta.DecodeInt64(updatedPriorityData)

	fmt.Printf("   更新后的姓名: %s\n", string(updatedName))
	fmt.Printf("   更新后的优先级: %d\n", updatedPriority)

	// 7. 元数据的批量操作
	fmt.Println("7. 元数据批量操作...")

	// 批量设置多个属性
	batchData := map[uint16][]byte{
		fragmenta.UserTag(0x2000): []byte("批量操作示例"),
		fragmenta.UserTag(0x2001): []byte("值1"),
		fragmenta.UserTag(0x2002): []byte("值2"),
		fragmenta.UserTag(0x2003): []byte("值3"),
	}

	for tag, value := range batchData {
		db.SetMetadata(tag, value)
	}

	db.Commit()

	// 批量获取
	fmt.Println("   批量读取结果:")
	for tag := range batchData {
		value, _ := db.GetMetadata(tag)
		fmt.Printf("   标签 0x%X: %s\n", tag, string(value))
	}

	// 8. 删除元数据
	fmt.Println("8. 删除元数据...")
	// 模拟删除操作(实际API可能提供专门的删除方法)
	db.SetMetadata(fragmenta.UserTag(0x2001), nil) // 有些实现可能通过设置nil值来删除
	db.Commit()

	// 检查是否删除成功
	value, err := db.GetMetadata(fragmenta.UserTag(0x2001))
	if err != nil || len(value) == 0 {
		fmt.Println("   标签已成功删除")
	} else {
		fmt.Printf("   标签仍然存在，值为: %s\n", string(value))
	}

	fmt.Println("\n=== 元数据操作示例完成 ===")
}
