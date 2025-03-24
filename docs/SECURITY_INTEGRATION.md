# 存储引擎与安全模块集成

本文档描述了存储引擎与安全模块的集成设计和实现。

## 1. 设计目标

集成存储引擎与安全模块的主要目标是：

1. 为存储的数据提供加密保护，确保敏感数据的安全
2. 保持现有存储功能的完整性，不破坏已有功能
3. 提供灵活的配置，允许用户根据需求开启或关闭加密功能
4. 支持块级别的加密和解密
5. 为未来的访问控制功能提供基础

## 2. 架构设计

集成架构采用了模块化设计，主要包含以下组件：

1. **存储管理器接口扩展**：在`StorageManager`接口中添加了安全相关方法
2. **存储管理器实现增强**：在`StorageManagerImpl`中添加了安全管理器字段和相关实现
3. **安全适配器**：提供了存储与安全模块之间的适配层，简化集成使用
4. **测试和示例**：为集成功能提供了全面的测试和使用示例

### 2.1 接口扩展

在`StorageManager`接口中，添加了以下安全相关方法：

```go
// 安全相关功能
SetSecurityManager(securityManager interface{}) error
IsEncryptionEnabled() bool
SetEncryptionEnabled(enabled bool) error
EncryptBlock(id uint32, data []byte) ([]byte, error)
DecryptBlock(id uint32, data []byte) ([]byte, error)
```

### 2.2 存储管理器实现增强

在`StorageManagerImpl`实现中：

1. 添加了`securityManager`字段和`encryptionEnabled`标志
2. 实现了安全相关方法
3. 增强了`WriteBlock`和`ReadBlock`方法，支持数据加密和解密

### 2.3 安全适配器

创建了`StorageSecurityAdapter`，提供了：

1. 存储管理器和安全管理器的集成管理
2. 配置选项的统一管理
3. 工厂方法创建具有安全功能的存储管理器
4. 简化的API用于启用/禁用加密

## 3. 实现细节

### 3.1 数据加密流程

数据写入流程：
1. 检查加密是否启用及安全管理器是否设置
2. 如果启用加密，调用`EncryptBlock`方法加密数据
3. 将加密后的数据写入底层存储

数据读取流程：
1. 从底层存储读取数据
2. 检查加密是否启用及安全管理器是否设置
3. 如果启用加密，调用`DecryptBlock`方法解密数据
4. 返回解密后的原始数据

### 3.2 密钥管理

安全模块负责密钥的生成、存储和管理：
1. 支持主密钥和派生密钥
2. 密钥存储在安全存储中
3. 使用标准加密算法（如AES-256-GCM）

## 4. 使用示例

基本使用流程：

```go
// 1. 创建存储管理器
storageConfig := &storage.StorageConfig{...}
storageManager, _ := storage.NewStorageManager(storageConfig)

// 2. 创建安全管理器
secConfig := &security.SecurityConfig{...}
securityManager, _ := security.NewDefaultSecurityManager(secConfig)

// 3. 初始化安全管理器
securityManager.Initialize(context.Background())

// 4. 集成安全与存储
storageManager.SetSecurityManager(securityManager)
storageManager.SetEncryptionEnabled(true)

// 5. 使用加密存储
storageManager.WriteBlock(blockID, data)  // 数据将自动加密
readData, _ := storageManager.ReadBlock(blockID)  // 数据将自动解密
```

更详细的示例请参考`examples/security_storage/main.go`。

## 5. 测试

集成测试验证了以下功能：
1. 加密数据写入和读取
2. 加密启用/禁用切换
3. 混合模式（部分加密、部分未加密）
4. 手动加解密功能

## 6. 未来扩展

计划中的后续功能包括：
1. 访问控制和权限管理
2. 支持更多加密算法选项
3. 密钥轮换机制
4. 块级别的差异化加密策略

## 7. 注意事项

1. 启用加密会增加CPU使用和存储开销
2. 加密数据稍大于原始数据（因为包含加密元数据）
3. 密钥管理至关重要，密钥丢失将导致数据不可恢复 