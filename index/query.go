// package index 提供查询解析和执行功能
package index

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 查询相关错误
var (
	ErrInvalidQuery        = errors.New("无效的查询语句")
	ErrUnsupportedOperator = errors.New("不支持的操作符")
	ErrInvalidValue        = errors.New("无效的值")
	ErrInvalidFieldType    = errors.New("无效的字段类型")
	ErrSyntaxError         = errors.New("查询语法错误")
	ErrMetadataNotFound    = errors.New("未找到元数据")
)

// 操作符类型
type OperatorType string

const (
	// 比较操作符
	OpEqual        OperatorType = "eq"  // 等于
	OpNotEqual     OperatorType = "ne"  // 不等于
	OpGreater      OperatorType = "gt"  // 大于
	OpGreaterEqual OperatorType = "gte" // 大于等于
	OpLess         OperatorType = "lt"  // 小于
	OpLessEqual    OperatorType = "lte" // 小于等于

	// 字符串操作符
	OpContains   OperatorType = "contains"   // 包含
	OpStartsWith OperatorType = "startswith" // 以...开始
	OpEndsWith   OperatorType = "endswith"   // 以...结束
	OpMatches    OperatorType = "matches"    // 正则匹配

	// 逻辑操作符
	OpAnd OperatorType = "and" // 逻辑与
	OpOr  OperatorType = "or"  // 逻辑或
	OpNot OperatorType = "not" // 逻辑非

	// 集合操作符
	OpIn    OperatorType = "in"  // 在集合中
	OpNotIn OperatorType = "nin" // 不在集合中

	// 特殊操作符
	OpExists  OperatorType = "exists"  // 字段存在
	OpBetween OperatorType = "between" // 在范围内
)

// 字段类型
type FieldType string

const (
	TypeString  FieldType = "string"
	TypeInteger FieldType = "integer"
	TypeFloat   FieldType = "float"
	TypeBoolean FieldType = "boolean"
	TypeDate    FieldType = "date"
	TypeTag     FieldType = "tag" // 标签类型（对应uint32）
)

// QueryCondition 查询条件
type QueryCondition struct {
	// Field 字段名
	Field string

	// FieldType 字段类型
	FieldType FieldType

	// Operator 操作符
	Operator OperatorType

	// Value 值
	Value interface{}

	// Children 子条件（用于逻辑操作符）
	Children []*QueryCondition
}

// QuerySort 排序条件
type QuerySort struct {
	// Field 排序字段
	Field string

	// Ascending 是否升序
	Ascending bool
}

// Query 查询
type Query struct {
	// RootCondition 查询根条件
	RootCondition *QueryCondition

	// SortBy 排序条件
	SortBy []*QuerySort

	// Limit 限制结果数量
	Limit int

	// Offset 结果偏移
	Offset int

	// IncludeDeleted 是否包含已删除项
	IncludeDeleted bool
}

// QueryResult 查询结果
type QueryResult struct {
	// IDs 查询到的ID
	IDs []uint32

	// TotalCount 总数（不考虑分页）
	TotalCount int

	// ExecutionTime 执行时间
	ExecutionTime time.Duration
}

// MetadataProvider 元数据提供器接口
type MetadataProvider interface {
	// GetMetadataForID 获取指定ID的元数据
	GetMetadataForID(id uint32) (map[string]interface{}, error)

	// GetAllIDs 获取所有ID
	GetAllIDs() ([]uint32, error)
}

// DefaultMetadataProvider 默认元数据提供器实现
type DefaultMetadataProvider struct {
	// 索引管理器
	indexManager IndexManager

	// 元数据缓存
	metadataCache map[uint32]map[string]interface{}
}

// NewDefaultMetadataProvider 创建默认元数据提供器
func NewDefaultMetadataProvider(indexManager IndexManager) *DefaultMetadataProvider {
	return &DefaultMetadataProvider{
		indexManager:  indexManager,
		metadataCache: make(map[uint32]map[string]interface{}),
	}
}

// GetMetadataForID 获取指定ID的元数据
func (mp *DefaultMetadataProvider) GetMetadataForID(id uint32) (map[string]interface{}, error) {
	// 从缓存中获取
	if metadata, ok := mp.metadataCache[id]; ok {
		return metadata, nil
	}

	// TODO: 从存储中获取元数据
	// 实际实现时应该从存储系统中获取元数据
	// 此处仅作为示例，返回空元数据
	return nil, ErrMetadataNotFound
}

// GetAllIDs 获取所有ID
func (mp *DefaultMetadataProvider) GetAllIDs() ([]uint32, error) {
	// 实际实现时应该从存储系统中获取所有ID
	// 此处简化为从缓存中获取
	ids := make([]uint32, 0, len(mp.metadataCache))
	for id := range mp.metadataCache {
		ids = append(ids, id)
	}
	return ids, nil
}

// QueryExecutor 查询执行器接口
type QueryExecutor interface {
	// Execute 执行查询
	Execute(query *Query) (*QueryResult, error)

	// ParseQueryString 解析查询字符串
	ParseQueryString(queryStr string) (*Query, error)
}

// DefaultQueryExecutor 默认查询执行器实现
type DefaultQueryExecutor struct {
	// 索引管理器
	indexManager IndexManager

	// 元数据提供器
	metadataProvider MetadataProvider
}

// NewQueryExecutor 创建查询执行器
func NewQueryExecutor(indexManager IndexManager) QueryExecutor {
	metadataProvider := NewDefaultMetadataProvider(indexManager)

	return &DefaultQueryExecutor{
		indexManager:     indexManager,
		metadataProvider: metadataProvider,
	}
}

// 带元数据提供器创建查询执行器
func NewQueryExecutorWithMetadataProvider(indexManager IndexManager, metadataProvider MetadataProvider) QueryExecutor {
	return &DefaultQueryExecutor{
		indexManager:     indexManager,
		metadataProvider: metadataProvider,
	}
}

// Execute 执行查询
func (qe *DefaultQueryExecutor) Execute(query *Query) (*QueryResult, error) {
	if query == nil || query.RootCondition == nil {
		return nil, ErrInvalidQuery
	}

	// 记录开始时间
	startTime := time.Now()

	// 执行查询
	ids, err := qe.evaluateCondition(query.RootCondition)
	if err != nil {
		return nil, err
	}

	// 记录总数
	totalCount := len(ids)

	// 应用排序
	if len(query.SortBy) > 0 {
		ids, err = qe.applySorting(ids, query.SortBy)
		if err != nil {
			return nil, err
		}
	}

	// 应用分页
	if query.Offset > 0 {
		if query.Offset >= len(ids) {
			ids = []uint32{}
		} else {
			ids = ids[query.Offset:]
		}
	}

	if query.Limit > 0 && len(ids) > query.Limit {
		ids = ids[:query.Limit]
	}

	return &QueryResult{
		IDs:           ids,
		TotalCount:    totalCount,
		ExecutionTime: time.Since(startTime),
	}, nil
}

// ParseQueryString 解析查询字符串
func (qe *DefaultQueryExecutor) ParseQueryString(queryStr string) (*Query, error) {
	if queryStr == "" {
		return nil, ErrInvalidQuery
	}

	// 分割查询字符串
	parts := strings.Split(queryStr, ";")

	query := &Query{
		Limit:          -1,
		Offset:         0,
		IncludeDeleted: false,
	}

	// 解析每个部分
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 解析限制
		if strings.HasPrefix(part, "limit:") {
			limitStr := strings.TrimPrefix(part, "limit:")
			limitStr = strings.TrimSpace(limitStr)
			limit, err := strconv.Atoi(limitStr)
			if err != nil {
				return nil, fmt.Errorf("无效的limit值: %s", limitStr)
			}
			query.Limit = limit
			continue
		}

		// 解析偏移
		if strings.HasPrefix(part, "offset:") {
			offsetStr := strings.TrimPrefix(part, "offset:")
			offsetStr = strings.TrimSpace(offsetStr)
			offset, err := strconv.Atoi(offsetStr)
			if err != nil {
				return nil, fmt.Errorf("无效的offset值: %s", offsetStr)
			}
			query.Offset = offset
			continue
		}

		// 解析排序
		if strings.HasPrefix(part, "sort:") {
			sortStr := strings.TrimPrefix(part, "sort:")
			query.SortBy = qe.parseSortString(sortStr)
			continue
		}

		// 解析条件
		condition, err := qe.parseConditionString(part)
		if err != nil {
			return nil, err
		}
		query.RootCondition = condition
	}

	return query, nil
}

// parseSortString 解析排序字符串
func (qe *DefaultQueryExecutor) parseSortString(sortStr string) []*QuerySort {
	sortStr = strings.TrimSpace(sortStr)
	if sortStr == "" {
		return nil
	}

	// 分割多个排序字段
	fields := strings.Split(sortStr, ",")
	sorts := make([]*QuerySort, 0, len(fields))

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		sort := &QuerySort{
			Ascending: true,
		}

		// 检查排序方向
		if strings.HasPrefix(field, "+") {
			sort.Field = strings.TrimPrefix(field, "+")
		} else if strings.HasPrefix(field, "-") {
			sort.Field = strings.TrimPrefix(field, "-")
			sort.Ascending = false
		} else {
			sort.Field = field
		}

		sort.Field = strings.TrimSpace(sort.Field)
		if sort.Field != "" {
			sorts = append(sorts, sort)
		}
	}

	return sorts
}

// 解析条件字符串
func (qe *DefaultQueryExecutor) parseConditionString(condStr string) (*QueryCondition, error) {
	// 检查是否是exists操作符
	if existsMatch := regexp.MustCompile(`^exists\s+(.*?)$`).FindStringSubmatch(condStr); len(existsMatch) == 2 {
		return qe.parseExistsCondition(existsMatch[1])
	}

	// 检查是否是between操作符
	if betweenMatch := regexp.MustCompile(`^(.*?)\s+between\s+(.*?)\s+and\s+(.*?)$`).FindStringSubmatch(condStr); len(betweenMatch) == 4 {
		return qe.parseBetweenCondition(betweenMatch[1], betweenMatch[2], betweenMatch[3])
	}

	// 检查是否是集合操作符
	if inMatch := regexp.MustCompile(`^(.*?)\s+in\s+\[(.*?)\]$`).FindStringSubmatch(condStr); len(inMatch) == 3 {
		return qe.parseInCondition(inMatch[1], inMatch[2], false)
	}

	if notInMatch := regexp.MustCompile(`^(.*?)\s+not\s+in\s+\[(.*?)\]$`).FindStringSubmatch(condStr); len(notInMatch) == 3 {
		return qe.parseInCondition(notInMatch[1], notInMatch[2], true)
	}

	// 检查是否包含逻辑操作符
	if strings.Contains(condStr, " and ") {
		return qe.parseLogicalCondition(condStr, OpAnd)
	} else if strings.Contains(condStr, " or ") {
		return qe.parseLogicalCondition(condStr, OpOr)
	}

	// 解析简单条件
	return qe.parseSimpleCondition(condStr)
}

// 解析逻辑条件
func (qe *DefaultQueryExecutor) parseLogicalCondition(condStr string, operator OperatorType) (*QueryCondition, error) {
	var separator string
	if operator == OpAnd {
		separator = " and "
	} else if operator == OpOr {
		separator = " or "
	} else {
		return nil, ErrUnsupportedOperator
	}

	// 分离子条件
	subCondStrs := strings.Split(condStr, separator)
	if len(subCondStrs) < 2 {
		return nil, ErrSyntaxError
	}

	children := make([]*QueryCondition, 0, len(subCondStrs))
	for _, subCondStr := range subCondStrs {
		subCondStr = strings.TrimSpace(subCondStr)
		if subCondStr == "" {
			continue
		}

		subCond, err := qe.parseConditionString(subCondStr)
		if err != nil {
			return nil, err
		}

		children = append(children, subCond)
	}

	if len(children) < 2 {
		return nil, ErrSyntaxError
	}

	return &QueryCondition{
		Operator: operator,
		Children: children,
	}, nil
}

// parseSimpleCondition 解析简单条件
func (qe *DefaultQueryExecutor) parseSimpleCondition(condStr string) (*QueryCondition, error) {
	// 查找操作符
	var operator OperatorType
	var operatorStr string
	var operatorFound bool

	// 支持的操作符映射
	operatorMap := map[string]OperatorType{
		"==": OpEqual,
		"!=": OpNotEqual,
		">":  OpGreater,
		">=": OpGreaterEqual,
		"<":  OpLess,
		"<=": OpLessEqual,
	}

	// 按长度排序操作符，优先匹配长的
	operators := []string{">=", "<=", "==", "!=", ">", "<"}
	for _, op := range operators {
		if strings.Contains(condStr, op) {
			operatorStr = op
			operator = operatorMap[op]
			operatorFound = true
			break
		}
	}

	if !operatorFound {
		return nil, ErrSyntaxError
	}

	// 分割字段和值
	parts := strings.Split(condStr, operatorStr)
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: 条件格式错误", ErrSyntaxError)
	}

	field := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// 解析字段类型
	var fieldType FieldType
	if strings.HasPrefix(field, "tag:") {
		fieldType = TypeTag
		field = strings.TrimPrefix(field, "tag:")
	} else {
		// 根据值推断类型
		if _, err := strconv.Atoi(value); err == nil {
			fieldType = TypeInteger
		} else if _, err := strconv.ParseFloat(value, 64); err == nil {
			fieldType = TypeFloat
		} else if value == "true" || value == "false" {
			fieldType = TypeBoolean
		} else {
			fieldType = TypeString
		}
	}

	// 解析值
	parsedValue, err := qe.parseValue(value, fieldType)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSyntaxError, err)
	}

	return &QueryCondition{
		Field:     field,
		FieldType: fieldType,
		Operator:  operator,
		Value:     parsedValue,
	}, nil
}

// parseInCondition 解析 in 和 not in 条件
func (qe *DefaultQueryExecutor) parseInCondition(fieldStr, valuesStr string, isNotIn bool) (*QueryCondition, error) {
	fieldStr = strings.TrimSpace(fieldStr)
	if fieldStr == "" {
		return nil, ErrSyntaxError
	}

	// 判断字段类型
	fieldType := TypeString // 默认为字符串类型

	// 简单类型推断
	if strings.HasPrefix(fieldStr, "tag:") {
		fieldStr = strings.TrimPrefix(fieldStr, "tag:")
		fieldType = TypeTag
	} else if strings.HasPrefix(fieldStr, "int:") {
		fieldStr = strings.TrimPrefix(fieldStr, "int:")
		fieldType = TypeInteger
	} else if strings.HasPrefix(fieldStr, "float:") {
		fieldStr = strings.TrimPrefix(fieldStr, "float:")
		fieldType = TypeFloat
	} else if strings.HasPrefix(fieldStr, "bool:") {
		fieldStr = strings.TrimPrefix(fieldStr, "bool:")
		fieldType = TypeBoolean
	} else if strings.HasPrefix(fieldStr, "date:") {
		fieldStr = strings.TrimPrefix(fieldStr, "date:")
		fieldType = TypeDate
	}

	// 解析值列表
	valueStrings := strings.Split(valuesStr, ",")
	values := make([]interface{}, 0, len(valueStrings))

	for _, valueStr := range valueStrings {
		valueStr = strings.TrimSpace(valueStr)
		if valueStr == "" {
			continue
		}

		// 解析单个值
		var value interface{}
		var err error

		switch fieldType {
		case TypeString:
			// 去除字符串的引号（如果有）
			if (strings.HasPrefix(valueStr, "'") && strings.HasSuffix(valueStr, "'")) ||
				(strings.HasPrefix(valueStr, "\"") && strings.HasSuffix(valueStr, "\"")) {
				valueStr = valueStr[1 : len(valueStr)-1]
			}
			value = valueStr
		case TypeInteger:
			value, err = strconv.ParseInt(valueStr, 10, 64)
		case TypeFloat:
			value, err = strconv.ParseFloat(valueStr, 64)
		case TypeBoolean:
			value, err = strconv.ParseBool(valueStr)
		case TypeDate:
			// 尝试解析日期
			value, err = time.Parse(time.RFC3339, valueStr)
			if err != nil {
				// 尝试其他常见日期格式
				formats := []string{
					"2006-01-02",
					"2006-01-02 15:04:05",
				}

				for _, format := range formats {
					value, err = time.Parse(format, valueStr)
					if err == nil {
						break
					}
				}
			}
		case TypeTag:
			// 解析标签（uint32）
			var tagVal uint64
			tagVal, err = strconv.ParseUint(valueStr, 10, 32)
			if err == nil {
				value = uint32(tagVal)
			}
		}

		if err != nil {
			return nil, fmt.Errorf("%w: 无法解析值 '%s' 为 %s 类型: %v", ErrInvalidValue, valueStr, fieldType, err)
		}

		values = append(values, value)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("%w: 集合为空", ErrSyntaxError)
	}

	var operator OperatorType
	if isNotIn {
		operator = OpNotIn
	} else {
		operator = OpIn
	}

	return &QueryCondition{
		Field:     fieldStr,
		FieldType: fieldType,
		Operator:  operator,
		Value:     values,
	}, nil
}

// evaluateCondition 评估查询条件
func (qe *DefaultQueryExecutor) evaluateCondition(condition *QueryCondition) ([]uint32, error) {
	if condition == nil {
		return nil, ErrInvalidQuery
	}

	// 处理逻辑操作符
	if condition.Operator == OpAnd || condition.Operator == OpOr {
		if len(condition.Children) == 0 {
			return nil, ErrInvalidQuery
		}

		// 评估第一个子条件
		result, err := qe.evaluateCondition(condition.Children[0])
		if err != nil {
			return nil, err
		}

		// 评估其余子条件并应用逻辑操作
		for i := 1; i < len(condition.Children); i++ {
			childResult, err := qe.evaluateCondition(condition.Children[i])
			if err != nil {
				return nil, err
			}

			if condition.Operator == OpAnd {
				// 逻辑与：取交集
				result = qe.intersect(result, childResult)
			} else {
				// 逻辑或：取并集
				result = qe.union(result, childResult)
			}
		}

		return result, nil
	} else if condition.Operator == OpNot {
		if len(condition.Children) != 1 {
			return nil, ErrInvalidQuery
		}

		// 对子条件求反
		// TODO: 实现求反操作（需要知道所有可能的ID）
		return nil, fmt.Errorf("暂不支持逻辑非操作")
	}

	// 处理标签条件
	if condition.FieldType == TypeTag {
		// 特殊字段处理
		if condition.Field == "type" && condition.Operator == OpEqual {
			// 对于type字段，需要将值从int转为对应的标签ID
			switch v := condition.Value.(type) {
			case int:
				// 将type=1转为tag=1000，type=2转为tag=1001，以此类推
				tag := uint32(1000 + v - 1)
				return qe.indexManager.FindByTag(tag)
			case int64:
				tag := uint32(1000 + int(v) - 1)
				return qe.indexManager.FindByTag(tag)
			default:
				return nil, fmt.Errorf("无效的type值类型: %T", condition.Value)
			}
		} else if condition.Field == "category" && condition.Operator == OpEqual {
			// 对于category字段，需要将值从int转为对应的标签ID
			switch v := condition.Value.(type) {
			case int:
				// 将category=10转为tag=2010，以此类推
				tag := uint32(2000 + v)
				return qe.indexManager.FindByTag(tag)
			case int64:
				tag := uint32(2000 + int(v))
				return qe.indexManager.FindByTag(tag)
			default:
				return nil, fmt.Errorf("无效的category值类型: %T", condition.Value)
			}
		} else if condition.Operator == OpEqual {
			// 普通标签等于查询
			switch v := condition.Value.(type) {
			case uint32:
				return qe.indexManager.FindByTag(v)
			case int64:
				return qe.indexManager.FindByTag(uint32(v))
			case int:
				return qe.indexManager.FindByTag(uint32(v))
			default:
				return nil, fmt.Errorf("无效的标签值类型: %T", condition.Value)
			}
		} else if condition.Operator == OpIn || condition.Operator == OpNotIn {
			// 处理In操作符，针对标签
			return qe.evaluateTagInCondition(condition)
		} else {
			return nil, ErrUnsupportedOperator
		}
	}

	// 对于其他类型的字段，在元数据中进行查询
	return qe.evaluateMetadataCondition(condition)
}

// evaluateTagInCondition 评估标签的In条件
func (qe *DefaultQueryExecutor) evaluateTagInCondition(condition *QueryCondition) ([]uint32, error) {
	values, ok := condition.Value.([]interface{})
	if !ok {
		return nil, ErrInvalidValue
	}

	// 处理空集合
	if len(values) == 0 {
		return []uint32{}, nil
	}

	// 查询每个标签，并合并结果
	var resultIDs []uint32
	var err error

	for i, value := range values {
		tag, ok := value.(uint32)
		if !ok {
			return nil, fmt.Errorf("%w: 无效的标签值", ErrInvalidValue)
		}

		// 查询标签
		tagIDs, err := qe.indexManager.FindByTag(tag)
		if err != nil {
			return nil, err
		}

		// 第一个标签，直接赋值
		if i == 0 {
			resultIDs = tagIDs
		} else {
			// 合并结果
			if condition.Operator == OpIn {
				// in: 取并集
				resultIDs = qe.union(resultIDs, tagIDs)
			} else {
				// not in: 取交集
				resultIDs = qe.intersect(resultIDs, tagIDs)
			}
		}
	}

	// 对于not in操作符，需要取反
	if condition.Operator == OpNotIn {
		// 获取所有ID
		allIDs, err := qe.metadataProvider.GetAllIDs()
		if err != nil {
			return nil, err
		}

		// 从所有ID中移除匹配的ID
		resultIDs = qe.difference(allIDs, resultIDs)
	}

	return resultIDs, err
}

// evaluateMetadataCondition 评估元数据查询条件
func (qe *DefaultQueryExecutor) evaluateMetadataCondition(condition *QueryCondition) ([]uint32, error) {
	// 获取所有ID
	allIDs, err := qe.metadataProvider.GetAllIDs()
	if err != nil {
		return nil, err
	}

	// 过滤满足条件的ID
	var resultIDs []uint32

	for _, id := range allIDs {
		// 获取元数据
		metadata, err := qe.metadataProvider.GetMetadataForID(id)
		if err != nil {
			if err == ErrMetadataNotFound {
				// 如果没有元数据，跳过
				continue
			}
			return nil, err
		}

		// 检查是否符合条件
		var matched bool

		if condition.Operator == OpExists {
			// 检查字段是否存在
			_, exists := metadata[condition.Field]
			matched = exists
		} else {
			// 检查字段是否存在
			fieldValue, ok := metadata[condition.Field]
			if !ok {
				// 字段不存在，跳过
				continue
			}

			// 根据操作符类型判断匹配方式
			var matchErr error

			if condition.Operator == OpIn || condition.Operator == OpNotIn {
				// 处理集合操作符
				matched, matchErr = qe.matchInCondition(condition, fieldValue)
			} else if condition.Operator == OpBetween {
				// 处理范围操作符
				matched, matchErr = qe.matchBetweenCondition(condition, fieldValue)
			} else {
				// 处理其他操作符
				matched, matchErr = qe.matchCondition(condition, fieldValue)
			}

			if matchErr != nil {
				return nil, matchErr
			}
		}

		if matched {
			resultIDs = append(resultIDs, id)
		}
	}

	return resultIDs, nil
}

// matchInCondition 判断值是否满足集合条件
func (qe *DefaultQueryExecutor) matchInCondition(condition *QueryCondition, value interface{}) (bool, error) {
	values, ok := condition.Value.([]interface{})
	if !ok {
		return false, ErrInvalidValue
	}

	// 检查值是否在集合中
	for _, v := range values {
		// 创建一个临时条件，用于比较
		tempCondition := &QueryCondition{
			Field:     condition.Field,
			FieldType: condition.FieldType,
			Operator:  OpEqual,
			Value:     v,
		}

		// 检查是否匹配
		matched, err := qe.matchCondition(tempCondition, value)
		if err != nil {
			return false, err
		}

		if matched {
			// 如果是in操作符，有一个匹配就返回true
			// 如果是not in操作符，有一个匹配就返回false
			return condition.Operator == OpIn, nil
		}
	}

	// 如果没有匹配的项
	// 对于in操作符，返回false
	// 对于not in操作符，返回true
	return condition.Operator == OpNotIn, nil
}

// difference 计算两个切片的差集（a中有但b中没有的元素）
func (qe *DefaultQueryExecutor) difference(a, b []uint32) []uint32 {
	if len(a) == 0 {
		return []uint32{}
	}
	if len(b) == 0 {
		return a
	}

	// 使用map记录b中的元素
	bMap := make(map[uint32]bool, len(b))
	for _, id := range b {
		bMap[id] = true
	}

	// 找出a中有但b中没有的元素
	result := make([]uint32, 0, len(a))
	for _, id := range a {
		if !bMap[id] {
			result = append(result, id)
		}
	}

	return result
}

// matchCondition 判断值是否满足条件
func (qe *DefaultQueryExecutor) matchCondition(condition *QueryCondition, value interface{}) (bool, error) {
	switch condition.FieldType {
	case TypeString:
		return qe.matchStringCondition(condition, value)
	case TypeInteger:
		return qe.matchIntegerCondition(condition, value)
	case TypeFloat:
		return qe.matchFloatCondition(condition, value)
	case TypeBoolean:
		return qe.matchBooleanCondition(condition, value)
	case TypeDate:
		return qe.matchDateCondition(condition, value)
	default:
		return false, ErrInvalidFieldType
	}
}

// matchStringCondition 判断字符串是否满足条件
func (qe *DefaultQueryExecutor) matchStringCondition(condition *QueryCondition, value interface{}) (bool, error) {
	strValue, ok := value.(string)
	if !ok {
		// 尝试转换为字符串
		strValue = fmt.Sprintf("%v", value)
	}

	condValue, ok := condition.Value.(string)
	if !ok {
		return false, ErrInvalidValue
	}

	switch condition.Operator {
	case OpEqual:
		return strValue == condValue, nil
	case OpNotEqual:
		return strValue != condValue, nil
	case OpContains:
		return strings.Contains(strValue, condValue), nil
	case OpStartsWith:
		return strings.HasPrefix(strValue, condValue), nil
	case OpEndsWith:
		return strings.HasSuffix(strValue, condValue), nil
	case OpMatches:
		// 使用正则表达式匹配
		matched, err := regexp.MatchString(condValue, strValue)
		if err != nil {
			return false, err
		}
		return matched, nil
	default:
		return false, ErrUnsupportedOperator
	}
}

// matchIntegerCondition 判断整数是否满足条件
func (qe *DefaultQueryExecutor) matchIntegerCondition(condition *QueryCondition, value interface{}) (bool, error) {
	var intValue int64

	// 尝试将值转换为int64
	switch v := value.(type) {
	case int:
		intValue = int64(v)
	case int8:
		intValue = int64(v)
	case int16:
		intValue = int64(v)
	case int32:
		intValue = int64(v)
	case int64:
		intValue = v
	case uint:
		intValue = int64(v)
	case uint8:
		intValue = int64(v)
	case uint16:
		intValue = int64(v)
	case uint32:
		intValue = int64(v)
	case uint64:
		if v > (1<<63 - 1) {
			return false, ErrInvalidValue
		}
		intValue = int64(v)
	case float32:
		intValue = int64(v)
	case float64:
		intValue = int64(v)
	case string:
		var err error
		intValue, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return false, err
		}
	default:
		return false, ErrInvalidValue
	}

	condValue, ok := condition.Value.(int64)
	if !ok {
		return false, ErrInvalidValue
	}

	switch condition.Operator {
	case OpEqual:
		return intValue == condValue, nil
	case OpNotEqual:
		return intValue != condValue, nil
	case OpGreater:
		return intValue > condValue, nil
	case OpGreaterEqual:
		return intValue >= condValue, nil
	case OpLess:
		return intValue < condValue, nil
	case OpLessEqual:
		return intValue <= condValue, nil
	default:
		return false, ErrUnsupportedOperator
	}
}

// matchFloatCondition 判断浮点数是否满足条件
func (qe *DefaultQueryExecutor) matchFloatCondition(condition *QueryCondition, value interface{}) (bool, error) {
	var floatValue float64

	// 尝试将值转换为float64
	switch v := value.(type) {
	case int:
		floatValue = float64(v)
	case int8:
		floatValue = float64(v)
	case int16:
		floatValue = float64(v)
	case int32:
		floatValue = float64(v)
	case int64:
		floatValue = float64(v)
	case uint:
		floatValue = float64(v)
	case uint8:
		floatValue = float64(v)
	case uint16:
		floatValue = float64(v)
	case uint32:
		floatValue = float64(v)
	case uint64:
		floatValue = float64(v)
	case float32:
		floatValue = float64(v)
	case float64:
		floatValue = v
	case string:
		var err error
		floatValue, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return false, err
		}
	default:
		return false, ErrInvalidValue
	}

	condValue, ok := condition.Value.(float64)
	if !ok {
		return false, ErrInvalidValue
	}

	switch condition.Operator {
	case OpEqual:
		return floatValue == condValue, nil
	case OpNotEqual:
		return floatValue != condValue, nil
	case OpGreater:
		return floatValue > condValue, nil
	case OpGreaterEqual:
		return floatValue >= condValue, nil
	case OpLess:
		return floatValue < condValue, nil
	case OpLessEqual:
		return floatValue <= condValue, nil
	default:
		return false, ErrUnsupportedOperator
	}
}

// matchBooleanCondition 判断布尔值是否满足条件
func (qe *DefaultQueryExecutor) matchBooleanCondition(condition *QueryCondition, value interface{}) (bool, error) {
	var boolValue bool

	// 尝试将值转换为bool
	switch v := value.(type) {
	case bool:
		boolValue = v
	case string:
		var err error
		boolValue, err = strconv.ParseBool(v)
		if err != nil {
			return false, err
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// 数字0为false，其他为true
		boolValue = v != 0
	default:
		return false, ErrInvalidValue
	}

	condValue, ok := condition.Value.(bool)
	if !ok {
		return false, ErrInvalidValue
	}

	switch condition.Operator {
	case OpEqual:
		return boolValue == condValue, nil
	case OpNotEqual:
		return boolValue != condValue, nil
	default:
		return false, ErrUnsupportedOperator
	}
}

// matchDateCondition 判断日期是否满足条件
func (qe *DefaultQueryExecutor) matchDateCondition(condition *QueryCondition, value interface{}) (bool, error) {
	var dateValue time.Time

	// 尝试将值转换为time.Time
	switch v := value.(type) {
	case time.Time:
		dateValue = v
	case string:
		// 尝试解析日期
		var err error
		dateValue, err = time.Parse(time.RFC3339, v)
		if err != nil {
			// 尝试其他常见日期格式
			formats := []string{
				"2006-01-02",
				"2006-01-02 15:04:05",
			}

			for _, format := range formats {
				dateValue, err = time.Parse(format, v)
				if err == nil {
					break
				}
			}

			if err != nil {
				return false, err
			}
		}
	default:
		return false, ErrInvalidValue
	}

	condValue, ok := condition.Value.(time.Time)
	if !ok {
		return false, ErrInvalidValue
	}

	switch condition.Operator {
	case OpEqual:
		return dateValue.Equal(condValue), nil
	case OpNotEqual:
		return !dateValue.Equal(condValue), nil
	case OpGreater:
		return dateValue.After(condValue), nil
	case OpGreaterEqual:
		return dateValue.After(condValue) || dateValue.Equal(condValue), nil
	case OpLess:
		return dateValue.Before(condValue), nil
	case OpLessEqual:
		return dateValue.Before(condValue) || dateValue.Equal(condValue), nil
	default:
		return false, ErrUnsupportedOperator
	}
}

// intersect 取两个ID列表的交集
func (qe *DefaultQueryExecutor) intersect(a, b []uint32) []uint32 {
	if len(a) == 0 || len(b) == 0 {
		return []uint32{}
	}

	// 使用map优化查找
	bMap := make(map[uint32]bool, len(b))
	for _, id := range b {
		bMap[id] = true
	}

	result := make([]uint32, 0)
	for _, id := range a {
		if bMap[id] {
			result = append(result, id)
		}
	}

	return result
}

// union 取两个ID列表的并集
func (qe *DefaultQueryExecutor) union(a, b []uint32) []uint32 {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}

	// 使用map去重
	result := make([]uint32, 0, len(a)+len(b))
	seen := make(map[uint32]bool, len(a)+len(b))

	// 添加a中的元素
	for _, id := range a {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	// 添加b中的元素
	for _, id := range b {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	return result
}

// parseExistsCondition 解析 exists 条件
func (qe *DefaultQueryExecutor) parseExistsCondition(fieldStr string) (*QueryCondition, error) {
	fieldStr = strings.TrimSpace(fieldStr)
	if fieldStr == "" {
		return nil, ErrSyntaxError
	}

	// 判断字段类型
	fieldType := TypeString // 默认为字符串类型

	// 简单类型推断
	if strings.HasPrefix(fieldStr, "tag:") {
		fieldStr = strings.TrimPrefix(fieldStr, "tag:")
		fieldType = TypeTag
	} else if strings.HasPrefix(fieldStr, "int:") {
		fieldStr = strings.TrimPrefix(fieldStr, "int:")
		fieldType = TypeInteger
	} else if strings.HasPrefix(fieldStr, "float:") {
		fieldStr = strings.TrimPrefix(fieldStr, "float:")
		fieldType = TypeFloat
	} else if strings.HasPrefix(fieldStr, "bool:") {
		fieldStr = strings.TrimPrefix(fieldStr, "bool:")
		fieldType = TypeBoolean
	} else if strings.HasPrefix(fieldStr, "date:") {
		fieldStr = strings.TrimPrefix(fieldStr, "date:")
		fieldType = TypeDate
	}

	return &QueryCondition{
		Field:     fieldStr,
		FieldType: fieldType,
		Operator:  OpExists,
		Value:     true, // 默认检查字段存在
	}, nil
}

// parseBetweenCondition 解析 between 条件
func (qe *DefaultQueryExecutor) parseBetweenCondition(fieldStr, minStr, maxStr string) (*QueryCondition, error) {
	fieldStr = strings.TrimSpace(fieldStr)
	minStr = strings.TrimSpace(minStr)
	maxStr = strings.TrimSpace(maxStr)

	if fieldStr == "" || minStr == "" || maxStr == "" {
		return nil, ErrSyntaxError
	}

	// 判断字段类型
	fieldType := TypeString // 默认为字符串类型

	// 简单类型推断
	if strings.HasPrefix(fieldStr, "tag:") {
		fieldStr = strings.TrimPrefix(fieldStr, "tag:")
		fieldType = TypeTag
	} else if strings.HasPrefix(fieldStr, "int:") {
		fieldStr = strings.TrimPrefix(fieldStr, "int:")
		fieldType = TypeInteger
	} else if strings.HasPrefix(fieldStr, "float:") {
		fieldStr = strings.TrimPrefix(fieldStr, "float:")
		fieldType = TypeFloat
	} else if strings.HasPrefix(fieldStr, "bool:") {
		fieldStr = strings.TrimPrefix(fieldStr, "bool:")
		fieldType = TypeBoolean
	} else if strings.HasPrefix(fieldStr, "date:") {
		fieldStr = strings.TrimPrefix(fieldStr, "date:")
		fieldType = TypeDate
	}

	// 解析最小值和最大值
	minValue, err := qe.parseValue(minStr, fieldType)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法解析最小值 '%s': %v", ErrInvalidValue, minStr, err)
	}

	maxValue, err := qe.parseValue(maxStr, fieldType)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法解析最大值 '%s': %v", ErrInvalidValue, maxStr, err)
	}

	// 创建范围值数组
	rangeValues := []interface{}{minValue, maxValue}

	return &QueryCondition{
		Field:     fieldStr,
		FieldType: fieldType,
		Operator:  OpBetween,
		Value:     rangeValues,
	}, nil
}

// parseValue 解析值
func (qe *DefaultQueryExecutor) parseValue(valueStr string, fieldType FieldType) (interface{}, error) {
	switch fieldType {
	case TypeString:
		return valueStr, nil
	case TypeInteger:
		return strconv.ParseInt(valueStr, 10, 64)
	case TypeFloat:
		return strconv.ParseFloat(valueStr, 64)
	case TypeBoolean:
		return strconv.ParseBool(valueStr)
	case TypeDate:
		// 尝试解析日期
		t, err := time.Parse(time.RFC3339, valueStr)
		if err != nil {
			// 尝试其他常见日期格式
			formats := []string{
				"2006-01-02",
				"2006-01-02 15:04:05",
			}

			for _, format := range formats {
				t, err = time.Parse(format, valueStr)
				if err == nil {
					break
				}
			}
		}
		return t, err
	case TypeTag:
		// 对于标签类型，使用int作为存储类型，方便后续处理
		intVal, err := strconv.Atoi(valueStr)
		if err != nil {
			return nil, err
		}
		return intVal, nil
	default:
		return nil, ErrInvalidFieldType
	}
}

// matchBetweenCondition 判断值是否在范围内
func (qe *DefaultQueryExecutor) matchBetweenCondition(condition *QueryCondition, value interface{}) (bool, error) {
	rangeValues, ok := condition.Value.([]interface{})
	if !ok || len(rangeValues) != 2 {
		return false, ErrInvalidValue
	}

	minValue := rangeValues[0]
	maxValue := rangeValues[1]

	switch condition.FieldType {
	case TypeString:
		return qe.matchStringBetween(value, minValue, maxValue)
	case TypeInteger:
		return qe.matchIntegerBetween(value, minValue, maxValue)
	case TypeFloat:
		return qe.matchFloatBetween(value, minValue, maxValue)
	case TypeDate:
		return qe.matchDateBetween(value, minValue, maxValue)
	default:
		return false, fmt.Errorf("%w: between操作符不支持该类型", ErrUnsupportedOperator)
	}
}

// matchStringBetween 判断字符串是否在范围内
func (qe *DefaultQueryExecutor) matchStringBetween(value, minValue, maxValue interface{}) (bool, error) {
	strValue, ok := value.(string)
	if !ok {
		// 尝试转换为字符串
		strValue = fmt.Sprintf("%v", value)
	}

	minStr, ok := minValue.(string)
	if !ok {
		return false, ErrInvalidValue
	}

	maxStr, ok := maxValue.(string)
	if !ok {
		return false, ErrInvalidValue
	}

	// 字符串比较
	return strValue >= minStr && strValue <= maxStr, nil
}

// matchIntegerBetween 判断整数是否在范围内
func (qe *DefaultQueryExecutor) matchIntegerBetween(value, minValue, maxValue interface{}) (bool, error) {
	// 将值转换为int64
	intValue, err := qe.toInt64(value)
	if err != nil {
		return false, err
	}

	// 将最小值转换为int64
	minInt, err := qe.toInt64(minValue)
	if err != nil {
		return false, err
	}

	// 将最大值转换为int64
	maxInt, err := qe.toInt64(maxValue)
	if err != nil {
		return false, err
	}

	// 范围比较
	return intValue >= minInt && intValue <= maxInt, nil
}

// matchFloatBetween 判断浮点数是否在范围内
func (qe *DefaultQueryExecutor) matchFloatBetween(value, minValue, maxValue interface{}) (bool, error) {
	// 将值转换为float64
	floatValue, err := qe.toFloat64(value)
	if err != nil {
		return false, err
	}

	// 将最小值转换为float64
	minFloat, err := qe.toFloat64(minValue)
	if err != nil {
		return false, err
	}

	// 将最大值转换为float64
	maxFloat, err := qe.toFloat64(maxValue)
	if err != nil {
		return false, err
	}

	// 范围比较
	return floatValue >= minFloat && floatValue <= maxFloat, nil
}

// matchDateBetween 判断日期是否在范围内
func (qe *DefaultQueryExecutor) matchDateBetween(value, minValue, maxValue interface{}) (bool, error) {
	// 将值转换为time.Time
	dateValue, err := qe.toTime(value)
	if err != nil {
		return false, err
	}

	// 将最小值转换为time.Time
	minDate, err := qe.toTime(minValue)
	if err != nil {
		return false, err
	}

	// 将最大值转换为time.Time
	maxDate, err := qe.toTime(maxValue)
	if err != nil {
		return false, err
	}

	// 范围比较
	return (dateValue.After(minDate) || dateValue.Equal(minDate)) &&
		(dateValue.Before(maxDate) || dateValue.Equal(maxDate)), nil
}

// toInt64 将值转换为int64
func (qe *DefaultQueryExecutor) toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > (1<<63 - 1) {
			return 0, ErrInvalidValue
		}
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, ErrInvalidValue
	}
}

// toFloat64 将值转换为float64
func (qe *DefaultQueryExecutor) toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, ErrInvalidValue
	}
}

// toTime 将值转换为time.Time
func (qe *DefaultQueryExecutor) toTime(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		// 尝试解析日期
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// 尝试其他常见日期格式
			formats := []string{
				"2006-01-02",
				"2006-01-02 15:04:05",
			}

			for _, format := range formats {
				t, err = time.Parse(format, v)
				if err == nil {
					return t, nil
				}
			}

			return time.Time{}, err
		}
		return t, nil
	default:
		return time.Time{}, ErrInvalidValue
	}
}

// applySorting 应用排序
func (qe *DefaultQueryExecutor) applySorting(ids []uint32, sortCriteria []*QuerySort) ([]uint32, error) {
	if len(sortCriteria) == 0 {
		return ids, nil
	}

	// 创建排序元素
	elements := make([]uint32, len(ids))
	copy(elements, ids)

	// 根据排序条件排序
	sort.Slice(elements, func(i, j int) bool {
		for _, sort := range sortCriteria {
			if sort.Field == "id" {
				if sort.Ascending {
					return elements[i] < elements[j]
				} else {
					return elements[i] > elements[j]
				}
			}
		}
		return elements[i] < elements[j]
	})

	return elements, nil
}
