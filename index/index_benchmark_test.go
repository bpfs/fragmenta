package index

import (
	"testing"
)

// BenchmarkIndexOperations 对比基本索引操作的性能
func BenchmarkIndexOperations(b *testing.B) {
	// 创建优化索引管理器
	config := &IndexConfig{
		AutoSave:       false,
		AutoRebuild:    false,
		AsyncUpdate:    false,
		MaxWorkers:     2,
		NumShards:      4,
		BatchThreshold: 500,
	}

	indexManager, err := NewOptimizedIndexManager(config)
	if err != nil {
		b.Fatalf("创建索引管理器失败: %v", err)
	}

	// 预先添加一些数据
	for i := 0; i < 1000; i++ {
		tag := uint32(i % 10)
		id := uint32(i)
		err := indexManager.AddIndex(tag, id)
		if err != nil {
			b.Fatalf("添加索引失败: %v", err)
		}
	}

	// 测试添加性能
	b.Run("AddIndex", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tag := uint32(i % 10)
			id := uint32(1000 + i)
			err := indexManager.AddIndex(tag, id)
			if err != nil {
				b.Fatalf("添加索引失败: %v", err)
			}
		}
	})

	// 测试查找性能
	b.Run("FindByTag", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tag := uint32(i % 10)
			_, err := indexManager.FindByTag(tag)
			if err != nil {
				b.Fatalf("查找索引失败: %v", err)
			}
		}
	})

	// 测试删除性能
	b.Run("RemoveIndex", func(b *testing.B) {
		// 先添加要删除的数据
		for i := 0; i < 1000; i++ {
			tag := uint32(20)
			id := uint32(2000 + i)
			err := indexManager.AddIndex(tag, id)
			if err != nil {
				b.Fatalf("添加索引失败: %v", err)
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i >= 1000 {
				break // 避免删除不存在的索引
			}
			err := indexManager.RemoveIndex(uint32(20), uint32(2000+i))
			if err != nil {
				b.Fatalf("删除索引失败: %v", err)
			}
		}
	})
}

// BenchmarkQueryOperations 测试不同查询操作的性能
func BenchmarkQueryOperations(b *testing.B) {
	// 使用模拟索引管理器
	mgr := createMockIndexManager()
	queryExecutor := NewQueryExecutor(mgr)

	// 基准测试简单查询
	b.Run("SimpleQuery", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			query, err := queryExecutor.ParseQueryString("tag:type==1")
			if err != nil {
				b.Fatalf("解析查询失败: %v", err)
			}

			result, err := queryExecutor.Execute(query)
			if err != nil {
				b.Fatalf("执行查询失败: %v", err)
			}
			if len(result.IDs) == 0 {
				b.Fatalf("查询结果为空")
			}
		}
	})

	// 基准测试复合查询
	b.Run("CompoundQuery", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			query, err := queryExecutor.ParseQueryString("tag:type==1 and tag:category==10")
			if err != nil {
				b.Fatalf("解析复合查询失败: %v", err)
			}

			_, err = queryExecutor.Execute(query)
			if err != nil {
				b.Fatalf("执行复合查询失败: %v", err)
			}
		}
	})

	// 基准测试排序查询
	b.Run("SortedQuery", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// 修复排序查询语法
			query, err := queryExecutor.ParseQueryString("tag:type==1; sort: -id; limit: 10")
			if err != nil {
				b.Fatalf("解析排序查询失败: %v", err)
			}

			result, err := queryExecutor.Execute(query)
			if err != nil {
				b.Fatalf("执行排序查询失败: %v", err)
			}
			if len(result.IDs) == 0 {
				b.Fatalf("排序查询结果为空")
			}
		}
	})
}

// BenchmarkOptimizer 测试索引优化器性能
func BenchmarkOptimizer(b *testing.B) {
	// 创建要优化的索引管理器
	config := &IndexConfig{
		AutoSave:       false,
		AutoRebuild:    false,
		AsyncUpdate:    false,
		MaxWorkers:     2,
		NumShards:      4,
		BatchThreshold: 500,
	}

	indexManager, err := NewOptimizedIndexManager(config)
	if err != nil {
		b.Fatalf("创建索引管理器失败: %v", err)
	}

	// 添加测试数据
	for i := 0; i < 2000; i++ {
		tag := uint32(i % 20)
		id := uint32(i)
		err := indexManager.AddIndex(tag, id)
		if err != nil {
			b.Fatalf("添加索引失败: %v", err)
		}

		// 添加重复数据
		if i%4 == 0 {
			err := indexManager.AddIndex(tag, id)
			if err != nil {
				b.Fatalf("添加重复索引失败: %v", err)
			}
		}
	}

	// 测试索引压缩性能
	b.Run("CompressIndex", func(b *testing.B) {
		optimizer := NewDefaultIndexOptimizer(&OptimizationConfig{
			CompressionLevel: 1,
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := optimizer.CompressIndex(indexManager, 1)
			if err != nil {
				b.Fatalf("压缩索引失败: %v", err)
			}
		}
	})

	// 测试前缀树构建性能
	b.Run("BuildPrefixIndex", func(b *testing.B) {
		optimizer := NewDefaultIndexOptimizer(&OptimizationConfig{
			EnablePrefixCompression: true,
			MaxPrefixTreeDepth:      5,
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := optimizer.BuildPrefixIndex(indexManager)
			if err != nil {
				b.Fatalf("构建前缀索引失败: %v", err)
			}
		}
	})

	// 测试完整优化过程性能
	b.Run("OptimizeIndex", func(b *testing.B) {
		optimizer := NewDefaultIndexOptimizer(&OptimizationConfig{
			WorkerCount:             2,
			BatchSize:               200,
			EnablePrefixCompression: true,
			CompressionLevel:        1,
			MaxPrefixTreeDepth:      4,
			ShardBalanceThreshold:   0.2,
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := optimizer.OptimizeIndex(indexManager)
			if err != nil {
				b.Fatalf("优化索引失败: %v", err)
			}
		}
	})
}
