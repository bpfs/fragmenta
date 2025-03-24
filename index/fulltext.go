// package index 提供全文搜索功能
package index

import (
	"errors"
	"time"
)

// 错误定义
var (
	ErrTokenizerNotFound       = errors.New("tokenizer not found")
	ErrIndexNotBuilt           = errors.New("full-text index not built")
	ErrUnsupportedTokenizer    = errors.New("unsupported tokenizer")
	ErrInvalidSearchExpression = errors.New("invalid search expression")
)

// TokenType 表示标记类型
type TokenType int

const (
	// 普通词语
	TokenWord TokenType = iota
	// 数字
	TokenNumber
	// 特殊符号
	TokenSymbol
	// 中文词语
	TokenChinese
	// 停用词
	TokenStopWord
)

// Token 表示文本分词后的标记
type Token struct {
	// 原始文本
	Text string
	// 规范化文本（小写、词干提取等处理后）
	Normalized string
	// 标记类型
	Type TokenType
	// 在原文中的位置
	Position int
	// 权重
	Weight float64
}

// Tokenizer 分词器接口
type Tokenizer interface {
	// 对文本进行分词
	Tokenize(text string) ([]*Token, error)
	// 添加停用词
	AddStopWords(words []string)
	// 获取分词器名称
	Name() string
}

// FullTextIndex 全文索引接口
type FullTextIndex interface {
	// 为文档构建索引
	IndexDocument(id uint32, content string, metadata map[string]interface{}) error
	// 批量索引文档
	BatchIndexDocuments(docs map[uint32]string, metadatas map[uint32]map[string]interface{}) error
	// 搜索文档
	Search(query string, options *SearchOptions) (*SearchResult, error)
	// 获取索引统计信息
	GetStatistics() *FullTextIndexStatistics
	// 删除文档
	RemoveDocument(id uint32) error
	// 清空索引
	ClearIndex() error
	// 保存索引到磁盘
	SaveIndex(path string) error
	// 从磁盘加载索引
	LoadIndex(path string) error
}

// SearchOptions 搜索选项
type SearchOptions struct {
	// 限制结果数量
	Limit int
	// 结果偏移
	Offset int
	// 相关性阈值（0-1）
	RelevanceThreshold float64
	// 是否高亮匹配内容
	Highlight bool
	// 是否启用模糊匹配
	FuzzyMatch bool
	// 模糊匹配阈值（0-1）
	FuzzyThreshold float64
	// 排序方式（相关性、时间等）
	SortBy string
	// 是否升序排序
	Ascending bool
	// 字段过滤
	FieldFilters map[string]interface{}
}

// SearchResult 搜索结果
type SearchResult struct {
	// 匹配的文档ID及相关性分数
	Matches map[uint32]float64
	// 匹配文档总数（不考虑分页）
	TotalMatches int
	// 执行时间
	ExecutionTime time.Duration
	// 高亮内容片段（如启用）
	Highlights map[uint32][]string
	// 按相关性排序的ID列表
	SortedIDs []uint32
	// 查询解析信息
	QueryInfo *QueryInfo
}

// QueryInfo 查询解析信息
type QueryInfo struct {
	// 查询中的词条
	Terms []string
	// 查询操作符
	Operators []string
	// 原始查询
	OriginalQuery string
	// 扩展查询（如有）
	ExpandedQuery string
}

// DocumentIndex 文档索引信息
type DocumentIndex struct {
	// 文档ID
	ID uint32
	// 分词结果
	Tokens []*Token
	// 词条频率映射
	TermFrequency map[string]int
	// 文档长度（标记数）
	Length int
	// 最后更新时间
	LastUpdated time.Time
	// 文档元数据
	Metadata map[string]interface{}
}

// InvertedIndex 倒排索引条目
type InvertedIndex struct {
	// 词条
	Term string
	// 文档频率（包含该词的文档数）
	DocumentFrequency int
	// 倒排列表
	Postings map[uint32]*PostingInfo
}

// PostingInfo 倒排列表项
type PostingInfo struct {
	// 词条频率（在文档中出现的次数）
	TermFrequency int
	// 位置列表（词条在文档中的位置）
	Positions []int
	// 权重
	Weight float64
}

// FullTextIndexStatistics 全文索引统计信息
type FullTextIndexStatistics struct {
	// 文档总数
	DocumentCount int
	// 唯一词条数
	TermCount int
	// 索引大小（字节）
	IndexSize int64
	// 最后更新时间
	LastUpdated time.Time
	// 平均文档长度
	AverageDocumentLength float64
	// 标记总数
	TotalTokens int
}
