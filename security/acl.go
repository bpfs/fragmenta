package security

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// 常见错误定义
var (
	// ErrPermissionDenied 表示权限被拒绝
	ErrPermissionDenied = errors.New("permission denied")

	// ErrInvalidSubject 表示无效的主体
	ErrInvalidSubject = errors.New("invalid subject")

	// ErrInvalidResource 表示无效的资源
	ErrInvalidResource = errors.New("invalid resource")

	// ErrInvalidOperation 表示无效的操作
	ErrInvalidOperation = errors.New("invalid operation")

	// ErrInvalidPolicy 表示无效的策略
	ErrInvalidPolicy = errors.New("invalid policy")
)

// ResourceType 资源类型
type ResourceType string

const (
	// FileResource 文件资源
	FileResource ResourceType = "file"

	// DirectoryResource 目录资源
	DirectoryResource ResourceType = "directory"

	// BlockResource 块资源
	BlockResource ResourceType = "block"

	// MetadataResource 元数据资源
	MetadataResource ResourceType = "metadata"

	// SystemResource 系统资源
	SystemResource ResourceType = "system"
)

// Operation 操作类型
type Operation string

const (
	// ReadOperation 读操作
	ReadOperation Operation = "read"

	// WriteOperation 写操作
	WriteOperation Operation = "write"

	// DeleteOperation 删除操作
	DeleteOperation Operation = "delete"

	// CreateOperation 创建操作
	CreateOperation Operation = "create"

	// ListOperation 列出操作
	ListOperation Operation = "list"

	// ExecuteOperation 执行操作
	ExecuteOperation Operation = "execute"

	// AdminOperation 管理操作
	AdminOperation Operation = "admin"
)

// Policy 权限策略
type Policy string

const (
	// AllowPolicy 允许策略
	AllowPolicy Policy = "allow"

	// DenyPolicy 拒绝策略
	DenyPolicy Policy = "deny"
)

// Subject 表示访问资源的主体
type Subject struct {
	// ID 主体标识符
	ID string

	// Type 主体类型（用户、组、角色等）
	Type string

	// Attributes 附加属性
	Attributes map[string]string
}

// SubjectType 主体类型
type SubjectType string

const (
	// UserSubject 用户主体
	UserSubject SubjectType = "user"

	// GroupSubject 组主体
	GroupSubject SubjectType = "group"

	// RoleSubject 角色主体
	RoleSubject SubjectType = "role"

	// SystemSubject 系统主体
	SystemSubject SubjectType = "system"
)

// Resource 表示被访问的资源
type Resource struct {
	// ID 资源标识符
	ID string

	// Type 资源类型
	Type ResourceType

	// Attributes 附加属性
	Attributes map[string]string
}

// ACLEntry 访问控制条目
type ACLEntry struct {
	// Subject 主体
	Subject Subject

	// Resource 资源
	Resource Resource

	// Operation 操作
	Operation Operation

	// Policy 策略（允许/拒绝）
	Policy Policy

	// Conditions 附加条件
	Conditions map[string]interface{}

	// CreatedAt 创建时间
	CreatedAt time.Time

	// ExpiresAt 过期时间（可选）
	ExpiresAt *time.Time
}

// ACLManager 访问控制列表管理接口
type ACLManager interface {
	// AddEntry 添加访问控制条目
	AddEntry(ctx context.Context, entry *ACLEntry) error

	// RemoveEntry 移除访问控制条目
	RemoveEntry(ctx context.Context, entry *ACLEntry) error

	// UpdateEntry 更新访问控制条目
	UpdateEntry(ctx context.Context, entry *ACLEntry) error

	// CheckAccess 检查是否允许访问
	CheckAccess(ctx context.Context, subject Subject, resource Resource, operation Operation) (bool, error)

	// ListEntries 列出符合条件的访问控制条目
	ListEntries(ctx context.Context, subject *Subject, resource *Resource, operation *Operation) ([]*ACLEntry, error)
}

// DefaultACLManager 默认的访问控制列表管理实现
type DefaultACLManager struct {
	// 存储ACL条目
	entries []*ACLEntry

	// 互斥锁保护并发访问
	mutex sync.RWMutex
}

// NewDefaultACLManager 创建默认的访问控制列表管理器
func NewDefaultACLManager() *DefaultACLManager {
	return &DefaultACLManager{
		entries: make([]*ACLEntry, 0),
	}
}

// AddEntry 添加访问控制条目
func (m *DefaultACLManager) AddEntry(ctx context.Context, entry *ACLEntry) error {
	if entry == nil {
		return errors.New("entry cannot be nil")
	}

	// 验证条目有效性
	if err := validateEntry(entry); err != nil {
		return err
	}

	// 设置创建时间（如果未设置）
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否已存在相同条目
	for _, e := range m.entries {
		if entriesEqual(e, entry) {
			return errors.New("duplicate entry")
		}
	}

	// 添加条目
	m.entries = append(m.entries, entry)
	return nil
}

// RemoveEntry 移除访问控制条目
func (m *DefaultACLManager) RemoveEntry(ctx context.Context, entry *ACLEntry) error {
	if entry == nil {
		return errors.New("entry cannot be nil")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 查找并移除匹配的条目
	for i, e := range m.entries {
		if entriesEqual(e, entry) {
			// 移除条目（保持顺序不变）
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			return nil
		}
	}

	return errors.New("entry not found")
}

// UpdateEntry 更新访问控制条目
func (m *DefaultACLManager) UpdateEntry(ctx context.Context, entry *ACLEntry) error {
	if entry == nil {
		return errors.New("entry cannot be nil")
	}

	// 验证条目有效性
	if err := validateEntry(entry); err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 查找并更新匹配的条目
	for i, e := range m.entries {
		if subjectEqual(e.Subject, entry.Subject) &&
			resourceEqual(e.Resource, entry.Resource) &&
			e.Operation == entry.Operation {
			// 更新条目
			m.entries[i] = entry
			return nil
		}
	}

	return errors.New("entry not found")
}

// CheckAccess 检查是否允许访问
func (m *DefaultACLManager) CheckAccess(ctx context.Context, subject Subject, resource Resource, operation Operation) (bool, error) {
	// 验证参数
	if subject.ID == "" {
		return false, ErrInvalidSubject
	}
	if resource.ID == "" {
		return false, ErrInvalidResource
	}
	if operation == "" {
		return false, ErrInvalidOperation
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 默认情况下，拒绝访问
	isAllowed := false

	// 先检查明确的拒绝规则（拒绝规则优先级高于允许规则）
	for _, entry := range m.entries {
		// 跳过过期的条目
		if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
			continue
		}

		// 检查是否匹配当前的主体、资源和操作
		if subjectMatch(entry.Subject, subject) &&
			resourceMatch(entry.Resource, resource) &&
			operationMatch(entry.Operation, operation) {

			// 检查附加条件
			if !evaluateConditions(entry.Conditions, subject, resource, operation) {
				continue
			}

			// 如果是拒绝策略，直接拒绝访问
			if entry.Policy == DenyPolicy {
				return false, nil
			}

			// 如果是允许策略，标记为允许（但继续检查是否有更高优先级的拒绝规则）
			if entry.Policy == AllowPolicy {
				isAllowed = true
			}
		}
	}

	return isAllowed, nil
}

// ListEntries 列出符合条件的访问控制条目
func (m *DefaultACLManager) ListEntries(ctx context.Context, subject *Subject, resource *Resource, operation *Operation) ([]*ACLEntry, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]*ACLEntry, 0)

	for _, entry := range m.entries {
		// 跳过过期的条目
		if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
			continue
		}

		// 根据过滤条件匹配
		if (subject == nil || subjectMatch(entry.Subject, *subject)) &&
			(resource == nil || resourceMatch(entry.Resource, *resource)) &&
			(operation == nil || entry.Operation == *operation) {
			result = append(result, entry)
		}
	}

	return result, nil
}

// 辅助函数: 验证条目有效性
func validateEntry(entry *ACLEntry) error {
	if entry.Subject.ID == "" {
		return ErrInvalidSubject
	}
	if entry.Resource.ID == "" {
		return ErrInvalidResource
	}
	if entry.Operation == "" {
		return ErrInvalidOperation
	}
	if entry.Policy != AllowPolicy && entry.Policy != DenyPolicy {
		return ErrInvalidPolicy
	}
	return nil
}

// 辅助函数: 检查两个条目是否相同
func entriesEqual(a, b *ACLEntry) bool {
	return subjectEqual(a.Subject, b.Subject) &&
		resourceEqual(a.Resource, b.Resource) &&
		a.Operation == b.Operation &&
		a.Policy == b.Policy
}

// 辅助函数: 检查两个主体是否相同
func subjectEqual(a, b Subject) bool {
	if a.ID != b.ID || a.Type != b.Type {
		return false
	}

	// 检查属性是否相同
	if len(a.Attributes) != len(b.Attributes) {
		return false
	}
	for k, v := range a.Attributes {
		if b.Attributes[k] != v {
			return false
		}
	}

	return true
}

// 辅助函数: 检查两个资源是否相同
func resourceEqual(a, b Resource) bool {
	if a.ID != b.ID || a.Type != b.Type {
		return false
	}

	// 检查属性是否相同
	if len(a.Attributes) != len(b.Attributes) {
		return false
	}
	for k, v := range a.Attributes {
		if b.Attributes[k] != v {
			return false
		}
	}

	return true
}

// 辅助函数: 检查主体是否匹配
func subjectMatch(pattern, subject Subject) bool {
	// 精确匹配ID
	if pattern.ID != subject.ID && pattern.ID != "*" {
		return false
	}

	// 匹配类型（如果指定）
	if pattern.Type != "" && pattern.Type != subject.Type {
		return false
	}

	// 匹配属性（如果指定）
	for k, v := range pattern.Attributes {
		if subject.Attributes[k] != v {
			return false
		}
	}

	return true
}

// 辅助函数: 检查资源是否匹配
func resourceMatch(pattern, resource Resource) bool {
	// 支持通配符和层次结构匹配
	if !matchResourceID(pattern.ID, resource.ID) {
		return false
	}

	// 匹配类型（如果指定）
	if pattern.Type != "" && pattern.Type != resource.Type {
		return false
	}

	// 匹配属性（如果指定）
	for k, v := range pattern.Attributes {
		if resource.Attributes[k] != v {
			return false
		}
	}

	return true
}

// 辅助函数: 匹配资源ID（支持通配符和层次结构）
func matchResourceID(pattern, resourceID string) bool {
	// 精确匹配
	if pattern == resourceID {
		return true
	}

	// 通配符匹配
	if pattern == "*" {
		return true
	}

	// 前缀通配符匹配（例如 "dir/*" 匹配 "dir/file1", "dir/subdir/file2" 等）
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-1] // 移除末尾的 "*"
		return strings.HasPrefix(resourceID, prefix)
	}

	// 层次结构匹配（例如 "/path/to/dir" 匹配 "/path/to/dir/file"）
	if strings.HasPrefix(resourceID, pattern+"/") {
		return true
	}

	return false
}

// 辅助函数: 检查操作是否匹配
func operationMatch(pattern, operation Operation) bool {
	return pattern == operation || pattern == "*"
}

// 辅助函数: 评估条件
func evaluateConditions(conditions map[string]interface{}, subject Subject, resource Resource, operation Operation) bool {
	if len(conditions) == 0 {
		return true
	}

	// 这里可以实现更复杂的条件评估逻辑
	// 当前实现只是一个占位符

	return true
}

// NewSubject 创建新的主体
func NewSubject(id string, subjectType SubjectType, attributes map[string]string) Subject {
	if attributes == nil {
		attributes = make(map[string]string)
	}
	return Subject{
		ID:         id,
		Type:       string(subjectType),
		Attributes: attributes,
	}
}

// NewResource 创建新的资源
func NewResource(id string, resourceType ResourceType, attributes map[string]string) Resource {
	if attributes == nil {
		attributes = make(map[string]string)
	}
	return Resource{
		ID:         id,
		Type:       resourceType,
		Attributes: attributes,
	}
}

// NewACLEntry 创建新的访问控制条目
func NewACLEntry(subject Subject, resource Resource, operation Operation, policy Policy) *ACLEntry {
	return &ACLEntry{
		Subject:    subject,
		Resource:   resource,
		Operation:  operation,
		Policy:     policy,
		Conditions: make(map[string]interface{}),
		CreatedAt:  time.Now(),
	}
}
