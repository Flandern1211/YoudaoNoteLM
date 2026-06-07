package response

import "time"

// AdminUserResponse 管理员用户列表响应
type AdminUserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ConfigStatusGroupResponse 配置状态组响应
type ConfigStatusGroupResponse struct {
	Group       string `json:"group"`
	Total       int64  `json:"total"`
	Enabled     int64  `json:"enabled"`
	Description string `json:"description"`
}