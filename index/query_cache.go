package index

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// 定义缓存键结构
type queryCacheKey struct {
	// 查询条件序列化后的哈希值
	queryHash string

	// 其他唯一标识信息
	indexManagerID string
}

// 定义缓存项结构
type queryCacheItem struct {
	// 缓存键
	key queryCacheKey

	// 缓存的查询计划
	plan QueryPlan

	// 缓存的查询结果
	result *QueryResult

	// 访问时间
	accessTime time.Time

	// 创建时间
	createTime time.Time

	// 访问次数
	accessCount int
}

// ExtendedLRUQueryCache 实现基于LRU（最近最少使用）策略的查询缓存
type ExtendedLRUQueryCache struct {
	// 缓存容量
	capacity int

	// 使用双向链表存储缓存项，支持O(1)头尾操作
	list *list.List

	// 使用map存储键到链表节点的映射，支持O(1)查找
	items map[queryCacheKey]*list.Element

	// 用于计划缓存的哈希计算函数
	planHashFunc func(*Query) string

	// 用于结果缓存的哈希计算函数
	resultHashFunc func(QueryPlan) string

	// 缓存命中统计
	hits int

	// 缓存未命中统计
	misses int

	// 互斥锁，保证线程安全
	mutex sync.RWMutex

	// 缓存过期时间（秒）
	expiry int
}

// NewExtendedLRUQueryCache 创建新的LRU查询缓存
func NewExtendedLRUQueryCache(capacity int) *ExtendedLRUQueryCache {
	return &ExtendedLRUQueryCache{
		capacity:       capacity,
		list:           list.New(),
		items:          make(map[queryCacheKey]*list.Element),
		planHashFunc:   defaultQueryHash,
		resultHashFunc: defaultPlanHash,
		expiry:         3600, // 默认1小时过期
	}
}

// defaultQueryHash 默认查询哈希函数
func defaultQueryHash(query *Query) string {
	// 简单实现，实际应该对查询结构进行规范化处理后再哈希
	if query == nil || query.RootCondition == nil {
		return "empty_query"
	}

	// 处理值的字符串表示
	var valueStr string
	switch v := query.RootCondition.Value.(type) {
	case int:
		valueStr = fmt.Sprintf("%d", v)
	case int32:
		valueStr = fmt.Sprintf("%d", v)
	case int64:
		valueStr = fmt.Sprintf("%d", v)
	case uint:
		valueStr = fmt.Sprintf("%d", v)
	case uint32:
		valueStr = fmt.Sprintf("%d", v)
	case uint64:
		valueStr = fmt.Sprintf("%d", v)
	case float32:
		valueStr = fmt.Sprintf("%.6f", v)
	case float64:
		valueStr = fmt.Sprintf("%.6f", v)
	case string:
		valueStr = v
	case bool:
		valueStr = fmt.Sprintf("%t", v)
	default:
		// 对于其他类型，使用反射
		valueStr = fmt.Sprintf("%v", v)
	}

	// 使用条件的字符串表示作为哈希
	return query.RootCondition.Field +
		string(query.RootCondition.FieldType) +
		string(query.RootCondition.Operator) +
		valueStr +
		"limit:" + fmt.Sprintf("%d", query.Limit) +
		"offset:" + fmt.Sprintf("%d", query.Offset)
}

// defaultPlanHash 默认计划哈希函数
func defaultPlanHash(plan QueryPlan) string {
	// 简单实现，实际应该基于计划特性生成唯一标识
	if plan == nil {
		return "empty_plan"
	}

	return string(plan.GetType()) + "_" + plan.GetDescription()
}

// Get 根据查询获取缓存的查询计划
func (c *ExtendedLRUQueryCache) Get(query *Query) QueryPlan {
	if query == nil {
		return nil
	}

	key := queryCacheKey{
		queryHash: c.planHashFunc(query),
		// indexManagerID应从上下文获取，这里简化处理
		indexManagerID: "default",
	}

	// 读锁保护
	c.mutex.RLock()

	// 检查缓存
	if element, found := c.items[key]; found {
		item := element.Value.(*queryCacheItem)

		// 检查是否过期
		if c.isExpired(item) {
			c.mutex.RUnlock()
			// 需要写锁保护的删除操作
			c.mutex.Lock()
			c.removeElement(element)
			c.mutex.Unlock()
			c.misses++
			return nil
		}

		// 更新统计
		c.hits++

		// 读锁降级为写锁，更新访问信息
		c.mutex.RUnlock()
		c.mutex.Lock()
		defer c.mutex.Unlock()

		// 更新访问信息
		item.accessTime = time.Now()
		item.accessCount++

		// 将节点移动到链表头部（最近使用）
		c.list.MoveToFront(element)

		return item.plan
	}

	// 未命中
	c.mutex.RUnlock()
	c.misses++
	return nil
}

// Put 存储查询计划到缓存
func (c *ExtendedLRUQueryCache) Put(query *Query, plan QueryPlan) {
	if query == nil || plan == nil {
		return
	}

	key := queryCacheKey{
		queryHash: c.planHashFunc(query),
		// indexManagerID应从上下文获取，这里简化处理
		indexManagerID: "default",
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查是否已存在
	if element, found := c.items[key]; found {
		// 更新缓存项
		item := element.Value.(*queryCacheItem)
		item.plan = plan
		item.accessTime = time.Now()
		item.accessCount++

		// 移动到链表头部
		c.list.MoveToFront(element)
		return
	}

	// 创建新的缓存项
	item := &queryCacheItem{
		key:         key,
		plan:        plan,
		accessTime:  time.Now(),
		createTime:  time.Now(),
		accessCount: 1,
	}

	// 添加到链表头部
	element := c.list.PushFront(item)
	c.items[key] = element

	// 检查容量，如果超出则移除最久未使用的项
	if c.list.Len() > c.capacity {
		c.removeOldest()
	}
}

// GetResult 获取查询结果
func (c *ExtendedLRUQueryCache) GetResult(plan QueryPlan) (*QueryResult, bool) {
	if plan == nil {
		return nil, false
	}

	key := queryCacheKey{
		queryHash: c.resultHashFunc(plan),
		// indexManagerID应从上下文获取，这里简化处理
		indexManagerID: "default",
	}

	c.mutex.RLock()

	// 检查缓存
	if element, found := c.items[key]; found {
		item := element.Value.(*queryCacheItem)

		// 检查结果是否存在
		if item.result == nil {
			c.mutex.RUnlock()
			c.misses++ // 增加未命中计数
			return nil, false
		}

		// 检查是否过期
		if c.isExpired(item) {
			c.mutex.RUnlock()
			// 需要写锁保护的删除操作
			c.mutex.Lock()
			c.removeElement(element)
			c.misses++ // 增加未命中计数
			c.mutex.Unlock()
			return nil, false
		}

		result := item.result

		// 增加命中计数
		c.hits++

		// 更新访问信息
		c.mutex.RUnlock()
		c.mutex.Lock()
		defer c.mutex.Unlock()

		item.accessTime = time.Now()
		item.accessCount++

		// 将节点移动到链表头部
		c.list.MoveToFront(element)

		return result, true
	}

	c.mutex.RUnlock()
	c.misses++ // 增加未命中计数
	return nil, false
}

// PutResult 存储查询结果
func (c *ExtendedLRUQueryCache) PutResult(plan QueryPlan, result *QueryResult) {
	if plan == nil || result == nil {
		return
	}

	key := queryCacheKey{
		queryHash: c.resultHashFunc(plan),
		// indexManagerID应从上下文获取，这里简化处理
		indexManagerID: "default",
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查是否已存在
	if element, found := c.items[key]; found {
		// 更新缓存项
		item := element.Value.(*queryCacheItem)
		item.result = result
		item.accessTime = time.Now()
		item.accessCount++

		// 移动到链表头部
		c.list.MoveToFront(element)
		return
	}

	// 创建新的缓存项
	item := &queryCacheItem{
		key:         key,
		plan:        nil, // 这里只缓存结果，不缓存计划
		result:      result,
		accessTime:  time.Now(),
		createTime:  time.Now(),
		accessCount: 1,
	}

	// 添加到链表头部
	element := c.list.PushFront(item)
	c.items[key] = element

	// 检查容量，如果超出则移除最久未使用的项
	if c.list.Len() > c.capacity {
		c.removeOldest()
	}
}

// Clear 清除缓存
func (c *ExtendedLRUQueryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 清空链表和映射
	c.list.Init()
	c.items = make(map[queryCacheKey]*list.Element)
	c.hits = 0
	c.misses = 0
}

// removeOldest 移除最久未使用的缓存项
func (c *ExtendedLRUQueryCache) removeOldest() {
	// 获取链表尾部元素（最久未使用）
	element := c.list.Back()
	if element != nil {
		// 从映射中移除
		item := element.Value.(*queryCacheItem)
		delete(c.items, item.key)

		// 从链表中移除
		c.list.Remove(element)
	}
}

// removeElement 从缓存中移除指定元素
func (c *ExtendedLRUQueryCache) removeElement(element *list.Element) {
	if element == nil {
		return
	}

	// 从映射中移除
	item := element.Value.(*queryCacheItem)
	delete(c.items, item.key)

	// 从链表中移除
	c.list.Remove(element)
}

// isExpired 检查缓存项是否过期
func (c *ExtendedLRUQueryCache) isExpired(item *queryCacheItem) bool {
	if c.expiry <= 0 {
		return false
	}

	expireTime := item.createTime.Add(time.Duration(c.expiry) * time.Second)
	return time.Now().After(expireTime)
}

// GetStats 获取缓存统计信息
func (c *ExtendedLRUQueryCache) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	hitRate := 0.0
	totalAccess := c.hits + c.misses
	if totalAccess > 0 {
		hitRate = float64(c.hits) / float64(totalAccess)
	}

	return map[string]interface{}{
		"capacity": c.capacity,
		"size":     c.list.Len(),
		"hits":     c.hits,
		"misses":   c.misses,
		"hit_rate": hitRate,
		"expiry":   c.expiry,
	}
}

// SetExpiry 设置缓存过期时间（秒）
func (c *ExtendedLRUQueryCache) SetExpiry(seconds int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.expiry = seconds
}
