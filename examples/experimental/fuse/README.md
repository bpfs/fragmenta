# FragDB FUSE挂载功能 (实验性)

本目录包含将FragDB存储引擎挂载为文件系统的实验性功能示例。通过FUSE（用户空间文件系统）技术，可以将FragDB存储引擎的内容作为标准文件系统进行访问。

## 注意事项

⚠️ **警告**: 此功能是实验性的，仅用于演示和研究目的。在生产环境中使用前，请充分测试。

- 当前实现是模拟的，不会实际挂载文件系统
- 需要用户手动安装FUSE相关依赖
- 在macOS上需安装macFUSE
- 在Linux上需安装libfuse-dev

## 目录结构

- `common/` - 共享类型和函数
- `linux/` - Linux平台的FUSE挂载示例
- `mac/` - macOS平台的FUSE挂载示例

## 使用方法

### Linux

```bash
cd examples/experimental/fuse/linux
go build -o fragmenta-fuse
./fragmenta-fuse --mount /mnt/fragmenta --storage /path/to/storage.frag --create
```

参数说明:
- `--mount` - 设置挂载点路径 (默认: /tmp/fragmenta-mount)
- `--storage` - 设置存储文件路径 (默认: fuse-example.frag)
- `--create` - 创建新的存储文件 (如果不存在)
- `--debug` - 启用调试模式

### macOS

```bash
cd examples/experimental/fuse/mac
go build -o fragmenta-fuse-mac
./fragmenta-fuse-mac --mount /Volumes/fragmenta --storage /path/to/storage.frag --create
```

参数说明:
- `--mount` - 设置挂载点路径 (默认: /Volumes/fragdb)
- `--storage` - 设置存储文件路径 (默认: fuse-example-mac.frag)
- `--create` - 创建新的存储文件 (如果不存在)
- `--debug` - 启用调试模式
- `--volname` - 设置卷名称 (默认: FragDB存储)

## macOS 安装 macFUSE

macOS用户需要安装macFUSE才能使用FUSE功能:

1. 访问 https://github.com/osxfuse/osxfuse/releases 下载最新版本
2. 或使用Homebrew安装: `brew install --cask macfuse`
3. 安装后可能需要重启系统
4. macOS可能会要求在"系统偏好设置"中允许内核扩展

## 实现说明

当前的FUSE功能是实验性的，主要实现了:

1. 基本的文件和目录操作
2. 与FragDB存储引擎的集成
3. 平台特定的挂载处理

未来计划:
- 完善文件和目录操作
- 优化性能和稳定性
- 增加更多高级功能

## 故障排除

如果遇到挂载失败:

1. 确保已安装FUSE依赖
2. 检查挂载点是否存在且有适当权限
3. 尝试手动卸载: `umount <挂载点>`
4. 在macOS上，可以在Finder中右键点击卷图标选择"推出"

## 开发指南

要扩展FUSE功能:

1. 在`common/fuse_types.go`中添加新的选项或方法
2. 实现平台特定的挂载逻辑
3. 优化文件系统操作性能

## 已知限制

- 性能可能不如原生文件系统
- 不支持某些特殊的文件操作
- 大文件的读写效率可能较低
- 需要外部依赖(macFUSE/libfuse)

## 相关资源

- [FUSE项目](https://github.com/libfuse/libfuse)
- [macFUSE](https://osxfuse.github.io/)
- [Go-FUSE库](https://github.com/hanwen/go-fuse) 