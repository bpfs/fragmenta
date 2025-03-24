# FragDB (Fragmenta) 示例文档

本目录包含了FragDB存储引擎的使用示例和示范代码，帮助开发者快速上手和深入理解FragDB的各项功能。

## 目录结构

```
examples/
  ├── cmd/           - 示例程序入口目录
  │   └── main.go    - 主程序入口
  ├── basic/         - 基础功能示例，适合初学者
  ├── advanced/      - 高级功能示例，展示更复杂的用法
  ├── integration/   - 与其他系统集成的示例
  └── experimental/  - 实验性功能示例
      └── fuse/      - FUSE文件系统挂载功能(实验性)
          ├── linux/ - Linux系统FUSE挂载示例
          └── mac/   - macOS系统FUSE挂载示例
```

## 快速入门

如果您是首次接触FragDB，建议从基础示例开始：

1. `basic/basic_usage.go` - 基本的读写操作示例
2. `basic/metadata_ops.go` - 元数据操作示例
3. `basic/query_example.go` - 基本查询操作示例

## 目录内容说明

### 基础示例 (basic/)

- `basic_usage.go` - 基本的文件创建、打开、读写和关闭操作
- `metadata_ops.go` - 元数据的添加、修改、删除和查询
- `query_example.go` - 基本查询操作和过滤
- `storage_modes.go` - 不同存储模式的使用和切换
- `config_example.go` - 基本配置操作

### 高级示例 (advanced/)

- `query_examples.go` - 高级查询和复杂条件组合
- `security_example.go` - 使用安全特性(加密、签名等)
- `signature_example.go` - 数字签名和验证
- `acl_example.go` - 访问控制列表实现
- `hybrid_storage/` - 混合存储模式使用示例
- `dynamic_config/` - 动态配置更新示例
- `tlv_examples/` - TLV格式高级用法

### 集成示例 (integration/)

- `security_integration_example.go` - 安全框架与存储引擎集成
- `security_storage/` - 安全模块与存储的完整集成示例

### 实验性功能 (experimental/)

- `fuse/` - FUSE文件系统挂载功能(实验性研究功能)
  - `linux/` - Linux系统的FUSE挂载示例
  - `mac/` - macOS系统的FUSE挂载示例（需要手动安装macFUSE）

## 运行示例

运行统一示例程序：

```bash
# 从项目根目录运行
go run examples/cmd/main.go basic    # 运行基础示例
go run examples/cmd/main.go query    # 运行查询示例
go run examples/cmd/main.go storage  # 运行存储模式示例
go run examples/cmd/main.go security # 运行安全功能示例
go run examples/cmd/main.go all      # 运行所有基础示例

# 查看FUSE示例信息
go run examples/cmd/main.go fuse
```

FUSE示例需要单独运行：

```bash
# Linux FUSE示例
cd examples/experimental/fuse/linux
go build -o fuse-mount
./fuse-mount -mount /mnt/fragdb -storage data.frag -create

# macOS FUSE示例（需先安装macFUSE）
cd examples/experimental/fuse/mac
go build -o fuse-mount-mac
./fuse-mount-mac -mount /Volumes/fragdb -storage data.frag -create
```

## 示例代码使用须知

1. 示例代码主要用于演示API的使用方法，可能未包含完整的错误处理
2. 在生产环境中使用时，请确保添加适当的错误处理和资源清理
3. 实验性功能可能在未来版本中有较大变动，不建议在生产环境中使用
4. 部分示例中引用的API可能尚未完全实现，仅作为参考

## 贡献示例

欢迎贡献新的示例代码！如果您有好的用例或用法演示，请参考现有示例的格式提交PR。 