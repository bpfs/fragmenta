# TLV格式规范

TLV（Type-Length-Value）是一种灵活的编码格式，广泛应用于数据序列化、网络协议和二进制格式定义。在Fragmenta中，我们实现了完整的TLV编码和解码支持，为元数据存储、二进制协议和数据交换提供了高效的解决方案。

## 基本结构

TLV编码由三个部分组成：

- **Type（类型）**：一个字节，标识数据的类型
- **Length（长度）**：变长字段，表示Value部分的长度
- **Value（值）**：实际数据内容

### 类型定义

| 类型ID | 名称 | 说明 |
|--------|------|------|
| 0 | TLVTypeNull | 空值类型 |
| 1 | TLVTypeInt8 | 8位整数 |
| 2 | TLVTypeUint8 | 8位无符号整数 |
| 3 | TLVTypeInt16 | 16位整数 |
| 4 | TLVTypeUint16 | 16位无符号整数 |
| 5 | TLVTypeInt32 | 32位整数 |
| 6 | TLVTypeUint32 | 32位无符号整数 |
| 7 | TLVTypeInt64 | 64位整数 |
| 8 | TLVTypeUint64 | 64位无符号整数 |
| 9 | TLVTypeFloat32 | 32位浮点数 |
| 10 | TLVTypeFloat64 | 64位浮点数 |
| 11 | TLVTypeString | UTF-8字符串 |
| 12 | TLVTypeBytes | 字节数组 |
| 13 | TLVTypeBool | 布尔值 |
| 14 | TLVTypeArray | 数组 |
| 15 | TLVTypeMap | 映射 |
| 16 | TLVTypeCustom | 自定义类型 |

### 长度编码

为了支持不同大小的数据，长度字段采用变长编码：

- **短长度**：1字节，适用于长度<=255的数据
- **中长度**：2字节，适用于长度<=65535的数据
- **长长度**：4字节，适用于长度<=4294967295的数据
- **超长长度**：8字节，适用于更长的数据

长度格式标记占用一个字节，其中低3位用于标识长度类型：
- 0x00：短长度
- 0x01：中长度
- 0x02：长长度
- 0x03：超长长度

## API使用

### 基本类型编码

```go
// 整数编码
intValue := int64(12345)
intEncoded, err := fragmenta.EncodeTLVInt(intValue)

// 字符串编码
strValue := "Hello, TLV!"
strEncoded, err := fragmenta.EncodeTLVString(strValue)

// 布尔值编码
boolValue := true
boolEncoded, err := fragmenta.EncodeTLVBool(boolValue)
```

### 复合类型编码

```go
// 数组编码
arrayValue := []interface{}{123, "测试", true, 3.14}
arrayEncoded, err := fragmenta.EncodeTLVArray(arrayValue)

// 映射编码
mapValue := map[string]interface{}{
    "id": 1001,
    "name": "TLV示例",
    "enabled": true,
}
mapEncoded, err := fragmenta.EncodeTLVMap(mapValue)
```

### 解码

```go
// 解码TLV数据
decoded, err := fragmenta.DecodeTLV(bytes.NewReader(encodedData))
if err != nil {
    // 处理错误
}

// 获取解码后的值
value, err := fragmenta.DecodeTLVValue(decoded)
if err != nil {
    // 处理错误
}

// 根据类型处理值
switch v := value.(type) {
case int64:
    fmt.Printf("整数值: %d\n", v)
case string:
    fmt.Printf("字符串值: %s\n", v)
case []interface{}:
    fmt.Printf("数组值: %v\n", v)
case map[string]interface{}:
    fmt.Printf("映射值: %v\n", v)
}
```

## 应用场景

TLV格式在Fragmenta中有多种应用场景：

1. **元数据存储**：将复杂的元数据结构编码为TLV格式，方便存储和检索
2. **二进制协议**：定义高效的二进制通信协议，减少网络传输开销
3. **配置文件**：以二进制格式存储配置信息，提高解析效率
4. **跨语言数据交换**：作为不同语言间的数据交换格式
5. **版本兼容**：通过扩展TLV类型实现向后兼容的协议演进

## 性能特点

TLV格式具有以下性能特点：

- **紧凑高效**：编码后的数据占用空间小，适合网络传输和存储
- **自描述**：每个数据项包含类型和长度信息，便于解析
- **可扩展**：可以方便地添加新的数据类型而不影响已有的实现
- **灵活嵌套**：支持复杂的数据结构嵌套，如数组中包含映射等

## 最佳实践

1. **选择合适的类型**：整数类型会根据值的大小自动选择最紧凑的表示方式
2. **避免过深嵌套**：虽然支持任意深度的嵌套，但过深的嵌套会增加解析复杂度
3. **合理组织数据**：将相关的数据组织在同一个映射中，提高查询效率
4. **考虑前向兼容**：设计协议时预留扩展空间，便于将来添加新的字段

## 示例代码

请参考 `examples/tlv_examples/tlv_example.go` 获取完整的示例代码，展示了TLV格式在各种场景下的应用。 