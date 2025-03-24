package fragmenta

import (
	"os"
	"testing"
)

// 测试创建和打开Fragmenta格式文件
func TestCreateAndOpenFragmenta(t *testing.T) {
	// 创建临时文件
	tempFile, err := os.CreateTemp("", "fragdb-test-*.bin")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tempFile.Close()

	tempPath := tempFile.Name()

	// 测试完成后删除临时文件
	defer os.Remove(tempPath)

	// 创建Fragmenta格式文件
	options := &FragmentaOptions{
		StorageMode:       ContainerMode,
		BlockSize:         DefaultBlockSize,
		IndexUpdateMode:   IndexUpdateRealtime,
		MaxIndexCacheSize: DefaultIndexCacheSize,
	}

	fragmenta, err := CreateFragmenta(tempPath, options)
	if err != nil {
		t.Fatalf("创建Fragmenta格式文件失败: %v", err)
	}

	// 设置一些元数据
	err = fragmenta.SetMetadata(TagTitle, []byte("测试文件"))
	if err != nil {
		t.Fatalf("设置元数据失败: %v", err)
	}

	// 提交更改
	err = fragmenta.Commit()
	if err != nil {
		t.Fatalf("提交更改失败: %v", err)
	}

	// 关闭文件
	err = fragmenta.Close()
	if err != nil {
		t.Fatalf("关闭文件失败: %v", err)
	}

	// 重新打开文件
	fragmenta, err = OpenFragmenta(tempPath)
	if err != nil {
		t.Fatalf("打开Fragmenta格式文件失败: %v", err)
	}

	// 读取元数据
	title, err := fragmenta.GetMetadata(TagTitle)
	if err != nil {
		t.Fatalf("读取元数据失败: %v", err)
	}

	if string(title) != "测试文件" {
		t.Fatalf("元数据不匹配: 期望 '测试文件', 实际 '%s'", string(title))
	}

	// 关闭文件
	err = fragmenta.Close()
	if err != nil {
		t.Fatalf("关闭文件失败: %v", err)
	}
}

// 测试元数据批量操作
func TestBatchMetadataOperation(t *testing.T) {
	// 创建临时文件
	tempFile, err := os.CreateTemp("", "fragdb-test-*.bin")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tempFile.Close()

	tempPath := tempFile.Name()

	// 测试完成后删除临时文件
	defer os.Remove(tempPath)

	// 创建Fragmenta格式文件
	fragmenta, err := CreateFragmenta(tempPath, nil)
	if err != nil {
		t.Fatalf("创建Fragmenta格式文件失败: %v", err)
	}

	// 创建批量操作
	batch := &BatchMetadataOperation{
		Operations: []MetadataOperation{
			{
				Operation: 0, // 设置
				Tag:       TagTitle,
				Value:     []byte("测试标题"),
			},
			{
				Operation: 0, // 设置
				Tag:       TagDescription,
				Value:     []byte("这是一个测试描述"),
			},
			{
				Operation: 0, // 设置
				Tag:       UserTag(0x1001),
				Value:     EncodeInt64(42),
			},
		},
		AtomicExec:      true,
		RollbackOnError: true,
	}

	// 执行批量操作
	err = fragmenta.BatchMetadataOp(batch)
	if err != nil {
		t.Fatalf("批量元数据操作失败: %v", err)
	}

	// 提交更改
	err = fragmenta.Commit()
	if err != nil {
		t.Fatalf("提交更改失败: %v", err)
	}

	// 读取元数据
	title, err := fragmenta.GetMetadata(TagTitle)
	if err != nil {
		t.Fatalf("读取标题元数据失败: %v", err)
	}

	if string(title) != "测试标题" {
		t.Fatalf("标题元数据不匹配: 期望 '测试标题', 实际 '%s'", string(title))
	}

	desc, err := fragmenta.GetMetadata(TagDescription)
	if err != nil {
		t.Fatalf("读取描述元数据失败: %v", err)
	}

	if string(desc) != "这是一个测试描述" {
		t.Fatalf("描述元数据不匹配: 期望 '这是一个测试描述', 实际 '%s'", string(desc))
	}

	customData, err := fragmenta.GetMetadata(UserTag(0x1001))
	if err != nil {
		t.Fatalf("读取自定义元数据失败: %v", err)
	}

	if DecodeInt64(customData) != 42 {
		t.Fatalf("自定义元数据不匹配: 期望 42, 实际 %d", DecodeInt64(customData))
	}

	// 关闭文件
	err = fragmenta.Close()
	if err != nil {
		t.Fatalf("关闭文件失败: %v", err)
	}
}
