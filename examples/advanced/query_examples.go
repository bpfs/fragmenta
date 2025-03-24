package example

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bpfs/fragmenta"
)

// QueryExamples 展示FragDB格式的查询功能
func QueryExamples() {
	fmt.Println("=== FragDB查询功能示例 ===")

	// 创建临时文件路径
	tempPath := "query_example.frag"
	defer os.Remove(tempPath)

	// 1. 创建示例文件并填充数据
	fmt.Println("1. 创建示例文件并填充数据...")
	f, err := prepareQueryExampleFile(tempPath)
	if err != nil {
		log.Fatalf("准备查询示例文件失败: %v", err)
	}

	// 2. 基本标签查询
	fmt.Println("2. 基本标签查询...")
	results, err := f.QueryByTag(fragmenta.UserTag(0x1001), []byte("文档"))
	if err != nil {
		log.Fatalf("基本标签查询失败: %v", err)
	}
	fmt.Printf("   按标签 0x%04X 查询到 %d 个结果\n", fragmenta.UserTag(0x1001), len(results))

	// 3. 单条件元数据查询（相等条件）
	fmt.Println("3. 单条件元数据查询（相等条件）...")
	singleConditionEqualsQuery(f)

	// 4. 单条件元数据查询（范围条件）
	fmt.Println("4. 单条件元数据查询（范围条件）...")
	singleConditionRangeQuery(f)

	// 5. 多条件元数据查询（AND逻辑）
	fmt.Println("5. 多条件元数据查询（AND逻辑）...")
	multiConditionAndQuery(f)

	// 6. 多条件元数据查询（OR逻辑）
	fmt.Println("6. 多条件元数据查询（OR逻辑）...")
	multiConditionOrQuery(f)

	// 7. 根据内容类型查询
	fmt.Println("7. 根据内容类型查询...")
	contentTypeQuery(f)

	// 8. 排序和分页查询
	fmt.Println("8. 排序和分页查询...")
	sortAndPaginationQuery(f)

	// 9. 关闭文件
	fmt.Println("9. 关闭文件...")
	err = f.Close()
	if err != nil {
		log.Fatalf("关闭文件失败: %v", err)
	}

	fmt.Println("=== 查询功能示例完成 ===")
}

// prepareQueryExampleFile 准备用于查询示例的数据文件
func prepareQueryExampleFile(path string) (fragmenta.Fragmenta, error) {
	f, err := fragmenta.CreateFragmenta(path, nil)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %v", err)
	}

	// 添加一些文档文件
	documentTypes := []string{"文档", "表格", "演示文稿", "报告", "合同"}
	fileExtensions := []string{".docx", ".xlsx", ".pptx", ".pdf", ".txt"}

	// 为不同类型的文档添加元数据和内容
	for i := 0; i < 20; i++ {
		typeIndex := i % len(documentTypes)
		fileType := documentTypes[typeIndex]
		extension := fileExtensions[typeIndex]

		// 创建文件名
		fileName := fmt.Sprintf("示例%s_%02d%s", fileType, i, extension)

		// 设置基本元数据
		f.SetMetadata(fragmenta.UserTag(0x1001), []byte(fileType))
		f.SetMetadata(fragmenta.UserTag(0x1002), []byte(fileName))

		// 设置文件大小（随机值 1KB-10MB）
		fileSize := 1024 * (1 + (i*517)%10240) // 1KB - 10MB
		f.SetMetadata(fragmenta.UserTag(0x1003), fragmenta.EncodeInt64(int64(fileSize)))

		// 设置创建时间（过去30天内的随机时间）
		daysAgo := i % 30
		creationTime := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour)
		f.SetMetadata(fragmenta.UserTag(0x1004), fragmenta.EncodeInt64(creationTime.UnixNano()))

		// 设置优先级（1-5）
		priority := 1 + i%5
		f.SetMetadata(fragmenta.UserTag(0x1005), fragmenta.EncodeInt64(int64(priority)))

		// 设置作者
		authors := []string{"张三", "李四", "王五", "赵六", "钱七"}
		author := authors[i%len(authors)]
		f.SetMetadata(fragmenta.UserTag(0x1006), []byte(author))

		// 设置内容类型
		var contentType string
		switch typeIndex {
		case 0:
			contentType = "application/vnd.openxmlfragmentas-officedocument.wordprocessingml.document"
		case 1:
			contentType = "application/vnd.openxmlfragmentas-officedocument.spreadsheetml.sheet"
		case 2:
			contentType = "application/vnd.openxmlfragmentas-officedocument.presentationml.presentation"
		case 3:
			contentType = "application/pdf"
		default:
			contentType = "text/plain"
		}
		f.SetMetadata(fragmenta.TagContentType, []byte(contentType))

		// 写入示例数据块
		content := []byte(fmt.Sprintf("这是%s的内容，文件名为%s", fileType, fileName))
		_, err := f.WriteBlock(content, nil)
		if err != nil {
			return nil, fmt.Errorf("写入数据块失败: %v", err)
		}
	}

	// 提交更改
	err = f.Commit()
	if err != nil {
		return nil, fmt.Errorf("提交更改失败: %v", err)
	}

	return f, nil
}

// singleConditionEqualsQuery 单条件相等查询
func singleConditionEqualsQuery(f fragmenta.Fragmenta) {
	query := &fragmenta.MetadataQuery{
		Conditions: []fragmenta.MetadataCondition{
			{
				Tag:      fragmenta.UserTag(0x1001),
				Operator: fragmenta.OpEquals,
				Value:    []byte("文档"),
			},
		},
		Operator: fragmenta.LogicAnd,
	}

	result, err := f.QueryMetadata(query)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("   查询到与\"文档\"相等的记录: %d 条\n", result.ReturnCount)
	if result.ReturnCount > 0 {
		fmt.Println("   前3条记录：")
		displayCount := min(3, int(result.ReturnCount))
		for i := 0; i < displayCount; i++ {
			entry := result.Entries[i]
			fmt.Printf("      记录 #%d: MetadataID=0x%04X\n", i+1, entry.MetadataID)
		}
	}
}

// singleConditionRangeQuery 单条件范围查询
func singleConditionRangeQuery(f fragmenta.Fragmenta) {
	query := &fragmenta.MetadataQuery{
		Conditions: []fragmenta.MetadataCondition{
			{
				Tag:      fragmenta.UserTag(0x1003), // 文件大小
				Operator: fragmenta.OpGreaterThan,
				Value:    fragmenta.EncodeInt64(1024 * 1024), // 大于1MB
			},
		},
		Operator: fragmenta.LogicAnd,
	}

	result, err := f.QueryMetadata(query)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("   查询到文件大小大于1MB的记录: %d 条\n", result.ReturnCount)
}

// multiConditionAndQuery 多条件AND查询
func multiConditionAndQuery(f fragmenta.Fragmenta) {
	query := &fragmenta.MetadataQuery{
		Conditions: []fragmenta.MetadataCondition{
			{
				Tag:      fragmenta.UserTag(0x1001), // 文档类型
				Operator: fragmenta.OpEquals,
				Value:    []byte("报告"),
			},
			{
				Tag:      fragmenta.UserTag(0x1005), // 优先级
				Operator: fragmenta.OpGreaterThan,
				Value:    fragmenta.EncodeInt64(3), // 优先级大于3
			},
		},
		Operator: fragmenta.LogicAnd,
	}

	result, err := f.QueryMetadata(query)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("   查询到类型为\"报告\"且优先级大于3的记录: %d 条\n", result.ReturnCount)
}

// multiConditionOrQuery 多条件OR查询
func multiConditionOrQuery(f fragmenta.Fragmenta) {
	query := &fragmenta.MetadataQuery{
		Conditions: []fragmenta.MetadataCondition{
			{
				Tag:      fragmenta.UserTag(0x1001), // 文档类型
				Operator: fragmenta.OpEquals,
				Value:    []byte("表格"),
			},
			{
				Tag:      fragmenta.UserTag(0x1001), // 文档类型
				Operator: fragmenta.OpEquals,
				Value:    []byte("演示文稿"),
			},
		},
		Operator: fragmenta.LogicOr,
	}

	result, err := f.QueryMetadata(query)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("   查询到类型为\"表格\"或\"演示文稿\"的记录: %d 条\n", result.ReturnCount)
}

// contentTypeQuery 内容类型查询
func contentTypeQuery(f fragmenta.Fragmenta) {
	query := &fragmenta.MetadataQuery{
		Conditions: []fragmenta.MetadataCondition{
			{
				Tag:      fragmenta.TagContentType,
				Operator: fragmenta.OpContains,
				Value:    []byte("pdf"),
			},
		},
		Operator: fragmenta.LogicAnd,
	}

	result, err := f.QueryMetadata(query)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("   查询到内容类型包含\"pdf\"的记录: %d 条\n", result.ReturnCount)
}

// sortAndPaginationQuery 排序和分页查询
func sortAndPaginationQuery(f fragmenta.Fragmenta) {
	// 按创建时间降序，并分页
	query := &fragmenta.MetadataQuery{
		Conditions: []fragmenta.MetadataCondition{
			{
				Tag:      fragmenta.UserTag(0x1004), // 创建时间
				Operator: fragmenta.OpGreaterThan,
				Value:    fragmenta.EncodeInt64(0),
			},
		},
		Operator:  fragmenta.LogicAnd,
		SortBy:    fragmenta.UserTag(0x1004), // 按创建时间排序
		SortOrder: fragmenta.SortDescending,  // 降序
		Limit:     5,                         // 每页5条
		Offset:    0,                         // 第一页
	}

	result, err := f.QueryMetadata(query)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("   按创建时间降序排序，第一页（5条）: %d 条\n", result.ReturnCount)

	// 获取第二页
	query.Offset = 5 // 第二页

	result, err = f.QueryMetadata(query)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("   按创建时间降序排序，第二页（5条）: %d 条\n", result.ReturnCount)
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
