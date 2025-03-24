package security

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Role 角色定义
type Role struct {
	// ID 角色标识符
	ID string

	// Name 角色名称
	Name string

	// Description 角色描述
	Description string

	// Permissions 角色拥有的权限
	Permissions []Permission

	// ParentRoles 父角色（支持角色继承）
	ParentRoles []string

	// CreatedAt 创建时间
	CreatedAt time.Time

	// UpdatedAt 更新时间
	UpdatedAt time.Time
}

// Permission 权限定义
type Permission struct {
	// Resource 资源类型
	Resource ResourceType

	// ResourcePattern 资源匹配模式
	ResourcePattern string

	// Operations 允许的操作
	Operations []Operation
}

// RBACManager 基于角色的访问控制管理接口
type RBACManager interface {
	// CreateRole 创建角色
	CreateRole(ctx context.Context, role *Role) error

	// UpdateRole 更新角色
	UpdateRole(ctx context.Context, role *Role) error

	// DeleteRole 删除角色
	DeleteRole(ctx context.Context, roleID string) error

	// GetRole 获取角色
	GetRole(ctx context.Context, roleID string) (*Role, error)

	// ListRoles 列出所有角色
	ListRoles(ctx context.Context) ([]*Role, error)

	// AddRoleToSubject 为主体分配角色
	AddRoleToSubject(ctx context.Context, subjectID string, roleID string) error

	// RemoveRoleFromSubject 移除主体的角色
	RemoveRoleFromSubject(ctx context.Context, subjectID string, roleID string) error

	// GetSubjectRoles 获取主体的所有角色
	GetSubjectRoles(ctx context.Context, subjectID string) ([]string, error)

	// CheckPermission 检查主体是否拥有指定权限
	CheckPermission(ctx context.Context, subjectID string, resource Resource, operation Operation) (bool, error)
}

// DefaultRBACManager 默认的RBAC管理实现
type DefaultRBACManager struct {
	// 角色定义
	roles map[string]*Role

	// 主体-角色映射
	subjectRoles map[string][]string

	// 针对role的互斥锁
	roleMutex sync.RWMutex

	// 针对subject-role映射的互斥锁
	subjectRoleMutex sync.RWMutex
}

// NewDefaultRBACManager 创建默认的RBAC管理器
func NewDefaultRBACManager() *DefaultRBACManager {
	return &DefaultRBACManager{
		roles:        make(map[string]*Role),
		subjectRoles: make(map[string][]string),
	}
}

// CreateRole 创建角色
func (m *DefaultRBACManager) CreateRole(ctx context.Context, role *Role) error {
	if role == nil {
		return errors.New("role cannot be nil")
	}
	if role.ID == "" {
		return errors.New("role ID cannot be empty")
	}

	m.roleMutex.Lock()
	defer m.roleMutex.Unlock()

	// 检查角色是否已存在
	if _, exists := m.roles[role.ID]; exists {
		return fmt.Errorf("role with ID %s already exists", role.ID)
	}

	// 设置创建和更新时间
	now := time.Now()
	role.CreatedAt = now
	role.UpdatedAt = now

	// 保存角色
	m.roles[role.ID] = role
	return nil
}

// UpdateRole 更新角色
func (m *DefaultRBACManager) UpdateRole(ctx context.Context, role *Role) error {
	if role == nil {
		return errors.New("role cannot be nil")
	}
	if role.ID == "" {
		return errors.New("role ID cannot be empty")
	}

	m.roleMutex.Lock()
	defer m.roleMutex.Unlock()

	// 检查角色是否存在
	existingRole, exists := m.roles[role.ID]
	if !exists {
		return fmt.Errorf("role with ID %s does not exist", role.ID)
	}

	// 保留创建时间
	role.CreatedAt = existingRole.CreatedAt
	// 更新修改时间
	role.UpdatedAt = time.Now()

	// 更新角色
	m.roles[role.ID] = role
	return nil
}

// DeleteRole 删除角色
func (m *DefaultRBACManager) DeleteRole(ctx context.Context, roleID string) error {
	if roleID == "" {
		return errors.New("role ID cannot be empty")
	}

	m.roleMutex.Lock()
	defer m.roleMutex.Unlock()

	// 检查角色是否存在
	if _, exists := m.roles[roleID]; !exists {
		return fmt.Errorf("role with ID %s does not exist", roleID)
	}

	// 删除角色
	delete(m.roles, roleID)

	// 从所有主体中移除该角色
	m.subjectRoleMutex.Lock()
	defer m.subjectRoleMutex.Unlock()

	for subjectID, roles := range m.subjectRoles {
		newRoles := make([]string, 0, len(roles))
		for _, r := range roles {
			if r != roleID {
				newRoles = append(newRoles, r)
			}
		}
		m.subjectRoles[subjectID] = newRoles
	}

	return nil
}

// GetRole 获取角色
func (m *DefaultRBACManager) GetRole(ctx context.Context, roleID string) (*Role, error) {
	if roleID == "" {
		return nil, errors.New("role ID cannot be empty")
	}

	m.roleMutex.RLock()
	defer m.roleMutex.RUnlock()

	// 获取角色
	role, exists := m.roles[roleID]
	if !exists {
		return nil, fmt.Errorf("role with ID %s does not exist", roleID)
	}

	return role, nil
}

// ListRoles 列出所有角色
func (m *DefaultRBACManager) ListRoles(ctx context.Context) ([]*Role, error) {
	m.roleMutex.RLock()
	defer m.roleMutex.RUnlock()

	// 列出所有角色
	roles := make([]*Role, 0, len(m.roles))
	for _, role := range m.roles {
		roles = append(roles, role)
	}

	return roles, nil
}

// AddRoleToSubject 为主体分配角色
func (m *DefaultRBACManager) AddRoleToSubject(ctx context.Context, subjectID string, roleID string) error {
	if subjectID == "" {
		return errors.New("subject ID cannot be empty")
	}
	if roleID == "" {
		return errors.New("role ID cannot be empty")
	}

	// 验证角色是否存在
	m.roleMutex.RLock()
	_, exists := m.roles[roleID]
	m.roleMutex.RUnlock()
	if !exists {
		return fmt.Errorf("role with ID %s does not exist", roleID)
	}

	m.subjectRoleMutex.Lock()
	defer m.subjectRoleMutex.Unlock()

	// 获取主体的角色列表
	roles, exists := m.subjectRoles[subjectID]
	if !exists {
		// 主体不存在，创建角色列表
		m.subjectRoles[subjectID] = []string{roleID}
		return nil
	}

	// 检查角色是否已分配给该主体
	for _, r := range roles {
		if r == roleID {
			return nil // 角色已存在，直接返回
		}
	}

	// 添加角色到主体
	m.subjectRoles[subjectID] = append(roles, roleID)
	return nil
}

// RemoveRoleFromSubject 移除主体的角色
func (m *DefaultRBACManager) RemoveRoleFromSubject(ctx context.Context, subjectID string, roleID string) error {
	if subjectID == "" {
		return errors.New("subject ID cannot be empty")
	}
	if roleID == "" {
		return errors.New("role ID cannot be empty")
	}

	m.subjectRoleMutex.Lock()
	defer m.subjectRoleMutex.Unlock()

	// 获取主体的角色列表
	roles, exists := m.subjectRoles[subjectID]
	if !exists {
		return nil // 主体不存在，无需操作
	}

	// 移除角色
	found := false
	newRoles := make([]string, 0, len(roles))
	for _, r := range roles {
		if r != roleID {
			newRoles = append(newRoles, r)
		} else {
			found = true
		}
	}

	if !found {
		return nil // 角色不存在，无需操作
	}

	// 更新主体的角色列表
	if len(newRoles) == 0 {
		delete(m.subjectRoles, subjectID) // 如果角色列表为空，删除主体
	} else {
		m.subjectRoles[subjectID] = newRoles
	}

	return nil
}

// GetSubjectRoles 获取主体的所有角色
func (m *DefaultRBACManager) GetSubjectRoles(ctx context.Context, subjectID string) ([]string, error) {
	if subjectID == "" {
		return nil, errors.New("subject ID cannot be empty")
	}

	m.subjectRoleMutex.RLock()
	defer m.subjectRoleMutex.RUnlock()

	// 获取主体的角色列表
	roles, exists := m.subjectRoles[subjectID]
	if !exists {
		return []string{}, nil // 主体不存在，返回空列表
	}

	// 获取包括继承关系的所有角色
	allRoles := make(map[string]bool)
	for _, roleID := range roles {
		m.collectRoles(roleID, allRoles)
	}

	// 转换为字符串列表
	result := make([]string, 0, len(allRoles))
	for roleID := range allRoles {
		result = append(result, roleID)
	}

	return result, nil
}

// 递归收集所有角色（包括父角色）
func (m *DefaultRBACManager) collectRoles(roleID string, result map[string]bool) {
	m.roleMutex.RLock()
	defer m.roleMutex.RUnlock()

	// 如果角色已存在于结果中，避免循环引用
	if result[roleID] {
		return
	}

	// 添加当前角色
	result[roleID] = true

	// 如果角色不存在，直接返回
	role, exists := m.roles[roleID]
	if !exists {
		return
	}

	// 递归添加父角色
	for _, parentRoleID := range role.ParentRoles {
		m.collectRoles(parentRoleID, result)
	}
}

// CheckPermission 检查主体是否拥有指定权限
func (m *DefaultRBACManager) CheckPermission(ctx context.Context, subjectID string, resource Resource, operation Operation) (bool, error) {
	if subjectID == "" {
		return false, errors.New("subject ID cannot be empty")
	}

	// 获取主体的所有角色（包括继承关系）
	allRoles, err := m.GetSubjectRoles(ctx, subjectID)
	if err != nil {
		return false, err
	}

	m.roleMutex.RLock()
	defer m.roleMutex.RUnlock()

	// 检查是否有任一角色具有请求的权限
	for _, roleID := range allRoles {
		role, exists := m.roles[roleID]
		if !exists {
			continue
		}

		// 检查角色的权限
		if hasPermission(role, resource, operation) {
			return true, nil
		}
	}

	return false, nil
}

// 检查指定角色是否有特定权限
func hasPermission(role *Role, resource Resource, operation Operation) bool {
	for _, permission := range role.Permissions {
		// 检查资源类型是否匹配
		if permission.Resource != "" && permission.Resource != resource.Type && permission.Resource != "*" {
			continue
		}

		// 检查资源ID是否匹配模式
		if permission.ResourcePattern != "" && !matchResourceID(permission.ResourcePattern, resource.ID) {
			continue
		}

		// 检查操作是否被允许
		for _, op := range permission.Operations {
			if op == operation || op == "*" {
				return true
			}
		}
	}

	return false
}

// NewRole 创建新的角色
func NewRole(id string, name string, description string) *Role {
	return &Role{
		ID:          id,
		Name:        name,
		Description: description,
		Permissions: make([]Permission, 0),
		ParentRoles: make([]string, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// AddPermission 为角色添加权限
func (r *Role) AddPermission(resourceType ResourceType, resourcePattern string, operations ...Operation) {
	permission := Permission{
		Resource:        resourceType,
		ResourcePattern: resourcePattern,
		Operations:      operations,
	}
	r.Permissions = append(r.Permissions, permission)
}

// AddParentRole 添加父角色（继承关系）
func (r *Role) AddParentRole(parentRoleID string) {
	// 检查是否已存在
	for _, id := range r.ParentRoles {
		if id == parentRoleID {
			return
		}
	}
	r.ParentRoles = append(r.ParentRoles, parentRoleID)
}

// AccessControlManager 访问控制管理器
// 整合ACL和RBAC功能
type AccessControlManager struct {
	aclManager  ACLManager
	rbacManager RBACManager
}

// NewAccessControlManager 创建访问控制管理器
func NewAccessControlManager() *AccessControlManager {
	return &AccessControlManager{
		aclManager:  NewDefaultACLManager(),
		rbacManager: NewDefaultRBACManager(),
	}
}

// GetACLManager 获取ACL管理器
func (m *AccessControlManager) GetACLManager() ACLManager {
	return m.aclManager
}

// GetRBACManager 获取RBAC管理器
func (m *AccessControlManager) GetRBACManager() RBACManager {
	return m.rbacManager
}

// CheckAccess 检查访问权限
// 同时使用ACL和RBAC进行检查
func (m *AccessControlManager) CheckAccess(ctx context.Context, subject Subject, resource Resource, operation Operation) (bool, error) {
	// 首先检查ACL
	allowed, err := m.aclManager.CheckAccess(ctx, subject, resource, operation)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil // ACL允许访问
	}

	// 如果ACL未明确允许，检查RBAC权限
	rbacAllowed, err := m.rbacManager.CheckPermission(ctx, subject.ID, resource, operation)
	if err != nil {
		return false, err
	}

	// 返回基于RBAC的结果
	return rbacAllowed, nil
}
