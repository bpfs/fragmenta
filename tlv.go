package fragmenta

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// TLV常量定义
const (
	// TLV类型定义
	TLVTypeNull    uint8 = 0  // 空类型
	TLVTypeInt8    uint8 = 1  // 8位整数
	TLVTypeUint8   uint8 = 2  // 8位无符号整数
	TLVTypeInt16   uint8 = 3  // 16位整数
	TLVTypeUint16  uint8 = 4  // 16位无符号整数
	TLVTypeInt32   uint8 = 5  // 32位整数
	TLVTypeUint32  uint8 = 6  // 32位无符号整数
	TLVTypeInt64   uint8 = 7  // 64位整数
	TLVTypeUint64  uint8 = 8  // 64位无符号整数
	TLVTypeFloat32 uint8 = 9  // 32位浮点数
	TLVTypeFloat64 uint8 = 10 // 64位浮点数
	TLVTypeString  uint8 = 11 // UTF-8字符串
	TLVTypeBytes   uint8 = 12 // 字节数组
	TLVTypeBool    uint8 = 13 // 布尔值
	TLVTypeArray   uint8 = 14 // 数组
	TLVTypeMap     uint8 = 15 // 映射
	TLVTypeCustom  uint8 = 16 // 自定义类型

	// 长度编码常量
	TLVLenMask     uint8 = 0x07 // 长度掩码
	TLVLenShort    uint8 = 0x00 // 短长度(1字节)
	TLVLenMedium   uint8 = 0x01 // 中长度(2字节)
	TLVLenLong     uint8 = 0x02 // 长长度(4字节)
	TLVLenVeryLong uint8 = 0x03 // 超长长度(8字节)

	// 最大长度限制
	TLVMaxShortLen  uint8  = 0xFF       // 短长度最大值
	TLVMaxMediumLen uint16 = 0xFFFF     // 中长度最大值
	TLVMaxLongLen   uint32 = 0xFFFFFFFF // 长长度最大值
)

// 错误定义
var (
	ErrInvalidTLVType   = errors.New("无效的TLV类型")
	ErrInvalidTLVLength = errors.New("无效的TLV长度")
	ErrTLVDataTooLarge  = errors.New("TLV数据太大")
	ErrTLVReadFailed    = errors.New("TLV读取失败")
	ErrTLVWriteFailed   = errors.New("TLV写入失败")
)

// TLVHeader TLV头部结构
type TLVHeader struct {
	Type   uint8
	Length uint64
}

// TLVValue TLV值接口
type TLVValue interface {
	Type() uint8
	Bytes() []byte
	String() string
}

// TLVItem 完整的TLV项
type TLVItem struct {
	Header TLVHeader
	Value  []byte
}

// 编码TLV头部
func EncodeTLVHeader(t uint8, length uint64) ([]byte, error) {
	var buf bytes.Buffer

	// 写入类型
	if err := buf.WriteByte(t); err != nil {
		return nil, fmt.Errorf("写入TLV类型失败: %w", err)
	}

	// 确定长度编码格式
	var lenFormat uint8
	if length <= uint64(TLVMaxShortLen) {
		lenFormat = TLVLenShort
	} else if length <= uint64(TLVMaxMediumLen) {
		lenFormat = TLVLenMedium
	} else if length <= uint64(TLVMaxLongLen) {
		lenFormat = TLVLenLong
	} else {
		lenFormat = TLVLenVeryLong
	}

	// 写入长度格式标记
	if err := buf.WriteByte(lenFormat); err != nil {
		return nil, fmt.Errorf("写入长度格式失败: %w", err)
	}

	// 根据不同长度格式写入实际长度
	switch lenFormat {
	case TLVLenShort:
		if err := buf.WriteByte(uint8(length)); err != nil {
			return nil, fmt.Errorf("写入短长度失败: %w", err)
		}
	case TLVLenMedium:
		if err := binary.Write(&buf, binary.LittleEndian, uint16(length)); err != nil {
			return nil, fmt.Errorf("写入中长度失败: %w", err)
		}
	case TLVLenLong:
		if err := binary.Write(&buf, binary.LittleEndian, uint32(length)); err != nil {
			return nil, fmt.Errorf("写入长长度失败: %w", err)
		}
	case TLVLenVeryLong:
		if err := binary.Write(&buf, binary.LittleEndian, length); err != nil {
			return nil, fmt.Errorf("写入超长长度失败: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// 解码TLV头部
func DecodeTLVHeader(r io.Reader) (TLVHeader, error) {
	var header TLVHeader

	// 读取类型
	typeBytes := make([]byte, 1)
	if _, err := io.ReadFull(r, typeBytes); err != nil {
		return header, fmt.Errorf("读取TLV类型失败: %w", err)
	}
	header.Type = typeBytes[0]

	// 读取长度格式
	lenFormatBytes := make([]byte, 1)
	if _, err := io.ReadFull(r, lenFormatBytes); err != nil {
		return header, fmt.Errorf("读取长度格式失败: %w", err)
	}
	lenFormat := lenFormatBytes[0] & TLVLenMask

	// 根据不同长度格式读取实际长度
	switch lenFormat {
	case TLVLenShort:
		lengthBytes := make([]byte, 1)
		if _, err := io.ReadFull(r, lengthBytes); err != nil {
			return header, fmt.Errorf("读取短长度失败: %w", err)
		}
		header.Length = uint64(lengthBytes[0])
	case TLVLenMedium:
		var length uint16
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return header, fmt.Errorf("读取中长度失败: %w", err)
		}
		header.Length = uint64(length)
	case TLVLenLong:
		var length uint32
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return header, fmt.Errorf("读取长长度失败: %w", err)
		}
		header.Length = uint64(length)
	case TLVLenVeryLong:
		var length uint64
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return header, fmt.Errorf("读取超长长度失败: %w", err)
		}
		header.Length = length
	default:
		return header, ErrInvalidTLVLength
	}

	return header, nil
}

// 编码完整的TLV项
func EncodeTLV(t uint8, data []byte) ([]byte, error) {
	// 编码头部
	header, err := EncodeTLVHeader(t, uint64(len(data)))
	if err != nil {
		return nil, err
	}

	// 组合头部和数据
	result := make([]byte, len(header)+len(data))
	copy(result, header)
	copy(result[len(header):], data)

	return result, nil
}

// 解码完整的TLV项
func DecodeTLV(r io.Reader) (TLVItem, error) {
	var item TLVItem

	// 解码头部
	header, err := DecodeTLVHeader(r)
	if err != nil {
		return item, err
	}
	item.Header = header

	// 读取值
	value := make([]byte, header.Length)
	if _, err := io.ReadFull(r, value); err != nil {
		return item, fmt.Errorf("读取TLV值失败: %w", err)
	}
	item.Value = value

	return item, nil
}

// 编码整数类型
func EncodeTLVInt(value int64) ([]byte, error) {
	var t uint8
	var data []byte

	// 根据值的大小选择最小的整数类型
	switch {
	case value >= -128 && value <= 127:
		t = TLVTypeInt8
		data = []byte{byte(value)}
	case value >= -32768 && value <= 32767:
		t = TLVTypeInt16
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, int16(value))
		data = buf.Bytes()
	case value >= -2147483648 && value <= 2147483647:
		t = TLVTypeInt32
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, int32(value))
		data = buf.Bytes()
	default:
		t = TLVTypeInt64
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, value)
		data = buf.Bytes()
	}

	return EncodeTLV(t, data)
}

// 编码无符号整数类型
func EncodeTLVUint(value uint64) ([]byte, error) {
	var t uint8
	var data []byte

	// 根据值的大小选择最小的整数类型
	switch {
	case value <= 255:
		t = TLVTypeUint8
		data = []byte{byte(value)}
	case value <= 65535:
		t = TLVTypeUint16
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, uint16(value))
		data = buf.Bytes()
	case value <= 4294967295:
		t = TLVTypeUint32
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, uint32(value))
		data = buf.Bytes()
	default:
		t = TLVTypeUint64
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, value)
		data = buf.Bytes()
	}

	return EncodeTLV(t, data)
}

// 编码浮点数类型
func EncodeTLVFloat(value float64) ([]byte, error) {
	var t uint8
	var data []byte

	// 确定是否可以使用32位浮点数
	float32Value := float32(value)
	if float64(float32Value) == value {
		t = TLVTypeFloat32
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, float32Value)
		data = buf.Bytes()
	} else {
		t = TLVTypeFloat64
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, value)
		data = buf.Bytes()
	}

	return EncodeTLV(t, data)
}

// 编码字符串类型
func EncodeTLVString(value string) ([]byte, error) {
	return EncodeTLV(TLVTypeString, []byte(value))
}

// 编码字节数组类型
func EncodeTLVBytes(value []byte) ([]byte, error) {
	return EncodeTLV(TLVTypeBytes, value)
}

// 编码布尔类型
func EncodeTLVBool(value bool) ([]byte, error) {
	var data byte
	if value {
		data = 1
	} else {
		data = 0
	}
	return EncodeTLV(TLVTypeBool, []byte{data})
}

// 编码数组类型
func EncodeTLVArray(values []interface{}) ([]byte, error) {
	// 先编码所有元素
	var itemsData bytes.Buffer
	for _, v := range values {
		var encodedItem []byte
		var err error

		switch val := v.(type) {
		case nil:
			encodedItem, err = EncodeTLV(TLVTypeNull, []byte{})
		case int, int8, int16, int32, int64:
			var intVal int64
			switch i := val.(type) {
			case int:
				intVal = int64(i)
			case int8:
				intVal = int64(i)
			case int16:
				intVal = int64(i)
			case int32:
				intVal = int64(i)
			case int64:
				intVal = i
			}
			encodedItem, err = EncodeTLVInt(intVal)
		case uint, uint8, uint16, uint32, uint64:
			var uintVal uint64
			switch i := val.(type) {
			case uint:
				uintVal = uint64(i)
			case uint8:
				uintVal = uint64(i)
			case uint16:
				uintVal = uint64(i)
			case uint32:
				uintVal = uint64(i)
			case uint64:
				uintVal = i
			}
			encodedItem, err = EncodeTLVUint(uintVal)
		case float32, float64:
			var floatVal float64
			switch f := val.(type) {
			case float32:
				floatVal = float64(f)
			case float64:
				floatVal = f
			}
			encodedItem, err = EncodeTLVFloat(floatVal)
		case bool:
			encodedItem, err = EncodeTLVBool(val)
		case string:
			encodedItem, err = EncodeTLVString(val)
		case []byte:
			encodedItem, err = EncodeTLVBytes(val)
		case []interface{}:
			encodedItem, err = EncodeTLVArray(val)
		case map[string]interface{}:
			encodedItem, err = EncodeTLVMap(val)
		default:
			return nil, fmt.Errorf("不支持的数组元素类型: %T", val)
		}

		if err != nil {
			return nil, err
		}
		if _, err := itemsData.Write(encodedItem); err != nil {
			return nil, err
		}
	}

	// 将所有编码后的元素作为TLVTypeArray类型的数据
	return EncodeTLV(TLVTypeArray, itemsData.Bytes())
}

// 编码映射类型
func EncodeTLVMap(values map[string]interface{}) ([]byte, error) {
	// 先编码所有键值对
	var itemsData bytes.Buffer
	for k, v := range values {
		// 编码键（键始终是字符串类型）
		keyEncoded, err := EncodeTLVString(k)
		if err != nil {
			return nil, err
		}
		if _, err := itemsData.Write(keyEncoded); err != nil {
			return nil, err
		}

		// 编码值（根据类型）
		var valueEncoded []byte
		switch val := v.(type) {
		case nil:
			valueEncoded, err = EncodeTLV(TLVTypeNull, []byte{})
		case int, int8, int16, int32, int64:
			var intVal int64
			switch i := val.(type) {
			case int:
				intVal = int64(i)
			case int8:
				intVal = int64(i)
			case int16:
				intVal = int64(i)
			case int32:
				intVal = int64(i)
			case int64:
				intVal = i
			}
			valueEncoded, err = EncodeTLVInt(intVal)
		case uint, uint8, uint16, uint32, uint64:
			var uintVal uint64
			switch i := val.(type) {
			case uint:
				uintVal = uint64(i)
			case uint8:
				uintVal = uint64(i)
			case uint16:
				uintVal = uint64(i)
			case uint32:
				uintVal = uint64(i)
			case uint64:
				uintVal = i
			}
			valueEncoded, err = EncodeTLVUint(uintVal)
		case float32, float64:
			var floatVal float64
			switch f := val.(type) {
			case float32:
				floatVal = float64(f)
			case float64:
				floatVal = f
			}
			valueEncoded, err = EncodeTLVFloat(floatVal)
		case bool:
			valueEncoded, err = EncodeTLVBool(val)
		case string:
			valueEncoded, err = EncodeTLVString(val)
		case []byte:
			valueEncoded, err = EncodeTLVBytes(val)
		case []interface{}:
			valueEncoded, err = EncodeTLVArray(val)
		case map[string]interface{}:
			valueEncoded, err = EncodeTLVMap(val)
		default:
			return nil, fmt.Errorf("不支持的映射值类型: %T", val)
		}

		if err != nil {
			return nil, err
		}
		if _, err := itemsData.Write(valueEncoded); err != nil {
			return nil, err
		}
	}

	// 将所有编码后的键值对作为TLVTypeMap类型的数据
	return EncodeTLV(TLVTypeMap, itemsData.Bytes())
}

// 解码TLV值
func DecodeTLVValue(item TLVItem) (interface{}, error) {
	switch item.Header.Type {
	case TLVTypeNull:
		return nil, nil
	case TLVTypeInt8:
		if len(item.Value) != 1 {
			return nil, ErrInvalidTLVLength
		}
		return int8(item.Value[0]), nil
	case TLVTypeUint8:
		if len(item.Value) != 1 {
			return nil, ErrInvalidTLVLength
		}
		return item.Value[0], nil
	case TLVTypeInt16:
		if len(item.Value) != 2 {
			return nil, ErrInvalidTLVLength
		}
		var value int16
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeUint16:
		if len(item.Value) != 2 {
			return nil, ErrInvalidTLVLength
		}
		var value uint16
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeInt32:
		if len(item.Value) != 4 {
			return nil, ErrInvalidTLVLength
		}
		var value int32
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeUint32:
		if len(item.Value) != 4 {
			return nil, ErrInvalidTLVLength
		}
		var value uint32
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeInt64:
		if len(item.Value) != 8 {
			return nil, ErrInvalidTLVLength
		}
		var value int64
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeUint64:
		if len(item.Value) != 8 {
			return nil, ErrInvalidTLVLength
		}
		var value uint64
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeFloat32:
		if len(item.Value) != 4 {
			return nil, ErrInvalidTLVLength
		}
		var value float32
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeFloat64:
		if len(item.Value) != 8 {
			return nil, ErrInvalidTLVLength
		}
		var value float64
		if err := binary.Read(bytes.NewReader(item.Value), binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case TLVTypeString:
		return string(item.Value), nil
	case TLVTypeBytes:
		return item.Value, nil
	case TLVTypeBool:
		if len(item.Value) != 1 {
			return nil, ErrInvalidTLVLength
		}
		return item.Value[0] != 0, nil
	case TLVTypeArray:
		return DecodeTLVArray(item.Value)
	case TLVTypeMap:
		return DecodeTLVMap(item.Value)
	default:
		return nil, ErrInvalidTLVType
	}
}

// 解码TLV数组
func DecodeTLVArray(data []byte) ([]interface{}, error) {
	r := bytes.NewReader(data)
	result := []interface{}{}

	for r.Len() > 0 {
		item, err := DecodeTLV(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		value, err := DecodeTLVValue(item)
		if err != nil {
			return nil, err
		}

		result = append(result, value)
	}

	return result, nil
}

// 解码TLV映射
func DecodeTLVMap(data []byte) (map[string]interface{}, error) {
	r := bytes.NewReader(data)
	result := make(map[string]interface{})

	for r.Len() > 0 {
		// 读取键
		keyItem, err := DecodeTLV(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if keyItem.Header.Type != TLVTypeString {
			return nil, fmt.Errorf("映射的键必须是字符串类型，收到了类型 %d", keyItem.Header.Type)
		}

		key, err := DecodeTLVValue(keyItem)
		if err != nil {
			return nil, err
		}

		keyStr, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("映射的键解码错误，期望字符串，得到 %T", key)
		}

		// 确保还有值可以读取
		if r.Len() <= 0 {
			return nil, fmt.Errorf("映射中的键 '%s' 没有对应的值", keyStr)
		}

		// 读取值
		valueItem, err := DecodeTLV(r)
		if err != nil {
			return nil, err
		}

		value, err := DecodeTLVValue(valueItem)
		if err != nil {
			return nil, err
		}

		result[keyStr] = value
	}

	return result, nil
}
