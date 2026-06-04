package middleware

import (
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/jwt"
	"YoudaoNoteLm/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// ContextUserID 用户 ID 上下文键
	ContextUserID = "user_id"
	// ContextUsername 用户名 上下文键
	ContextUsername = "username"
)

// Auth JWT 认证中间件（仅接受 Access Token，检查黑名单）
func Auth(blacklist service.TokenBlacklistService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "请提供认证令牌")
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "令牌格式错误")
			c.Abort()
			return
		}

		// 解析 Token
		claims, err := jwt.ParseToken(parts[1])
		if err != nil {
			response.Unauthorized(c, err.Error())
			c.Abort()
			return
		}

		// 必须是 access token
		if claims.TokenType != jwt.AccessToken {
			response.Unauthorized(c, "请使用 access_token 进行认证")
			c.Abort()
			return
		}

		// 检查 token 是否已被吊销
		revoked, err := blacklist.IsRevoked(c.Request.Context(), claims.ID)
		if err != nil {
			response.InternalError(c, "验证令牌状态失败")
			c.Abort()
			return
		}
		if revoked {
			response.Unauthorized(c, "令牌已失效，请重新登录")
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(ContextUserID, claims.GetUserID())
		c.Set(ContextUsername, claims.GetUsername())

		c.Next()
	}
}

// GetUserID 从上下文获取用户 ID
func GetUserID(c *gin.Context) uint {
	if userID, exists := c.Get(ContextUserID); exists {
		return userID.(uint)
	}
	return 0
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(ContextUsername); exists {
		return username.(string)
	}
	return ""
}

// OptionalAuth 可选的 JWT 认证中间件（仅接受 Access Token，检查黑名单）
func OptionalAuth(blacklist service.TokenBlacklistService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := jwt.ParseToken(parts[1])
		if err == nil && claims.TokenType == jwt.AccessToken {
			// 检查黑名单
			revoked, _ := blacklist.IsRevoked(c.Request.Context(), claims.ID)
			if !revoked {
				c.Set(ContextUserID, claims.GetUserID())
				c.Set(ContextUsername, claims.GetUsername())
			}
		}

		c.Next()
	}
}
