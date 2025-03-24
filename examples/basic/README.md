# FragDB 基础示例

本目录包含FragDB存储引擎的基础使用示例，演示了如何使用FragDB的核心功能。

## 示例文件说明

### 基本使用示例 ([basic_usage.go](basic_usage.go))

演示FragDB的基本操作，包括：
- 创建和打开FragDB文件
- 写入和读取数据块
- 设置和获取元数据
- 链接数据块
- 提交更改和关闭文件

```go
// 创建FragDB文件
f, err := fragmenta.CreateFragmenta("example.frag", options)

// 写入数据块
blockID, err := f.WriteBlock(data, nil)

// 设置元数据
err = f.SetMetadata(fragmenta.UserTag(0x1001), []byte("文本文件"))

// 获取元数据
fileName, err := f.GetMetadata(fragmenta.UserTag(0x0004))
```

### 高级查询示例 ([query_example.go](query_example.go))

演示FragDB的强大查询功能，包括：
- 标签值匹配查询
- 范围查询
- 复合查询（AND、OR、分组）
- 全文搜索
- 排序和分页

```go
// 简单标签查询
query := fragmenta.NewQuery().
    MatchTagValue(fragmenta.UserTagRange(0x2000, 0x2020), []byte("图片"))

// 复合查询示例
query = fragmenta.NewQuery().
    GroupStart().
        MatchTagValue(fragmenta.UserTagRange(0x2000, 0x2020), []byte("音频")).
        Or().
        MatchTagValue(fragmenta.UserTagRange(0x2000, 0x2020), []byte("视频")).
    GroupEnd().
    And().
    MatchTagValueMin(fragmenta.UserTagRange(0x4000, 0x4020), minSize)
```

### 存储模式示例 ([storage_modes_example.go](storage_modes_example.go))

演示FragDB的多种存储模式，包括：
- 容器模式：所有数据存储在单一文件中
- 目录模式：数据分散在目录结构中
- 内存模式：数据存储在内存中（不持久化）
- 混合模式：根据文件大小和类型智能选择存储方式

```go
// 容器模式
f, err := fragmenta.CreateFragmenta("container.frag", &fragmenta.FragmentaOptions{
    StorageMode: fragmenta.ContainerMode,
})

// 目录模式
f, err := fragmenta.CreateFragmenta("dir_path", &fragmenta.FragmentaOptions{
    StorageMode: fragmenta.DirectoryMode,
})
```

### 安全功能示例 ([security_example.go](security_example.go))

演示FragDB的安全功能，包括：
- 密码保护
- 内容加密
- 元数据加密
- 完整性验证
- 安全策略管理

```go
// 创建安全选项
secOpts := &security.SecurityOptions{
    PasswordProtection: &security.PasswordProtection{
        Password: "my-secure-password-123",
        HashingIterations: 10000,
        KeyLength: 32,
        SaltLength: 16,
    },
    EncryptContent: true,
    EncryptionAlgorithm: security.AES256GCM,
    EncryptMetadata: true,
    EnableIntegrityChecks: true,
}

// 使用安全选项创建文件
f, err := fragmenta.CreateFragmenta("secure.frag", &fragmenta.FragmentaOptions{
    SecurityOptions: secOpts,
})
```

## 运行示例

从项目根目录运行：

```bash
# 运行单个示例
go run examples/main.go basic
go run examples/main.go query
go run examples/main.go storage
go run examples/main.go security

# 运行所有示例
go run examples/main.go all
```

## 注意事项

1. 示例会创建临时文件/目录用于演示
2. 每个示例开始时会清理上一次运行创建的文件
3. 安全性示例中的密码和加密仅用于演示
4. 这些示例优先展示API的用法，实际应用中应考虑错误处理和资源管理 