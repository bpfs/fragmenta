// package example 提供FragDB查询功能的示例
package example

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bpfs/fragmenta"
)

// 临时定义一些查询相关常量，实际应由fragmenta包提供
const (
	TagEqual uint8 = iota
	TagRange
	TagExists
	TagNotExists
	FullTextMatch
)

// Query 是一个临时的查询结构体
// 在实际应用中，应使用fragmenta.Query
type Query struct {
	conditions []queryCondition
}

// queryCondition 表示查询条件
type queryCondition struct {
	tagOffset uint16
	opType    uint8
	value     interface{}
}

// AddCondition 添加查询条件
func (q *Query) AddCondition(tagOffset uint16, opType uint8, value interface{}) *Query {
	q.conditions = append(q.conditions, queryCondition{
		tagOffset: tagOffset,
		opType:    opType,
		value:     value,
	})
	return q
}

// Execute 执行查询
// 这里简单模拟查询执行，实际应由fragmenta包实现
func (q *Query) Execute(db fragmenta.Fragmenta) ([]uint16, error) {
	// 这里仅做示例使用，实际实现会复杂得多
	// 返回一些假数据，以避免修改太多其他代码
	documentIDs := []uint16{0x1000, 0x1001, 0x1002, 0x1003, 0x1004}

	// 根据条件筛选
	var results []uint16
	for _, docID := range documentIDs {
		match := true

		// 检查每个条件
		for _, cond := range q.conditions {
			// 根据条件类型进行匹配
			// 这里仅简单模拟
			switch cond.opType {
			case TagEqual:
				if !tagEqual(db, docID, cond.tagOffset, cond.value) {
					match = false
				}
			case TagExists:
				if !tagExists(db, docID, cond.tagOffset) {
					match = false
				}
			case TagRange:
				if !tagInRange(db, docID, cond.tagOffset, cond.value) {
					match = false
				}
			}

			if !match {
				break
			}
		}

		if match {
			results = append(results, docID)
		}
	}

	return results, nil
}

// tagEqual 检查标签值是否等于给定值
func tagEqual(db fragmenta.Fragmenta, docID, tagOffset uint16, value interface{}) bool {
	if value == nil {
		return false
	}

	expected, ok := value.([]byte)
	if !ok {
		return false
	}

	actual, err := db.GetMetadata(fragmenta.UserTag(docID + tagOffset))
	if err != nil {
		return false
	}

	return string(actual) == string(expected)
}

// tagExists 检查标签是否存在
func tagExists(db fragmenta.Fragmenta, docID, tagOffset uint16) bool {
	_, err := db.GetMetadata(fragmenta.UserTag(docID + tagOffset))
	return err == nil
}

// tagInRange 检查标签值是否在范围内
func tagInRange(db fragmenta.Fragmenta, docID, tagOffset uint16, value interface{}) bool {
	ranges, ok := value.([][]byte)
	if !ok || len(ranges) != 2 {
		return false
	}

	actual, err := db.GetMetadata(fragmenta.UserTag(docID + tagOffset))
	if err != nil {
		return false
	}

	// 字符串比较
	if isStringValue(actual) {
		min, max := string(ranges[0]), string(ranges[1])
		val := string(actual)
		return val >= min && val <= max
	}

	// 数值比较
	if len(actual) == 8 { // int64
		val := fragmenta.DecodeInt64(actual)
		min := fragmenta.DecodeInt64(ranges[0])
		max := fragmenta.DecodeInt64(ranges[1])
		return val >= min && val <= max
	}

	return false
}

// isStringValue 判断数据是否为字符串值
func isStringValue(data []byte) bool {
	// 简单检查是否为UTF-8文本
	return len(data) > 0 && data[0] < 128
}

// AdvancedQueryExample 演示FragDB的高级查询功能
func AdvancedQueryExample() {
	fmt.Println("=== FragDB 高级查询示例 ===")
	fmt.Println("本示例演示FragDB的强大查询能力")

	// 创建测试文件
	queryFile := "query_example.frag"
	os.Remove(queryFile)

	// 创建存储实例
	db, err := fragmenta.CreateFragmenta(queryFile, &fragmenta.FragmentaOptions{
		StorageMode: 0, // ContainerMode
	})
	if err != nil {
		fmt.Printf("创建查询测试文件失败: %v\n", err)
		return
	}
	defer func() {
		db.Close()
		os.Remove(queryFile)
	}()

	// 创建测试文档集合
	createTestDocuments(db)

	// 执行各种查询示例
	fmt.Println("\n--- 查询示例 ---")

	// 1. 简单标签值匹配查询
	doSimpleTagQuery(db)

	// 2. 范围查询
	doRangeQuery(db)

	// 3. 组合查询
	doCompoundQuery(db)

	// 4. 全文搜索
	doFullTextSearch(db)

	// 5. 分页和排序
	doPaginationAndSorting(db)

	fmt.Println("\n=== 查询示例完成 ===")
}

// 创建一组测试文档用于演示查询功能
func createTestDocuments(db fragmenta.Fragmenta) {
	fmt.Println("\n创建测试文档集合...")

	// 创建不同类型的文档和元数据
	docs := []struct {
		title     string
		content   string
		docType   string
		important bool
		size      int
		created   time.Time
		tags      []string
	}{
		{
			title:     "项目计划",
			content:   "这是一个重要的项目计划文档，包含项目的所有关键信息。",
			docType:   "文档",
			important: true,
			size:      1024 * 15,
			created:   time.Now().Add(-time.Hour * 24 * 10),
			tags:      []string{"项目", "规划", "重要"},
		},
		{
			title:     "会议记录",
			content:   "上周的团队会议记录，讨论了项目进展和问题。",
			docType:   "文档",
			important: false,
			size:      1024 * 5,
			created:   time.Now().Add(-time.Hour * 24 * 5),
			tags:      []string{"会议", "记录"},
		},
		{
			title:     "产品Logo",
			content:   "公司产品的Logo图片文件。",
			docType:   "图片",
			important: true,
			size:      1024 * 150,
			created:   time.Now().Add(-time.Hour * 24 * 30),
			tags:      []string{"设计", "品牌", "重要"},
		},
		{
			title:     "用户反馈截图",
			content:   "用户界面使用反馈的截图。",
			docType:   "图片",
			important: false,
			size:      1024 * 80,
			created:   time.Now().Add(-time.Hour * 24 * 2),
			tags:      []string{"用户", "反馈", "界面"},
		},
		{
			title:     "产品演示视频",
			content:   "最新产品功能的演示视频。",
			docType:   "视频",
			important: true,
			size:      1024 * 1024 * 50,
			created:   time.Now().Add(-time.Hour * 24),
			tags:      []string{"产品", "演示", "营销", "重要"},
		},
	}

	// 将文档写入存储
	for i, doc := range docs {
		// 写入文档内容
		blockID, err := db.WriteBlock([]byte(doc.content), nil)
		if err != nil {
			fmt.Printf("写入文档内容失败: %v\n", err)
			continue
		}

		// 分配一个文档ID
		docID := uint16(0x1000 + i)

		// 写入文档元数据
		db.SetMetadata(fragmenta.UserTag(docID), fragmenta.EncodeInt64(int64(blockID)))
		db.SetMetadata(fragmenta.UserTag(docID+0x100), []byte(doc.title))
		db.SetMetadata(fragmenta.UserTag(docID+0x200), []byte(doc.docType))

		// 重要性标记
		if doc.important {
			db.SetMetadata(fragmenta.UserTag(docID+0x300), []byte("重要"))
		}

		// 文件大小
		db.SetMetadata(fragmenta.UserTag(docID+0x400), fragmenta.EncodeInt64(int64(doc.size)))

		// 创建时间
		db.SetMetadata(fragmenta.UserTag(docID+0x500), []byte(doc.created.Format(time.RFC3339)))

		// 标签
		for j, tag := range doc.tags {
			db.SetMetadata(fragmenta.UserTag(docID+0x600+uint16(j)), []byte(tag))
		}

		fmt.Printf("已创建文档: %s (ID: 0x%X)\n", doc.title, docID)
	}

	// 提交更改
	err := db.Commit()
	if err != nil {
		fmt.Printf("提交文档失败: %v\n", err)
		return
	}

	fmt.Println("测试文档创建完成")
}

// 创建新查询实例的辅助函数
func newQuery() *Query {
	// 实际应使用fragmenta.NewQuery()
	// 这里仅为示例使用
	return &Query{}
}

// doSimpleTagQuery 演示简单的标签值匹配查询
func doSimpleTagQuery(db fragmenta.Fragmenta) {
	fmt.Println("\n1. 简单标签值匹配查询:")

	// 查询文档类型为"图片"的文档
	fmt.Println("   a) 查找所有图片类型的文档:")

	// 创建查询
	imageQuery := newQuery()
	imageQuery.AddCondition(0x200, TagEqual, []byte("图片"))

	// 执行查询
	results, err := imageQuery.Execute(db)
	if err != nil {
		fmt.Printf("   查询失败: %v\n", err)
		return
	}

	fmt.Printf("   找到 %d 个图片文档:\n", len(results))
	for _, docID := range results {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		fmt.Printf("   - %s (ID: 0x%X)\n", string(title), docID)
	}

	// 查询带有"重要"标签的文档
	fmt.Println("\n   b) 查找所有标记为重要的文档:")

	// 创建查询
	importantQuery := newQuery()
	importantQuery.AddCondition(0x300, TagExists, nil)

	// 执行查询
	results, err = importantQuery.Execute(db)
	if err != nil {
		fmt.Printf("   查询失败: %v\n", err)
		return
	}

	fmt.Printf("   找到 %d 个重要文档:\n", len(results))
	for _, docID := range results {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		docType, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x200))
		fmt.Printf("   - %s (%s, ID: 0x%X)\n", string(title), string(docType), docID)
	}
}

// doRangeQuery 演示范围查询功能
func doRangeQuery(db fragmenta.Fragmenta) {
	fmt.Println("\n2. 范围查询:")

	// 查询大小在10KB到100KB之间的文档
	fmt.Println("   a) 查找大小在10KB到100KB之间的文档:")

	// 创建查询
	sizeQuery := newQuery()
	sizeQuery.AddCondition(0x400, TagRange, [][]byte{
		fragmenta.EncodeInt64(int64(10 * 1024)),  // 最小值
		fragmenta.EncodeInt64(int64(100 * 1024)), // 最大值
	})

	// 执行查询
	results, err := sizeQuery.Execute(db)
	if err != nil {
		fmt.Printf("   查询失败: %v\n", err)
		return
	}

	fmt.Printf("   找到 %d 个符合大小条件的文档:\n", len(results))
	for _, docID := range results {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		sizeData, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x400))
		size := fragmenta.DecodeInt64(sizeData)
		fmt.Printf("   - %s (大小: %.2f KB, ID: 0x%X)\n",
			string(title), float64(size)/1024, docID)
	}

	// 查询一周内创建的文档
	fmt.Println("\n   b) 查找最近一周内创建的文档:")

	// 计算一周前的时间
	oneWeekAgo := time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339)

	// 创建查询
	timeQuery := newQuery()
	timeQuery.AddCondition(0x500, TagRange, [][]byte{
		[]byte(oneWeekAgo),                      // 最小值(一周前)
		[]byte(time.Now().Format(time.RFC3339)), // 最大值(现在)
	})

	// 执行查询
	results, err = timeQuery.Execute(db)
	if err != nil {
		fmt.Printf("   查询失败: %v\n", err)
		return
	}

	fmt.Printf("   找到 %d 个最近一周创建的文档:\n", len(results))
	for _, docID := range results {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		dateData, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x500))
		fmt.Printf("   - %s (创建时间: %s, ID: 0x%X)\n",
			string(title), string(dateData), docID)
	}
}

// doCompoundQuery 演示组合查询功能
func doCompoundQuery(db fragmenta.Fragmenta) {
	fmt.Println("\n3. 组合查询:")

	// 查询重要的图片文档
	fmt.Println("   a) 查找所有重要的图片文档:")

	// 创建查询
	compoundQuery := newQuery()
	compoundQuery.AddCondition(0x200, TagEqual, []byte("图片"))
	compoundQuery.AddCondition(0x300, TagExists, nil)

	// 执行查询
	results, err := compoundQuery.Execute(db)
	if err != nil {
		fmt.Printf("   查询失败: %v\n", err)
		return
	}

	fmt.Printf("   找到 %d 个重要的图片文档:\n", len(results))
	for _, docID := range results {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		fmt.Printf("   - %s (ID: 0x%X)\n", string(title), docID)
	}

	// 查询大文件但非视频类型的文档
	fmt.Println("\n   b) 查找大于50KB但不是视频类型的文档:")

	// 创建大文件查询
	largeFileQuery := newQuery()
	largeFileQuery.AddCondition(0x400, TagRange, [][]byte{
		fragmenta.EncodeInt64(int64(50 * 1024)),        // 最小值
		fragmenta.EncodeInt64(int64(10 * 1024 * 1024)), // 最大值
	})

	// 执行查询获取大文件
	largeFiles, err := largeFileQuery.Execute(db)
	if err != nil {
		fmt.Printf("   查询失败: %v\n", err)
		return
	}

	// 创建视频文件查询
	videoQuery := newQuery()
	videoQuery.AddCondition(0x200, TagEqual, []byte("视频"))

	// 执行查询获取视频文件
	videos, err := videoQuery.Execute(db)
	if err != nil {
		fmt.Printf("   查询失败: %v\n", err)
		return
	}

	// 排除视频文件
	var results2 []uint16
	for _, id := range largeFiles {
		isVideo := false
		for _, videoID := range videos {
			if id == videoID {
				isVideo = true
				break
			}
		}
		if !isVideo {
			results2 = append(results2, id)
		}
	}

	fmt.Printf("   找到 %d 个大于50KB但不是视频的文档:\n", len(results2))
	for _, docID := range results2 {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		docType, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x200))
		sizeData, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x400))
		size := fragmenta.DecodeInt64(sizeData)
		fmt.Printf("   - %s (%s, 大小: %.2f KB, ID: 0x%X)\n",
			string(title), string(docType), float64(size)/1024, docID)
	}
}

// doFullTextSearch 演示全文搜索功能
func doFullTextSearch(db fragmenta.Fragmenta) {
	fmt.Println("\n4. 全文搜索:")

	// 搜索标题或内容中包含特定关键字的文档
	fmt.Println("   a) 搜索内容中包含'项目'关键字的文档:")

	// 获取所有文档ID
	allDocs := []uint16{0x1000, 0x1001, 0x1002, 0x1003, 0x1004}
	var results []uint16

	// 读取每个文档内容进行搜索
	for _, docID := range allDocs {
		// 获取内容块ID
		blockIDData, err := db.GetMetadata(fragmenta.UserTag(docID))
		if err != nil {
			continue
		}
		blockID := uint32(fragmenta.DecodeInt64(blockIDData))

		// 读取内容
		content, err := db.ReadBlock(blockID)
		if err != nil {
			continue
		}

		// 检查内容中是否包含关键字
		if containsKeyword(string(content), "项目") {
			results = append(results, docID)
		}
	}

	fmt.Printf("   找到 %d 个包含'项目'关键字的文档:\n", len(results))
	for _, docID := range results {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		fmt.Printf("   - %s (ID: 0x%X)\n", string(title), docID)
	}
}

// 检查文本是否包含关键字的辅助函数
func containsKeyword(text, keyword string) bool {
	return strings.Contains(text, keyword)
}

// doPaginationAndSorting 演示分页和排序功能
func doPaginationAndSorting(db fragmenta.Fragmenta) {
	fmt.Println("\n5. 分页和排序:")

	// 按文件大小排序并分页
	fmt.Println("   a) 按文件大小排序所有文档(降序):")

	// 获取所有文档
	allDocs := []uint16{0x1000, 0x1001, 0x1002, 0x1003, 0x1004}

	// 收集大小信息
	type docInfo struct {
		id    uint16
		title string
		size  int64
	}

	var docsInfo []docInfo
	for _, docID := range allDocs {
		title, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x100))
		sizeData, _ := db.GetMetadata(fragmenta.UserTag(docID + 0x400))
		size := fragmenta.DecodeInt64(sizeData)

		docsInfo = append(docsInfo, docInfo{
			id:    docID,
			title: string(title),
			size:  size,
		})
	}

	// 按大小排序(降序)
	for i := 0; i < len(docsInfo); i++ {
		for j := i + 1; j < len(docsInfo); j++ {
			if docsInfo[i].size < docsInfo[j].size {
				docsInfo[i], docsInfo[j] = docsInfo[j], docsInfo[i]
			}
		}
	}

	// 显示排序结果
	fmt.Println("   按大小排序的文档列表:")
	for i, doc := range docsInfo {
		fmt.Printf("   %d. %s (大小: %.2f KB, ID: 0x%X)\n",
			i+1, doc.title, float64(doc.size)/1024, doc.id)
	}

	// 分页展示(每页2个文档)
	pageSize := 2
	pageCount := (len(docsInfo) + pageSize - 1) / pageSize

	fmt.Printf("\n   b) 分页展示 (共%d页，每页%d个文档):\n", pageCount, pageSize)

	for page := 0; page < pageCount; page++ {
		start := page * pageSize
		end := start + pageSize
		if end > len(docsInfo) {
			end = len(docsInfo)
		}

		fmt.Printf("\n   第%d页:\n", page+1)
		for i := start; i < end; i++ {
			doc := docsInfo[i]
			fmt.Printf("   %d. %s (大小: %.2f KB, ID: 0x%X)\n",
				i+1, doc.title, float64(doc.size)/1024, doc.id)
		}
	}
}
