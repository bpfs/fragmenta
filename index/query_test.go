package index

import (
	"testing"
	"time"
)

// 测试查询解析
func TestQueryParsing(t *testing.T) {
	// 创建模拟索引管理器
	mockIndexManager := createMockIndexManager()
	queryExecutor := NewQueryExecutor(mockIndexManager)

	// 测试用例
	testCases := []struct {
		name        string
		queryStr    string
		expectError bool
		errorType   error
	}{
		{
			name:        "简单标签等于查询",
			queryStr:    "tag:type==1",
			expectError: false,
			errorType:   nil,
		},
		{
			name:        "带限制的查询",
			queryStr:    "tag:type==1; limit: 10",
			expectError: false,
			errorType:   nil,
		},
		{
			name:        "带排序的查询",
			queryStr:    "tag:type==1; sort: -name",
			expectError: false,
			errorType:   nil,
		},
		{
			name:        "复杂逻辑查询",
			queryStr:    "tag:type==1 and tag:category==10",
			expectError: false,
			errorType:   nil,
		},
		{
			name:        "带多个排序的查询",
			queryStr:    "tag:type==1; sort: -name, +size",
			expectError: false,
			errorType:   nil,
		},
		{
			name:        "无效操作符",
			queryStr:    "tag:type??1",
			expectError: true,
			errorType:   ErrSyntaxError,
		},
		{
			name:        "空查询",
			queryStr:    "",
			expectError: true,
			errorType:   ErrInvalidQuery,
		},
		{
			name:        "不等于操作符",
			queryStr:    "tag:type!=1",
			expectError: false,
			errorType:   nil,
		},
		{
			name:        "大于操作符",
			queryStr:    "tag:size>1000",
			expectError: false,
			errorType:   nil,
		},
		{
			name:        "逻辑或操作符",
			queryStr:    "tag:type==1 or tag:type==2",
			expectError: false,
			errorType:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := queryExecutor.ParseQueryString(tc.queryStr)

			// 检查错误
			if tc.expectError {
				if err == nil {
					t.Errorf("期望错误但没有发生错误")
				}
				// 如果指定了错误类型，检查错误类型
				if tc.errorType != nil && err != tc.errorType {
					t.Errorf("期望错误类型为 %v, 但得到 %v", tc.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("不期望错误但发生错误: %v", err)
				}
				if query == nil {
					t.Errorf("期望非空查询对象但得到nil")
				}
			}

			// 如果查询成功解析，检查其基本属性
			if err == nil && query != nil {
				// 检查查询结构的一些基本属性
				if tc.queryStr == "" && query.RootCondition != nil {
					t.Errorf("空查询应该没有条件")
				}
				if tc.queryStr != "" && !tc.expectError && query.RootCondition == nil {
					t.Errorf("非空有效查询应该有条件")
				}
			}
		})
	}
}

// 测试查询执行
func TestQueryExecution(t *testing.T) {
	// 创建模拟索引管理器
	mockIndexManager := createMockIndexManager()
	queryExecutor := NewQueryExecutor(mockIndexManager)

	// 测试用例
	testCases := []struct {
		name          string
		queryStr      string
		expectedCount int // 期望的结果数量
		expectError   bool
	}{
		{
			name:          "查询类型为1的文件",
			queryStr:      "tag:type==1",
			expectedCount: 3, // 模拟数据中有3个type=1的文件
			expectError:   false,
		},
		{
			name:          "查询类型为2的文件",
			queryStr:      "tag:type==2",
			expectedCount: 2, // 模拟数据中有2个type=2的文件
			expectError:   false,
		},
		{
			name:          "查询类型为1且类别为10的文件",
			queryStr:      "tag:type==1 and tag:category==10",
			expectedCount: 1, // 模拟数据中有1个同时满足条件的文件
			expectError:   false,
		},
		{
			name:          "查询类型为1或类型为2的文件",
			queryStr:      "tag:type==1 or tag:type==2",
			expectedCount: 5, // 模拟数据中有5个满足条件的文件
			expectError:   false,
		},
		{
			name:          "查询不存在的类型",
			queryStr:      "tag:type==999",
			expectedCount: 0, // 应该没有匹配项
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := queryExecutor.ParseQueryString(tc.queryStr)
			if err != nil {
				if !tc.expectError {
					t.Errorf("解析查询失败: %v", err)
				}
				return
			}

			// 调试信息：打印查询对象内容
			t.Logf("解析的查询对象: %+v", query)
			if query.RootCondition != nil {
				t.Logf("RootCondition: Field=%s, FieldType=%s, Operator=%s, Value=%v",
					query.RootCondition.Field,
					query.RootCondition.FieldType,
					query.RootCondition.Operator,
					query.RootCondition.Value)

				// 如果有子条件，也打印出来
				if len(query.RootCondition.Children) > 0 {
					for i, child := range query.RootCondition.Children {
						t.Logf("Child %d: Field=%s, FieldType=%s, Operator=%s, Value=%v",
							i, child.Field, child.FieldType, child.Operator, child.Value)
					}
				}
			}

			result, err := queryExecutor.Execute(query)
			if err != nil {
				if !tc.expectError {
					t.Errorf("执行查询失败: %v", err)
				}
				return
			}

			if tc.expectedCount != len(result.IDs) {
				t.Errorf("期望结果数量为 %d, 但得到 %d", tc.expectedCount, len(result.IDs))
				t.Logf("得到的结果: %v", result.IDs)
			}
		})
	}
}

// 测试排序和分页
func TestQuerySortAndPagination(t *testing.T) {
	// 创建模拟索引管理器
	mockIndexManager := createMockIndexManager()
	queryExecutor := NewQueryExecutor(mockIndexManager)

	// 测试用例
	testCases := []struct {
		name          string
		queryStr      string
		expectedIDs   []uint32
		expectedCount int
		expectError   bool
	}{
		{
			name:          "限制查询结果数",
			queryStr:      "tag:type==1; limit: 2",
			expectedIDs:   []uint32{101, 102},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "设置结果偏移",
			queryStr:      "tag:type==1; offset: 1; limit: 2",
			expectedIDs:   []uint32{102, 103},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "排序查询",
			queryStr:      "tag:type==1; sort: +id",
			expectedIDs:   []uint32{101, 102, 103},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name:          "降序排序",
			queryStr:      "tag:type==1; sort: -id",
			expectedIDs:   []uint32{103, 102, 101},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name:          "偏移超出范围",
			queryStr:      "tag:type==1; offset: 5",
			expectedIDs:   []uint32{},
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := queryExecutor.ParseQueryString(tc.queryStr)
			if err != nil {
				if !tc.expectError {
					t.Errorf("解析查询失败: %v", err)
				}
				return
			}

			result, err := queryExecutor.Execute(query)
			if err != nil {
				if !tc.expectError {
					t.Errorf("执行查询失败: %v", err)
				}
				return
			}

			if len(result.IDs) != tc.expectedCount {
				t.Errorf("期望结果数量为 %d, 但得到 %d", tc.expectedCount, len(result.IDs))
				t.Logf("得到的结果: %v", result.IDs)
			}

			// 检查结果顺序
			if len(result.IDs) == len(tc.expectedIDs) {
				for i := 0; i < len(result.IDs); i++ {
					if result.IDs[i] != tc.expectedIDs[i] {
						t.Errorf("结果顺序不正确，位置 %d 期望 %d，但得到 %d",
							i, tc.expectedIDs[i], result.IDs[i])
					}
				}
			}
		})
	}
}

// 创建模拟索引管理器
func createMockIndexManager() *MockIndexManager {
	return createTestMockIndexManager()
}

// TestPrefixTree 测试前缀树
func TestPrefixTree(t *testing.T) {
	im := &IndexManagerImpl{
		config: &IndexConfig{
			EnablePrefixCompression: true,
		},
	}

	// 测试添加索引
	tests := []struct {
		tag uint32
		id  uint32
	}{
		{1, 123},
		{1, 124},
		{1, 125},
		{2, 456},
		{2, 457},
	}

	for _, test := range tests {
		err := im.AddIndex(test.tag, test.id)
		if err != nil {
			t.Errorf("添加索引失败: %v", err)
		}
	}

	// 测试获取前缀树
	tree, err := im.GetPrefixTree(1)
	if err != nil {
		t.Errorf("获取前缀树失败: %v", err)
	}
	if tree == nil {
		t.Error("前缀树为空")
	}

	// 测试前缀搜索
	ids, err := im.FindByPrefix(1, "12")
	if err != nil {
		t.Errorf("前缀搜索失败: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("期望找到3个ID，实际找到%d个", len(ids))
	}

	// 测试空前缀搜索
	ids, err = im.FindByPrefix(1, "")
	if err != nil {
		t.Errorf("空前缀搜索失败: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("期望找到3个ID，实际找到%d个", len(ids))
	}

	// 测试不存在的标签
	ids, err = im.FindByPrefix(3, "1")
	if err != nil {
		t.Errorf("不存在的标签搜索失败: %v", err)
	}
	if len(ids) != 0 {
		t.Error("不存在的标签应该返回空结果")
	}

	// 测试移除索引
	err = im.RemoveIndex(1, 123)
	if err != nil {
		t.Errorf("移除索引失败: %v", err)
	}

	// 验证移除后的结果
	ids, err = im.FindByPrefix(1, "12")
	if err != nil {
		t.Errorf("移除后的前缀搜索失败: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("期望找到2个ID，实际找到%d个", len(ids))
	}
}

// TestCompoundQuery 测试复合查询功能
func TestCompoundQuery(t *testing.T) {
	// 创建索引管理器
	im := &IndexManagerImpl{
		config: &IndexConfig{
			EnablePrefixCompression: true,
		},
	}

	// 添加测试数据
	testData := []struct {
		tag uint32
		id  uint32
	}{
		{1, 100}, // type=1
		{1, 200},
		{1, 300},
		{2, 150}, // type=2
		{2, 250},
		{3, 100}, // type=3
		{3, 200},
		{4, 150}, // type=4
		{4, 250},
	}

	for _, data := range testData {
		err := im.AddIndex(data.tag, data.id)
		if err != nil {
			t.Fatalf("添加索引失败: %v", err)
		}
	}

	// 测试用例
	tests := []struct {
		name        string
		conditions  []IndexQueryCondition
		wantIDs     []uint32
		wantErr     bool
		description string
	}{
		{
			name: "单个条件查询",
			conditions: []IndexQueryCondition{
				{Tag: 1, Operation: "eq", Value: nil},
			},
			wantIDs:     []uint32{100, 200, 300},
			wantErr:     false,
			description: "测试单个标签的查询",
		},
		{
			name: "两个标签交集",
			conditions: []IndexQueryCondition{
				{Tag: 1, Operation: "eq", Value: nil},
				{Tag: 2, Operation: "eq", Value: nil},
			},
			wantIDs:     []uint32{},
			wantErr:     false,
			description: "测试两个标签的交集查询",
		},
		{
			name: "范围查询",
			conditions: []IndexQueryCondition{
				{Tag: 1, Operation: "range", Value: []uint32{150, 250}},
			},
			wantIDs:     []uint32{200},
			wantErr:     false,
			description: "测试范围查询",
		},
		{
			name: "复合条件查询",
			conditions: []IndexQueryCondition{
				{Tag: 1, Operation: "eq", Value: nil},
				{Tag: 3, Operation: "eq", Value: nil},
			},
			wantIDs:     []uint32{100, 200},
			wantErr:     false,
			description: "测试多个条件的组合查询",
		},
		{
			name: "无效操作类型",
			conditions: []IndexQueryCondition{
				{Tag: 1, Operation: "invalid", Value: nil},
			},
			wantIDs:     nil,
			wantErr:     true,
			description: "测试无效的操作类型",
		},
		{
			name:        "空条件列表",
			conditions:  []IndexQueryCondition{},
			wantIDs:     nil,
			wantErr:     true,
			description: "测试空条件列表",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIDs, err := im.FindCompound(tt.conditions)

			// 检查错误
			if (err != nil) != tt.wantErr {
				t.Errorf("FindCompound() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// 检查结果数量
			if len(gotIDs) != len(tt.wantIDs) {
				t.Errorf("FindCompound() got %d IDs, want %d IDs", len(gotIDs), len(tt.wantIDs))
				return
			}

			// 检查结果内容和顺序
			for i := range gotIDs {
				if gotIDs[i] != tt.wantIDs[i] {
					t.Errorf("FindCompound() gotIDs[%d] = %d, want %d", i, gotIDs[i], tt.wantIDs[i])
				}
			}
		})
	}
}

// TestRangeQuery 测试范围查询功能
func TestRangeQuery(t *testing.T) {
	// 创建索引管理器
	im := &IndexManagerImpl{
		config: &IndexConfig{
			EnablePrefixCompression: true,
		},
		metadataIndices: make(map[uint32][]uint32),
	}

	// 添加测试数据
	testData := []struct {
		tag uint32
		id  uint32
	}{
		{1, 100},
		{1, 200},
		{1, 300},
		{1, 400},
		{1, 500},
		{2, 150},
		{2, 250},
		{2, 350},
		{2, 450},
	}

	for _, data := range testData {
		err := im.AddIndex(data.tag, data.id)
		if err != nil {
			t.Fatalf("添加索引失败: %v", err)
		}
	}

	// 测试用例
	tests := []struct {
		name    string
		tag     uint32
		start   uint32
		end     uint32
		wantIDs []uint32
		wantErr bool
	}{
		{
			name:    "完整范围查询",
			tag:     1,
			start:   100,
			end:     500,
			wantIDs: []uint32{100, 200, 300, 400, 500},
			wantErr: false,
		},
		{
			name:    "部分范围查询",
			tag:     1,
			start:   200,
			end:     400,
			wantIDs: []uint32{200, 300, 400},
			wantErr: false,
		},
		{
			name:    "边界值查询",
			tag:     1,
			start:   100,
			end:     100,
			wantIDs: []uint32{100},
			wantErr: false,
		},
		{
			name:    "范围外查询",
			tag:     1,
			start:   600,
			end:     700,
			wantIDs: nil,
			wantErr: true,
		},
		{
			name:    "空范围查询",
			tag:     1,
			start:   300,
			end:     200,
			wantIDs: nil,
			wantErr: true,
		},
		{
			name:    "不存在的标签",
			tag:     3,
			start:   100,
			end:     500,
			wantIDs: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIDs, err := im.FindByRange(tt.tag, tt.start, tt.end)

			// 检查错误
			if (err != nil) != tt.wantErr {
				t.Errorf("FindByRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// 检查结果数量
			if len(gotIDs) != len(tt.wantIDs) {
				t.Errorf("FindByRange() got %d IDs, want %d IDs", len(gotIDs), len(tt.wantIDs))
				return
			}

			// 检查结果内容和顺序
			for i := range gotIDs {
				if gotIDs[i] != tt.wantIDs[i] {
					t.Errorf("FindByRange() gotIDs[%d] = %d, want %d", i, gotIDs[i], tt.wantIDs[i])
				}
			}
		})
	}

	// 性能测试
	t.Run("性能测试", func(t *testing.T) {
		// 生成大量测试数据
		tag := uint32(10)
		for i := 0; i < 10000; i++ {
			err := im.AddIndex(tag, uint32(i))
			if err != nil {
				t.Fatalf("添加索引失败: %v", err)
			}
		}

		// 执行范围查询并记录时间
		start := time.Now()
		ids, err := im.FindByRange(tag, 1000, 9000)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("性能测试查询失败: %v", err)
		}

		if len(ids) != 8001 {
			t.Errorf("性能测试结果数量错误，got %d, want %d", len(ids), 8001)
		}

		t.Logf("范围查询性能测试: 处理 10000 条记录，查询耗时 %v", duration)
	})
}
