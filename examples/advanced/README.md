# FragDB 高级查询示例

本目录包含FragDB存储引擎的高级查询功能示例，展示了如何使用FragDB的查询引擎进行复杂数据查询。

## 查询功能示例

`query_examples.go` 文件演示了以下高级查询功能：

- 基本标签查询
- 单条件元数据查询（相等条件）
- 单条件元数据查询（范围条件）
- 多条件元数据查询（AND逻辑）
- 多条件元数据查询（OR逻辑）
- 内容类型查询
- 排序和分页查询

## 运行方式

这些示例可以通过主程序的advanced命令运行：

```bash
go run examples/cmd/main.go advanced
```

## 注意事项

这些示例展示了FragDB的查询能力，适用于需要高效查询和检索数据的应用场景。

查询功能支持：
- 条件组合（AND/OR）
- 范围查询（大于/小于/等于）
- 排序（升序/降序）
- 分页（偏移量和限制）
- 内容类型过滤

## 查询API说明

FragDB提供了结构化的查询API，支持构建复杂的查询条件：

```go
// 创建查询
query := &fragmenta.MetadataQuery{
    Conditions: []fragmenta.MetadataCondition{
        {
            Tag:      tagID,       // 标签ID
            Operator: opType,      // 操作符（相等、大于、小于等）
            Value:    value,       // 比较值
        },
    },
    Operator:  logicType,  // 逻辑操作（AND/OR）
    SortBy:    sortTag,    // 排序字段
    SortOrder: sortOrder,  // 排序顺序
    Limit:     limit,      // 结果数量限制
    Offset:    offset,     // 结果偏移量
}

// 执行查询
result, err := storage.QueryMetadata(query)
``` 