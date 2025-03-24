package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/bpfs/fragmenta"
)

// 演示TLV格式使用的主函数
func main() {
	// 基本类型编码和解码示例
	fmt.Println("=== 基本类型编码和解码示例 ===")
	basicTypesExample()

	// 复合类型编码和解码示例
	fmt.Println("\n=== 复合类型编码和解码示例 ===")
	compositeTypesExample()

	// TLV在元数据中的应用示例
	fmt.Println("\n=== TLV在元数据中的应用示例 ===")
	metadataExample()

	// TLV在二进制协议中的应用示例
	fmt.Println("\n=== TLV在二进制协议中的应用示例 ===")
	protocolExample()
}

// 演示基本类型的编码和解码
func basicTypesExample() {
	// 编码整数
	intValue := int64(12345)
	intEncoded, err := fragmenta.EncodeTLVInt(intValue)
	if err != nil {
		log.Fatalf("整数编码失败: %v", err)
	}
	fmt.Printf("整数 %d 编码后的长度: %d 字节\n", intValue, len(intEncoded))

	// 解码整数
	intDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(intEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	intResult, err := fragmenta.DecodeTLVValue(intDecoded)
	if err != nil {
		log.Fatalf("整数值解码失败: %v", err)
	}
	fmt.Printf("解码后的整数: %v (类型: %T)\n", intResult, intResult)

	// 编码字符串
	strValue := "这是一个TLV格式的字符串示例"
	strEncoded, err := fragmenta.EncodeTLVString(strValue)
	if err != nil {
		log.Fatalf("字符串编码失败: %v", err)
	}
	fmt.Printf("字符串编码后的长度: %d 字节\n", len(strEncoded))

	// 解码字符串
	strDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(strEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	strResult, err := fragmenta.DecodeTLVValue(strDecoded)
	if err != nil {
		log.Fatalf("字符串值解码失败: %v", err)
	}
	fmt.Printf("解码后的字符串: %v\n", strResult)

	// 编码布尔值
	boolValue := true
	boolEncoded, err := fragmenta.EncodeTLVBool(boolValue)
	if err != nil {
		log.Fatalf("布尔值编码失败: %v", err)
	}
	fmt.Printf("布尔值编码后的长度: %d 字节\n", len(boolEncoded))

	// 解码布尔值
	boolDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(boolEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	boolResult, err := fragmenta.DecodeTLVValue(boolDecoded)
	if err != nil {
		log.Fatalf("布尔值解码失败: %v", err)
	}
	fmt.Printf("解码后的布尔值: %v\n", boolResult)
}

// 演示复合类型的编码和解码
func compositeTypesExample() {
	// 创建一个数组
	arrayValue := []interface{}{
		123,
		"Hello, TLV!",
		true,
		3.14159,
	}

	// 编码数组
	arrayEncoded, err := fragmenta.EncodeTLVArray(arrayValue)
	if err != nil {
		log.Fatalf("数组编码失败: %v", err)
	}
	fmt.Printf("数组编码后的长度: %d 字节\n", len(arrayEncoded))

	// 解码数组
	arrayDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(arrayEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	arrayResult, err := fragmenta.DecodeTLVValue(arrayDecoded)
	if err != nil {
		log.Fatalf("数组值解码失败: %v", err)
	}
	fmt.Printf("解码后的数组: %v\n", arrayResult)

	// 创建一个映射
	mapValue := map[string]interface{}{
		"id":      1001,
		"name":    "TLV示例对象",
		"enabled": true,
		"tags":    []interface{}{"示例", "TLV", "测试"},
	}

	// 编码映射
	mapEncoded, err := fragmenta.EncodeTLVMap(mapValue)
	if err != nil {
		log.Fatalf("映射编码失败: %v", err)
	}
	fmt.Printf("映射编码后的长度: %d 字节\n", len(mapEncoded))

	// 解码映射
	mapDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(mapEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	mapResult, err := fragmenta.DecodeTLVValue(mapDecoded)
	if err != nil {
		log.Fatalf("映射值解码失败: %v", err)
	}
	fmt.Printf("解码后的映射: %v\n", mapResult)

	// 访问映射中的值
	if m, ok := mapResult.(map[string]interface{}); ok {
		fmt.Printf("映射中的name字段: %v\n", m["name"])
		if tags, ok := m["tags"].([]interface{}); ok {
			fmt.Printf("映射中的tags: %v\n", tags)
		}
	}
}

// 演示TLV在元数据中的应用
func metadataExample() {
	// 创建复杂的元数据结构
	metadata := map[string]interface{}{
		"title":    "TLV格式示例文档",
		"version":  1.0,
		"created":  1620000000,
		"author":   "Fragmenta开发团队",
		"is_draft": false,
		"keywords": []interface{}{"TLV", "编码", "格式化", "元数据"},
		"permissions": map[string]interface{}{
			"read":  []interface{}{"admin", "user"},
			"write": []interface{}{"admin"},
		},
	}

	// 编码元数据为TLV格式
	metadataEncoded, err := fragmenta.EncodeTLVMap(metadata)
	if err != nil {
		log.Fatalf("元数据编码失败: %v", err)
	}
	fmt.Printf("元数据编码后的长度: %d 字节\n", len(metadataEncoded))

	// 解码元数据
	metadataDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(metadataEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	metadataResult, err := fragmenta.DecodeTLVValue(metadataDecoded)
	if err != nil {
		log.Fatalf("元数据值解码失败: %v", err)
	}

	// 访问元数据中的值
	if m, ok := metadataResult.(map[string]interface{}); ok {
		fmt.Printf("文档标题: %v\n", m["title"])
		fmt.Printf("文档版本: %v\n", m["version"])
		fmt.Printf("文档作者: %v\n", m["author"])

		if permissions, ok := m["permissions"].(map[string]interface{}); ok {
			if readPerms, ok := permissions["read"].([]interface{}); ok {
				fmt.Printf("读取权限: %v\n", readPerms)
			}
		}
	}
}

// 演示TLV在二进制协议中的应用
func protocolExample() {
	// 创建一个请求消息
	requestMsg := map[string]interface{}{
		"msg_type":   "request",
		"cmd":        "get_data",
		"request_id": 12345,
		"timestamp":  1620100000,
		"params": map[string]interface{}{
			"dataType": "user",
			"id":       1001,
			"fields":   []interface{}{"name", "email", "role"},
		},
	}

	// 编码请求消息
	requestEncoded, err := fragmenta.EncodeTLVMap(requestMsg)
	if err != nil {
		log.Fatalf("请求消息编码失败: %v", err)
	}
	fmt.Printf("请求消息编码后的长度: %d 字节\n", len(requestEncoded))

	// 模拟网络传输...

	// 解码请求消息
	requestDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(requestEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	requestResult, err := fragmenta.DecodeTLVValue(requestDecoded)
	if err != nil {
		log.Fatalf("请求消息值解码失败: %v", err)
	}

	// 处理请求
	fmt.Println("收到请求:")
	if req, ok := requestResult.(map[string]interface{}); ok {
		fmt.Printf("  消息类型: %v\n", req["msg_type"])
		fmt.Printf("  命令: %v\n", req["cmd"])
		fmt.Printf("  请求ID: %v\n", req["request_id"])

		if params, ok := req["params"].(map[string]interface{}); ok {
			fmt.Printf("  数据类型: %v\n", params["dataType"])
			fmt.Printf("  ID: %v\n", params["id"])
			if fields, ok := params["fields"].([]interface{}); ok {
				fmt.Printf("  字段: %v\n", fields)
			}
		}
	}

	// 创建响应消息
	responseMsg := map[string]interface{}{
		"msg_type":   "response",
		"request_id": 12345,
		"timestamp":  1620100001,
		"status":     "success",
		"data": map[string]interface{}{
			"id":    1001,
			"name":  "测试用户",
			"email": "test@example.com",
			"role":  "admin",
		},
	}

	// 编码响应消息
	responseEncoded, err := fragmenta.EncodeTLVMap(responseMsg)
	if err != nil {
		log.Fatalf("响应消息编码失败: %v", err)
	}
	fmt.Printf("响应消息编码后的长度: %d 字节\n", len(responseEncoded))

	// 模拟网络传输...

	// 解码响应消息
	responseDecoded, err := fragmenta.DecodeTLV(bytes.NewReader(responseEncoded))
	if err != nil {
		log.Fatalf("TLV解码失败: %v", err)
	}
	responseResult, err := fragmenta.DecodeTLVValue(responseDecoded)
	if err != nil {
		log.Fatalf("响应消息值解码失败: %v", err)
	}

	// 处理响应
	fmt.Println("收到响应:")
	if resp, ok := responseResult.(map[string]interface{}); ok {
		fmt.Printf("  消息类型: %v\n", resp["msg_type"])
		fmt.Printf("  请求ID: %v\n", resp["request_id"])
		fmt.Printf("  状态: %v\n", resp["status"])

		if data, ok := resp["data"].(map[string]interface{}); ok {
			fmt.Printf("  用户数据: 名称=%v, 邮箱=%v, 角色=%v\n",
				data["name"], data["email"], data["role"])
		}
	}
}
