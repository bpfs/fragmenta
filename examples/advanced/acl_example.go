package example

import (
	"context"
	"fmt"
	"time"

	"github.com/bpfs/fragmenta/security"
)

// 演示访问控制框架的使用
func AccessControlExample() {
	// 创建一个访问控制管理器
	acm := security.NewAccessControlManager()

	// 获取ACL和RBAC管理器
	aclManager := acm.GetACLManager()
	rbacManager := acm.GetRBACManager()

	// 示例场景：文件访问控制
	// 创建主体（用户）
	alice := security.NewSubject("alice", security.UserSubject, map[string]string{
		"department": "engineering",
	})

	bob := security.NewSubject("bob", security.UserSubject, map[string]string{
		"department": "marketing",
	})

	// 创建资源（文件和目录）
	projectDoc := security.NewResource("/projects/project1/doc.txt", security.FileResource, map[string]string{
		"owner": "alice",
	})

	sharedDir := security.NewResource("/shared", security.DirectoryResource, nil)

	// 1. 基于ACL的权限控制示例
	fmt.Println("===== ACL权限控制示例 =====")

	// 添加ACL条目：允许Alice读写她的项目文档
	aliceDocEntry := security.NewACLEntry(alice, projectDoc, security.ReadOperation, security.AllowPolicy)
	aclManager.AddEntry(context.Background(), aliceDocEntry)

	aliceWriteEntry := security.NewACLEntry(alice, projectDoc, security.WriteOperation, security.AllowPolicy)
	aclManager.AddEntry(context.Background(), aliceWriteEntry)

	// 添加ACL条目：允许所有用户读取共享目录
	allUsersReadShared := security.NewACLEntry(
		security.NewSubject("*", security.UserSubject, nil),
		sharedDir,
		security.ReadOperation,
		security.AllowPolicy,
	)
	aclManager.AddEntry(context.Background(), allUsersReadShared)

	// 检查权限
	// Alice读取她的文档
	allowed, _ := aclManager.CheckAccess(context.Background(), alice, projectDoc, security.ReadOperation)
	fmt.Printf("Alice读取她的文档: %v\n", allowed)

	// Bob读取Alice的文档
	allowed, _ = aclManager.CheckAccess(context.Background(), bob, projectDoc, security.ReadOperation)
	fmt.Printf("Bob读取Alice的文档: %v\n", allowed)

	// Bob读取共享目录
	allowed, _ = aclManager.CheckAccess(context.Background(), bob, sharedDir, security.ReadOperation)
	fmt.Printf("Bob读取共享目录: %v\n", allowed)

	// 使用过期时间的ACL条目
	// 添加一个1秒后过期的条目
	expireTime := time.Now().Add(1 * time.Second)
	tempEntry := security.NewACLEntry(bob, projectDoc, security.ReadOperation, security.AllowPolicy)
	tempEntry.ExpiresAt = &expireTime
	aclManager.AddEntry(context.Background(), tempEntry)

	// 立即检查（应该允许）
	allowed, _ = aclManager.CheckAccess(context.Background(), bob, projectDoc, security.ReadOperation)
	fmt.Printf("临时允许Bob读取Alice的文档: %v\n", allowed)

	// 等待条目过期
	time.Sleep(2 * time.Second)
	allowed, _ = aclManager.CheckAccess(context.Background(), bob, projectDoc, security.ReadOperation)
	fmt.Printf("过期后Bob读取Alice的文档: %v\n", allowed)

	// 2. 基于RBAC的权限控制示例
	fmt.Println("\n===== RBAC权限控制示例 =====")

	// 创建角色
	adminRole := security.NewRole("admin", "Administrator", "系统管理员角色")
	adminRole.AddPermission("*", "*", security.ReadOperation, security.WriteOperation, security.DeleteOperation, security.AdminOperation)

	editorRole := security.NewRole("editor", "Editor", "内容编辑角色")
	editorRole.AddPermission(security.FileResource, "/projects/*", security.ReadOperation, security.WriteOperation)
	editorRole.AddPermission(security.DirectoryResource, "/projects", security.ReadOperation, security.ListOperation)

	viewerRole := security.NewRole("viewer", "Viewer", "只读角色")
	viewerRole.AddPermission(security.FileResource, "/projects/*", security.ReadOperation)
	viewerRole.AddPermission(security.DirectoryResource, "/projects", security.ReadOperation, security.ListOperation)

	// 注册角色
	rbacManager.CreateRole(context.Background(), adminRole)
	rbacManager.CreateRole(context.Background(), editorRole)
	rbacManager.CreateRole(context.Background(), viewerRole)

	// 角色继承：编辑包含查看者权限
	editorRole.AddParentRole("viewer")
	rbacManager.UpdateRole(context.Background(), editorRole)

	// 分配角色
	rbacManager.AddRoleToSubject(context.Background(), "alice", "editor")
	rbacManager.AddRoleToSubject(context.Background(), "bob", "viewer")

	// 检查基于角色的权限
	// Alice（编辑者）写入项目文件
	allowed, _ = rbacManager.CheckPermission(context.Background(), "alice", projectDoc, security.WriteOperation)
	fmt.Printf("Alice(编辑者)写入项目文件: %v\n", allowed)

	// Bob（查看者）读取项目文件
	allowed, _ = rbacManager.CheckPermission(context.Background(), "bob", projectDoc, security.ReadOperation)
	fmt.Printf("Bob(查看者)读取项目文件: %v\n", allowed)

	// Bob（查看者）写入项目文件（应该被拒绝）
	allowed, _ = rbacManager.CheckPermission(context.Background(), "bob", projectDoc, security.WriteOperation)
	fmt.Printf("Bob(查看者)写入项目文件: %v\n", allowed)

	// 3. 使用AccessControlManager整合ACL和RBAC
	fmt.Println("\n===== 整合ACL和RBAC权限检查 =====")

	// 移除之前测试中的临时ACL条目
	aclManager.RemoveEntry(context.Background(), tempEntry)

	// 添加特定的ACL条目：明确拒绝Bob访问特定文件
	bobDenyEntry := security.NewACLEntry(bob, projectDoc, security.ReadOperation, security.DenyPolicy)
	aclManager.AddEntry(context.Background(), bobDenyEntry)

	// 检查整合权限：Bob尽管有viewer角色，但ACL明确拒绝访问特定文件
	allowed, _ = acm.CheckAccess(context.Background(), bob, projectDoc, security.ReadOperation)
	fmt.Printf("ACL拒绝+RBAC允许 => Bob读取项目文件: %v\n", allowed)

	// 添加另一个资源并检查权限
	anotherDoc := security.NewResource("/projects/project2/doc.txt", security.FileResource, nil)

	// Bob的viewer角色允许读取任何项目文件
	allowed, _ = acm.CheckAccess(context.Background(), bob, anotherDoc, security.ReadOperation)
	fmt.Printf("没有ACL规则+RBAC允许 => Bob读取其他项目文件: %v\n", allowed)

	// 修改角色并查看权限变更
	viewerRole.Permissions = []security.Permission{} // 清除所有权限
	viewerRole.AddPermission(security.DirectoryResource, "/shared/*", security.ReadOperation)
	rbacManager.UpdateRole(context.Background(), viewerRole)

	// 此时Bob无法访问项目文件
	allowed, _ = acm.CheckAccess(context.Background(), bob, anotherDoc, security.ReadOperation)
	fmt.Printf("角色权限变更后 => Bob读取项目文件: %v\n", allowed)
}
