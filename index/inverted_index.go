// package index 提供倒排索引实现
package index

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// DefaultFullTextIndex 是倒排索引的默认实现
type DefaultFullTextIndex struct {
	// 索引映射，词项 -> 倒排索引条目
	indexMap map[string]*InvertedIndex

	// 文档索引，文档ID -> 文档索引信息
	documentMap map[uint32]*DocumentIndex

	// 分词器
	tokenizer Tokenizer

	// 互斥锁，用于并发控制
	mu sync.RWMutex

	// 统计信息
	stats FullTextIndexStatistics

	// 上次保存时间
	lastSaveTime time.Time
}

// NewFullTextIndex 创建新的全文索引
func NewFullTextIndex(tokenizer Tokenizer) *DefaultFullTextIndex {
	if tokenizer == nil {
		tokenizer = NewSimpleTokenizer()
	}

	return &DefaultFullTextIndex{
		indexMap:    make(map[string]*InvertedIndex),
		documentMap: make(map[uint32]*DocumentIndex),
		tokenizer:   tokenizer,
		stats: FullTextIndexStatistics{
			LastUpdated: time.Now(),
		},
		lastSaveTime: time.Now(),
	}
}

// IndexDocument 为文档构建索引
func (idx *DefaultFullTextIndex) IndexDocument(id uint32, content string, metadata map[string]interface{}) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 检查文档ID是否已存在，如果存在则先删除旧索引
	if _, exists := idx.documentMap[id]; exists {
		idx.removeDocumentUnsafe(id)
	}

	// 对文档内容进行分词
	tokens, err := idx.tokenizer.Tokenize(content)
	if err != nil {
		return err
	}

	// 创建文档索引信息
	docIndex := &DocumentIndex{
		ID:            id,
		Tokens:        tokens,
		TermFrequency: make(map[string]int),
		Length:        len(tokens),
		LastUpdated:   time.Now(),
		Metadata:      metadata,
	}

	// 更新词项频率
	for _, token := range tokens {
		// 忽略停用词
		if token.Type == TokenStopWord {
			continue
		}

		// 更新文档的词项频率
		docIndex.TermFrequency[token.Normalized]++

		// 获取或创建倒排索引条目
		invIndex, exists := idx.indexMap[token.Normalized]
		if !exists {
			invIndex = &InvertedIndex{
				Term:              token.Normalized,
				DocumentFrequency: 0,
				Postings:          make(map[uint32]*PostingInfo),
			}
			idx.indexMap[token.Normalized] = invIndex
		}

		// 获取或创建倒排列表项
		posting, exists := invIndex.Postings[id]
		if !exists {
			posting = &PostingInfo{
				TermFrequency: 0,
				Positions:     make([]int, 0),
				Weight:        1.0,
			}
			invIndex.Postings[id] = posting
			invIndex.DocumentFrequency++
		}

		// 更新倒排列表项
		posting.TermFrequency++
		posting.Positions = append(posting.Positions, token.Position)
	}

	// 保存文档索引信息
	idx.documentMap[id] = docIndex

	// 更新统计信息
	idx.updateStats()

	return nil
}

// BatchIndexDocuments 批量索引文档
func (idx *DefaultFullTextIndex) BatchIndexDocuments(docs map[uint32]string, metadatas map[uint32]map[string]interface{}) error {
	// 批量索引的简单实现：依次索引每个文档
	// 实际实现中应该采用并行处理和批量更新优化性能
	for id, content := range docs {
		metadata := make(map[string]interface{})
		if meta, exists := metadatas[id]; exists {
			metadata = meta
		}

		err := idx.IndexDocument(id, content, metadata)
		if err != nil {
			return err
		}
	}

	return nil
}

// Search 搜索文档
func (idx *DefaultFullTextIndex) Search(query string, options *SearchOptions) (*SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// 使用默认选项
	if options == nil {
		options = &SearchOptions{
			Limit:              10,
			Offset:             0,
			RelevanceThreshold: 0.1,
			Highlight:          false,
			FuzzyMatch:         false,
			FuzzyThreshold:     0.8,
			SortBy:             "relevance",
			Ascending:          false,
		}
	}

	// 开始计时
	startTime := time.Now()

	// 对查询进行分词
	tokens, err := idx.tokenizer.Tokenize(query)
	if err != nil {
		return nil, err
	}

	// 准备查询信息
	queryInfo := &QueryInfo{
		Terms:         make([]string, 0),
		Operators:     make([]string, 0),
		OriginalQuery: query,
	}

	// 提取查询词项
	queryTerms := make([]string, 0)
	for _, token := range tokens {
		if token.Type != TokenStopWord && token.Type != TokenSymbol {
			queryTerms = append(queryTerms, token.Normalized)
			queryInfo.Terms = append(queryInfo.Terms, token.Normalized)
		}
	}

	// 如果没有有效查询词项，返回空结果
	if len(queryTerms) == 0 {
		return &SearchResult{
			Matches:       make(map[uint32]float64),
			TotalMatches:  0,
			ExecutionTime: time.Since(startTime),
			Highlights:    make(map[uint32][]string),
			SortedIDs:     make([]uint32, 0),
			QueryInfo:     queryInfo,
		}, nil
	}

	// 计算相关性分数
	scores := idx.calculateScores(queryTerms)

	// 应用相关性阈值过滤
	filteredScores := make(map[uint32]float64)
	for id, score := range scores {
		if score >= options.RelevanceThreshold {
			filteredScores[id] = score
		}
	}

	// 排序结果
	sortedIDs := idx.sortResults(filteredScores, options)

	// 应用分页
	var pagedIDs []uint32
	if options.Offset < len(sortedIDs) {
		end := options.Offset + options.Limit
		if end > len(sortedIDs) {
			end = len(sortedIDs)
		}
		pagedIDs = sortedIDs[options.Offset:end]
	} else {
		pagedIDs = make([]uint32, 0)
	}

	// 生成高亮内容（如果需要）
	highlights := make(map[uint32][]string)
	if options.Highlight {
		// 简化实现：暂不实现高亮功能
	}

	// 构建结果
	result := &SearchResult{
		Matches:       filteredScores,
		TotalMatches:  len(filteredScores),
		ExecutionTime: time.Since(startTime),
		Highlights:    highlights,
		SortedIDs:     pagedIDs,
		QueryInfo:     queryInfo,
	}

	return result, nil
}

// GetStatistics 获取索引统计信息
func (idx *DefaultFullTextIndex) GetStatistics() *FullTextIndexStatistics {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// 返回统计信息的副本
	stats := idx.stats
	return &stats
}

// RemoveDocument 删除文档
func (idx *DefaultFullTextIndex) RemoveDocument(id uint32) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.removeDocumentUnsafe(id)
}

// removeDocumentUnsafe 删除文档的内部实现（不加锁）
func (idx *DefaultFullTextIndex) removeDocumentUnsafe(id uint32) error {
	// 检查文档是否存在
	docIndex, exists := idx.documentMap[id]
	if !exists {
		return ErrIndexNotFound
	}

	// 更新倒排索引
	for term := range docIndex.TermFrequency {
		if invIndex, exists := idx.indexMap[term]; exists {
			// 从倒排列表中删除文档
			delete(invIndex.Postings, id)

			// 更新文档频率
			invIndex.DocumentFrequency--

			// 如果没有文档包含该词项，则删除该词项的索引
			if invIndex.DocumentFrequency <= 0 {
				delete(idx.indexMap, term)
			}
		}
	}

	// 删除文档索引
	delete(idx.documentMap, id)

	// 更新统计信息
	idx.updateStats()

	return nil
}

// ClearIndex 清空索引
func (idx *DefaultFullTextIndex) ClearIndex() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 清空索引和文档映射
	idx.indexMap = make(map[string]*InvertedIndex)
	idx.documentMap = make(map[uint32]*DocumentIndex)

	// 重置统计信息
	idx.stats = FullTextIndexStatistics{
		LastUpdated: time.Now(),
	}

	return nil
}

// SaveIndex 保存索引到磁盘
func (idx *DefaultFullTextIndex) SaveIndex(path string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// 构建要保存的数据结构
	data := struct {
		IndexMap      map[string]*InvertedIndex `json:"index_map"`
		DocumentMap   map[uint32]*DocumentIndex `json:"document_map"`
		Stats         FullTextIndexStatistics   `json:"stats"`
		TokenizerName string                    `json:"tokenizer_name"`
	}{
		IndexMap:      idx.indexMap,
		DocumentMap:   idx.documentMap,
		Stats:         idx.stats,
		TokenizerName: idx.tokenizer.Name(),
	}

	// 将数据序列化为JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	err = os.WriteFile(path, jsonData, 0644)
	if err != nil {
		return err
	}

	// 更新保存时间
	idx.lastSaveTime = time.Now()

	return nil
}

// LoadIndex 从磁盘加载索引
func (idx *DefaultFullTextIndex) LoadIndex(path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 读取文件
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// 定义用于加载的数据结构
	var data struct {
		IndexMap      map[string]*InvertedIndex `json:"index_map"`
		DocumentMap   map[uint32]*DocumentIndex `json:"document_map"`
		Stats         FullTextIndexStatistics   `json:"stats"`
		TokenizerName string                    `json:"tokenizer_name"`
	}

	// 解析JSON
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}

	// 检查分词器类型兼容性
	if data.TokenizerName != idx.tokenizer.Name() {
		// 可以选择返回错误或仅记录警告
		// 此处简化处理，仅在分词器不匹配时继续使用当前分词器
	}

	// 更新索引数据
	idx.indexMap = data.IndexMap
	idx.documentMap = data.DocumentMap
	idx.stats = data.Stats

	// 更新最后保存时间
	idx.lastSaveTime = time.Now()

	return nil
}

// updateStats 更新索引统计信息
func (idx *DefaultFullTextIndex) updateStats() {
	// 重置统计信息
	stats := FullTextIndexStatistics{
		DocumentCount: len(idx.documentMap),
		TermCount:     len(idx.indexMap),
		LastUpdated:   time.Now(),
		TotalTokens:   0,
	}

	// 计算总标记数和平均文档长度
	var totalLength int
	for _, doc := range idx.documentMap {
		totalLength += doc.Length
		stats.TotalTokens += doc.Length
	}

	if stats.DocumentCount > 0 {
		stats.AverageDocumentLength = float64(totalLength) / float64(stats.DocumentCount)
	}

	// 更新统计信息
	idx.stats = stats
}

// calculateScores 计算查询与文档的相关性分数
func (idx *DefaultFullTextIndex) calculateScores(queryTerms []string) map[uint32]float64 {
	// 使用TF-IDF算法计算相关性分数
	scores := make(map[uint32]float64)

	// 文档总数
	N := float64(len(idx.documentMap))

	// 对每个查询词项
	for _, term := range queryTerms {
		// 获取倒排索引
		invIndex, exists := idx.indexMap[term]
		if !exists {
			continue
		}

		// 计算IDF (Inverse Document Frequency)
		// IDF = log(N / df)，其中df是包含该词项的文档数
		idf := 1.0
		if invIndex.DocumentFrequency > 0 {
			idf = 1.0 + log2(N/float64(invIndex.DocumentFrequency))
		}

		// 对包含该词项的每个文档
		for docID, posting := range invIndex.Postings {
			// 获取文档信息
			docIndex, exists := idx.documentMap[docID]
			if !exists {
				continue
			}

			// 计算TF (Term Frequency)
			// TF = 词项在文档中的出现次数 / 文档长度
			tf := float64(posting.TermFrequency) / float64(docIndex.Length)

			// 计算TF-IDF分数
			score := tf * idf * posting.Weight

			// 累加到文档总分
			scores[docID] += score
		}
	}

	return scores
}

// sortResults 根据选项对结果排序
func (idx *DefaultFullTextIndex) sortResults(scores map[uint32]float64, options *SearchOptions) []uint32 {
	// 提取ID列表
	ids := make([]uint32, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}

	// 根据排序选项进行排序
	switch options.SortBy {
	case "relevance":
		// 按相关性分数排序
		if options.Ascending {
			// 升序
			sortDocIDsByScoreAsc(ids, scores)
		} else {
			// 降序
			sortDocIDsByScoreDesc(ids, scores)
		}
	// 可以添加其他排序选项，如按时间、按名称等
	default:
		// 默认按相关性分数降序排序
		sortDocIDsByScoreDesc(ids, scores)
	}

	return ids
}

// sortDocIDsByScoreDesc 按分数降序排序文档ID
func sortDocIDsByScoreDesc(ids []uint32, scores map[uint32]float64) {
	sortDocIDs(ids, func(i, j int) bool {
		return scores[ids[i]] > scores[ids[j]]
	})
}

// sortDocIDsByScoreAsc 按分数升序排序文档ID
func sortDocIDsByScoreAsc(ids []uint32, scores map[uint32]float64) {
	sortDocIDs(ids, func(i, j int) bool {
		return scores[ids[i]] < scores[ids[j]]
	})
}

// sortDocIDs 对文档ID进行排序
func sortDocIDs(ids []uint32, less func(i, j int) bool) {
	// 简单的冒泡排序实现
	// 实际应使用更高效的排序算法
	n := len(ids)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if less(j+1, j) {
				ids[j], ids[j+1] = ids[j+1], ids[j]
			}
		}
	}
}

// log2 计算以2为底的对数
func log2(x float64) float64 {
	// 简化实现
	// 在实际代码中，应使用math包中的函数
	// 此处简化示意
	return 1.0
}
