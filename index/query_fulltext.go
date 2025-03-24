// package index 提供全文搜索查询集成功能
package index

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// 全文搜索的正则表达式模式
var (
	// 全文搜索模式：text:"some search terms"
	fulltextPattern = regexp.MustCompile(`text:"([^"]+)"`)

	// 全文搜索简化模式：text:some_term
	fulltextSimplePattern = regexp.MustCompile(`text:([^\s;]+)`)
)

// FullTextQueryExecutor 全文搜索查询执行器
type FullTextQueryExecutor struct {
	// 嵌入默认查询执行器
	*DefaultQueryExecutor

	// 全文索引
	fullTextIndex FullTextIndex
}

// NewFullTextQueryExecutor 创建全文搜索查询执行器
func NewFullTextQueryExecutor(indexManager IndexManager, fullTextIndex FullTextIndex) *FullTextQueryExecutor {
	return &FullTextQueryExecutor{
		DefaultQueryExecutor: &DefaultQueryExecutor{
			indexManager:     indexManager,
			metadataProvider: nil,
		},
		fullTextIndex: fullTextIndex,
	}
}

// NewFullTextQueryExecutorWithMetadataProvider 创建带元数据提供器的全文搜索查询执行器
func NewFullTextQueryExecutorWithMetadataProvider(indexManager IndexManager, metadataProvider MetadataProvider, fullTextIndex FullTextIndex) *FullTextQueryExecutor {
	return &FullTextQueryExecutor{
		DefaultQueryExecutor: &DefaultQueryExecutor{
			indexManager:     indexManager,
			metadataProvider: metadataProvider,
		},
		fullTextIndex: fullTextIndex,
	}
}

// ParseQueryString 解析查询字符串，支持全文搜索
func (qe *FullTextQueryExecutor) ParseQueryString(queryStr string) (*Query, error) {
	// 检查是否包含全文搜索语法
	hasFullTextSearch, fullTextTerms := qe.extractFullTextTerms(queryStr)

	if !hasFullTextSearch {
		// 如果不包含全文搜索语法，则使用默认解析器
		return qe.DefaultQueryExecutor.ParseQueryString(queryStr)
	}

	// 处理全文搜索查询
	// 从原始查询中移除全文搜索部分
	queryStr = qe.removeFullTextParts(queryStr)

	// 解析剩余的查询条件
	query, err := qe.DefaultQueryExecutor.ParseQueryString(queryStr)
	if err != nil && queryStr != "" {
		return nil, err
	}

	// 如果没有有效的查询（仅有全文搜索），则创建一个空查询
	if query == nil {
		query = &Query{
			Limit:          10,
			Offset:         0,
			IncludeDeleted: false,
		}
	}

	// 添加全文搜索条件
	fullTextCondition := &QueryCondition{
		Field:     "fulltext",
		FieldType: "text",
		Operator:  "fulltext",
		Value:     strings.Join(fullTextTerms, " "),
	}

	// 合并全文搜索条件和其他条件
	if query.RootCondition == nil {
		query.RootCondition = fullTextCondition
	} else {
		// 将全文搜索条件与现有条件通过AND操作符组合
		query.RootCondition = &QueryCondition{
			Operator: "and",
			Children: []*QueryCondition{fullTextCondition, query.RootCondition},
		}
	}

	return query, nil
}

// Execute 执行查询，支持全文搜索
func (qe *FullTextQueryExecutor) Execute(query *Query) (*QueryResult, error) {
	// 检查是否为全文搜索查询
	hasFullTextSearch := qe.hasFullTextCondition(query.RootCondition)

	if !hasFullTextSearch {
		// 如果不是全文搜索查询，则使用默认执行器
		return qe.DefaultQueryExecutor.Execute(query)
	}

	// 执行全文搜索
	startTime := time.Now()

	// 提取全文搜索条件
	fullTextCondition, otherCondition := qe.splitFullTextCondition(query.RootCondition)

	// 获取全文搜索内容
	searchText := ""
	if fullTextCondition != nil {
		searchText, _ = fullTextCondition.Value.(string)
	}

	// 构造搜索选项
	options := &SearchOptions{
		Limit:              query.Limit,
		Offset:             query.Offset,
		RelevanceThreshold: 0.1, // 默认相关性阈值
		Highlight:          false,
		FuzzyMatch:         false,
		SortBy:             "relevance",
		Ascending:          false,
	}

	// 执行全文搜索
	searchResult, err := qe.fullTextIndex.Search(searchText, options)
	if err != nil {
		return nil, err
	}

	// 如果没有其他条件，直接返回全文搜索结果
	if otherCondition == nil {
		// 将全文搜索结果转换为查询结果
		result := &QueryResult{
			IDs:           searchResult.SortedIDs,
			TotalCount:    searchResult.TotalMatches,
			ExecutionTime: time.Since(startTime),
		}
		return result, nil
	}

	// 如果有其他条件，需要进行结果过滤
	// 构造一个仅包含其他条件的查询
	filteredQuery := &Query{
		RootCondition:  otherCondition,
		SortBy:         query.SortBy,
		Limit:          query.Limit,
		Offset:         query.Offset,
		IncludeDeleted: query.IncludeDeleted,
	}

	// 执行标准查询
	standardResult, err := qe.DefaultQueryExecutor.Execute(filteredQuery)
	if err != nil {
		return nil, err
	}

	// 获取全文搜索结果ID集合
	fulltextIDSet := make(map[uint32]bool)
	for _, id := range searchResult.SortedIDs {
		fulltextIDSet[id] = true
	}

	// 获取标准查询结果ID集合
	standardIDSet := make(map[uint32]bool)
	for _, id := range standardResult.IDs {
		standardIDSet[id] = true
	}

	// 计算两个结果的交集
	intersectIDs := make([]uint32, 0)
	for id := range fulltextIDSet {
		if standardIDSet[id] {
			intersectIDs = append(intersectIDs, id)
		}
	}

	// 对交集结果进行排序（使用全文搜索的相关性排序）
	// 创建得分映射
	scores := make(map[uint32]float64)
	for id, score := range searchResult.Matches {
		if standardIDSet[id] {
			scores[id] = score
		}
	}

	// 按照相关性进行排序
	sortDocIDsByScoreDesc(intersectIDs, scores)

	// 构造最终结果
	result := &QueryResult{
		IDs:           intersectIDs,
		TotalCount:    len(intersectIDs),
		ExecutionTime: time.Since(startTime),
	}

	return result, nil
}

// extractFullTextTerms 从查询字符串中提取全文搜索词项
func (qe *FullTextQueryExecutor) extractFullTextTerms(queryStr string) (bool, []string) {
	terms := make([]string, 0)

	// 查找全文搜索模式："text:"some search terms""
	matches := fulltextPattern.FindAllStringSubmatch(queryStr, -1)
	for _, match := range matches {
		if len(match) > 1 {
			terms = append(terms, match[1])
		}
	}

	// 查找简化全文搜索模式："text:some_term"
	matches = fulltextSimplePattern.FindAllStringSubmatch(queryStr, -1)
	for _, match := range matches {
		if len(match) > 1 {
			terms = append(terms, match[1])
		}
	}

	return len(terms) > 0, terms
}

// removeFullTextParts 从查询字符串中移除全文搜索部分
func (qe *FullTextQueryExecutor) removeFullTextParts(queryStr string) string {
	// 移除 text:"..." 模式
	queryStr = fulltextPattern.ReplaceAllString(queryStr, "")

	// 移除 text:... 模式
	queryStr = fulltextSimplePattern.ReplaceAllString(queryStr, "")

	// 清理多余的空白和逻辑操作符
	queryStr = strings.TrimSpace(queryStr)
	queryStr = regexp.MustCompile(`\s+and\s+$`).ReplaceAllString(queryStr, "")
	queryStr = regexp.MustCompile(`^\s*and\s+`).ReplaceAllString(queryStr, "")
	queryStr = regexp.MustCompile(`\s+or\s+$`).ReplaceAllString(queryStr, "")
	queryStr = regexp.MustCompile(`^\s*or\s+`).ReplaceAllString(queryStr, "")

	return queryStr
}

// hasFullTextCondition 检查查询条件是否包含全文搜索条件
func (qe *FullTextQueryExecutor) hasFullTextCondition(condition *QueryCondition) bool {
	if condition == nil {
		return false
	}

	// 检查当前条件是否为全文搜索条件
	if condition.Field == "fulltext" && condition.Operator == "fulltext" {
		return true
	}

	// 递归检查子条件
	for _, child := range condition.Children {
		if qe.hasFullTextCondition(child) {
			return true
		}
	}

	return false
}

// splitFullTextCondition 拆分全文搜索条件和其他条件
func (qe *FullTextQueryExecutor) splitFullTextCondition(condition *QueryCondition) (*QueryCondition, *QueryCondition) {
	if condition == nil {
		return nil, nil
	}

	// 如果当前条件就是全文搜索条件
	if condition.Field == "fulltext" && condition.Operator == "fulltext" {
		return condition, nil
	}

	// 如果是逻辑操作符（AND或OR）
	if condition.Operator == "and" || condition.Operator == "or" {
		// 对子条件进行处理
		var fullTextChildren []*QueryCondition
		var otherChildren []*QueryCondition

		for _, child := range condition.Children {
			fullTextChild, otherChild := qe.splitFullTextCondition(child)

			// 收集全文搜索条件
			if fullTextChild != nil {
				fullTextChildren = append(fullTextChildren, fullTextChild)
			}

			// 收集其他条件
			if otherChild != nil {
				otherChildren = append(otherChildren, otherChild)
			}
		}

		// 构造全文搜索条件
		var fullTextCondition *QueryCondition
		if len(fullTextChildren) == 1 {
			fullTextCondition = fullTextChildren[0]
		} else if len(fullTextChildren) > 1 {
			fullTextCondition = &QueryCondition{
				Operator: condition.Operator,
				Children: fullTextChildren,
			}
		}

		// 构造其他条件
		var otherCondition *QueryCondition
		if len(otherChildren) == 1 {
			otherCondition = otherChildren[0]
		} else if len(otherChildren) > 1 {
			otherCondition = &QueryCondition{
				Operator: condition.Operator,
				Children: otherChildren,
			}
		}

		return fullTextCondition, otherCondition
	}

	// 其他情况，返回全文搜索条件为null，其他条件为当前条件
	return nil, condition
}

// 在DefaultQueryExecutor中添加对全文搜索操作符的支持
func init() {
	// 向OperatorType类型添加对全文搜索操作符的支持
	_ = OperatorType("fulltext")

	// 可以在日志中记录全文搜索功能已初始化
	fmt.Println("全文搜索功能已初始化")
}
