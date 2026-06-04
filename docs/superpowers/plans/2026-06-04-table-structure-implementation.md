# 表结构实现计划：笔记本、会话、消息、父块

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现四张核心表（notebooks、conversations、messages、parent_blocks）的实体定义，并将 BaseEntity 主键从 uint 改为 int

**Architecture:** 扁平关联设计，所有表通过外键直接关联。父块存 MySQL 原文，子块向量存 Milvus。BaseEntity.ID 从 uint 改为 int，影响所有现有实体和引用。

**Tech Stack:** Go, GORM, MySQL, gin, jwt

---

## 文件结构

### 新建文件
| 文件 | 职责 |
|------|------|
| `internal/model/entity/notebook.go` | 笔记本实体 |
| `internal/model/entity/conversation.go` | 会话实体 |
| `internal/model/entity/message.go` | 消息实体 |
| `internal/model/entity/parent_block.go` | 父块实体 |

### 修改文件
| 文件 | 修改内容 |
|------|----------|
| `internal/model/entity/base.go` | ID 从 uint 改为 int |
| `internal/model/entity/user.go` | 同步 ID 类型 |
| `pkg/jwt/claims.go` | UserID 从 uint 改为 int |
| `pkg/jwt/jwt.go` | GenerateTokenPair 参数类型 |
| `internal/middleware/auth.go` | GetUserID 返回 int |
| `internal/repository/user_interface.go` | 方法参数 id 从 uint 改为 int |
| `internal/repository/user_repository.go` | 方法参数 id 从 uint 改为 int |
| `internal/service/auth_service.go` | claims.UserID 类型适配 |
| `internal/api/v1/user/controller.go` | userID 类型适配 |
| `internal/app/app.go` | AutoMigrate 添加新表 |

---

## Task 1: 修改 BaseEntity 主键类型

**Files:**
- Modify: `internal/model/entity/base.go`

- [ ] **Step 1: 修改 BaseEntity.ID 类型**

```go
package entity

import (
	"time"

	"gorm.io/gorm"
)

// BaseEntity 基础实体
type BaseEntity struct {
	ID        int            `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate GORM hook - 创建前
func (b *BaseEntity) BeforeCreate(tx *gorm.DB) error {
	return nil
}

// BeforeUpdate GORM hook - 更新前
func (b *BaseEntity) BeforeUpdate(tx *gorm.DB) error {
	return nil
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译失败（因为其他文件还在用 uint）

- [ ] **Step 3: Commit**

```bash
git add internal/model/entity/base.go
git commit -m "refactor: change BaseEntity.ID from uint to int"
```

---

## Task 2: 修改 User 实体和 JWT Claims

**Files:**
- Modify: `internal/model/entity/user.go`
- Modify: `pkg/jwt/claims.go`
- Modify: `pkg/jwt/jwt.go`

- [ ] **Step 1: 修改 User 实体**

```go
package entity

import "time"

// User 用户实体
type User struct {
	BaseEntity
	Username       string     `gorm:"type:varchar(50);uniqueIndex;not null;comment:用户名" json:"username"`
	Password       string     `gorm:"type:varchar(255);not null;comment:密码" json:"-"`
	Email          string     `gorm:"type:varchar(100);uniqueIndex;not null;comment:邮箱" json:"email"`
	Nickname       string     `gorm:"type:varchar(50);comment:昵称" json:"nickname"`
	Avatar         string     `gorm:"type:varchar(255);comment:头像" json:"avatar"`
	Status         int        `gorm:"type:tinyint;default:1;comment:状态:1正常,2禁用" json:"status"`
	FailedAttempts int        `gorm:"type:tinyint;default:0;comment:连续登录失败次数" json:"-"`
	LockedUntil    *time.Time `gorm:"comment:锁定截止时间" json:"-"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// IsLocked 判断用户是否被锁定
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}
```

- [ ] **Step 2: 修改 JWT Claims**

```go
package jwt

import "github.com/golang-jwt/jwt/v5"

// TokenType token 类型
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// CustomClaims 自定义 JWT Claims
type CustomClaims struct {
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	TokenType TokenType `json:"token_type"`
	jwt.RegisteredClaims
}

// GetUserID 获取用户 ID
func (c *CustomClaims) GetUserID() int {
	return c.UserID
}

// GetUsername 获取用户名
func (c *CustomClaims) GetUsername() string {
	return c.Username
}

// GetTokenType 获取 token 类型
func (c *CustomClaims) GetTokenType() TokenType {
	return c.TokenType
}
```

- [ ] **Step 3: 修改 JWT 生成函数参数类型**

在 `pkg/jwt/jwt.go` 中，将 `GenerateAccessToken`、`GenerateRefreshToken`、`GenerateTokenPair` 的 `userID` 参数从 `uint` 改为 `int`：

```go
// GenerateAccessToken 生成 Access Token（15 分钟）
func GenerateAccessToken(userID int, username string) (string, error) {
	// ... 其余代码不变
}

// GenerateRefreshToken 生成 Refresh Token（7 天）
func GenerateRefreshToken(userID int, username string) (string, error) {
	// ... 其余代码不变
}

// GenerateTokenPair 生成 Access + Refresh Token 对
func GenerateTokenPair(userID int, username string) (*TokenPair, error) {
	// ... 其余代码不变
}
```

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 编译失败（middleware 和 repository 还在用 uint）

- [ ] **Step 5: Commit**

```bash
git add internal/model/entity/user.go pkg/jwt/claims.go pkg/jwt/jwt.go
git commit -m "refactor: change UserID from uint to int in User and JWT"
```

---

## Task 3: 修改 Middleware 和 Repository

**Files:**
- Modify: `internal/middleware/auth.go`
- Modify: `internal/repository/user_interface.go`
- Modify: `internal/repository/user_repository.go`

- [ ] **Step 1: 修改 Middleware GetUserID**

```go
// GetUserID 从上下文获取用户 ID
func GetUserID(c *gin.Context) int {
	if userID, exists := c.Get(ContextUserID); exists {
		return userID.(int)
	}
	return 0
}
```

- [ ] **Step 2: 修改 UserRepository 接口**

```go
// UserRepository 用户仓储接口
type UserRepository interface {
	// FindByID 根据 ID 查找用户
	FindByID(id int) (*entity.User, error)
	// ... 其他方法
	// Delete 删除用户
	Delete(id int) error
	// UpdateLoginAttempts 更新登录失败次数
	UpdateLoginAttempts(id int, attempts int) error
	// LockUser 锁定用户到指定时间
	LockUser(id int, until time.Time) error
	// ResetLoginAttempts 重置登录失败次数
	ResetLoginAttempts(id int) error
}
```

- [ ] **Step 3: 修改 UserRepository 实现**

将 `FindByID`、`Delete`、`UpdateLoginAttempts`、`LockUser`、`ResetLoginAttempts` 的 `id` 参数从 `uint` 改为 `int`。

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/auth.go internal/repository/user_interface.go internal/repository/user_repository.go
git commit -m "refactor: update middleware and repository to use int ID"
```

---

## Task 4: 创建笔记本实体

**Files:**
- Create: `internal/model/entity/notebook.go`

- [ ] **Step 1: 创建 Notebook 实体**

```go
package entity

// Notebook 笔记本实体
type Notebook struct {
	BaseEntity
	UserID int    `gorm:"index;not null;comment:所属用户ID"`
	Name   string `gorm:"type:varchar(100);not null;comment:笔记本名称"`
}

// TableName 指定表名
func (Notebook) TableName() string {
	return "notebooks"
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/model/entity/notebook.go
git commit -m "feat: add Notebook entity"
```

---

## Task 5: 创建会话实体

**Files:**
- Create: `internal/model/entity/conversation.go`

- [ ] **Step 1: 创建 Conversation 实体**

```go
package entity

// Conversation 会话实体
type Conversation struct {
	BaseEntity
	NotebookID int    `gorm:"index;not null;comment:所属笔记本ID"`
	Title      string `gorm:"type:varchar(100);not null;comment:会话标题"`
}

// TableName 指定表名
func (Conversation) TableName() string {
	return "conversations"
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/model/entity/conversation.go
git commit -m "feat: add Conversation entity"
```

---

## Task 6: 创建消息实体

**Files:**
- Create: `internal/model/entity/message.go`

- [ ] **Step 1: 创建 Message 实体**

```go
package entity

// Message 消息实体
type Message struct {
	BaseEntity
	ConversationID int    `gorm:"index;not null;comment:所属会话ID"`
	Role           string `gorm:"type:varchar(20);not null;comment:角色:user/assistant/system"`
	Content        string `gorm:"type:text;not null;comment:消息内容"`
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/model/entity/message.go
git commit -m "feat: add Message entity"
```

---

## Task 7: 创建父块实体

**Files:**
- Create: `internal/model/entity/parent_block.go`

- [ ] **Step 1: 创建 ParentBlock 实体**

```go
package entity

// ParentBlock 父块实体（资料来源的父子分块中的父块）
type ParentBlock struct {
	BaseEntity
	SourceID   int    `gorm:"index;not null;comment:所属资料来源ID"`
	Heading    string `gorm:"type:varchar(255);comment:父块标题/小标题"`
	Content    string `gorm:"type:text;not null;comment:父块原文内容"`
	ChunkIndex int    `gorm:"not null;comment:父块在来源中的序号(从0开始)"`
	Metadata   string `gorm:"type:json;comment:元数据JSON(页码/章节等)"`
}

// TableName 指定表名
func (ParentBlock) TableName() string {
	return "parent_blocks"
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/model/entity/parent_block.go
git commit -m "feat: add ParentBlock entity"
```

---

## Task 8: 更新 AutoMigrate

**Files:**
- Modify: `internal/app/app.go`

- [ ] **Step 1: 添加新表到 AutoMigrate**

在 `initDatabase` 方法中，将新实体添加到 AutoMigrate：

```go
// 自动迁移数据库表
logger.Info("开始数据库迁移...")
if err := a.mysqlDB.AutoMigrate(
	&entity.User{},
	&entity.Notebook{},
	&entity.Conversation{},
	&entity.Message{},
	&entity.ParentBlock{},
); err != nil {
	logger.Warn("数据库迁移警告", zap.Error(err))
} else {
	logger.Info("数据库迁移完成")
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/app/app.go
git commit -m "feat: add new tables to AutoMigrate"
```

---

## Task 9: 最终验证

- [ ] **Step 1: 运行 go vet**

Run: `go vet ./...`
Expected: 无输出（通过）

- [ ] **Step 2: 运行 go build**

Run: `go build ./...`
Expected: 无输出（通过）

- [ ] **Step 3: 验证数据库表结构**

启动应用，检查 MySQL 中是否成功创建了四张新表：

```sql
SHOW TABLES LIKE 'notebooks';
SHOW TABLES LIKE 'conversations';
SHOW TABLES LIKE 'messages';
SHOW TABLES LIKE 'parent_blocks';
```

- [ ] **Step 4: 最终 Commit**

```bash
git add -A
git commit -m "feat: complete table structure implementation for notebooks, conversations, messages, parent_blocks"
```
