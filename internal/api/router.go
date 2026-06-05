package api

import (
	"YoudaoNoteLm/internal/api/v1/auth"
	"YoudaoNoteLm/internal/api/v1/source"
	"YoudaoNoteLm/internal/api/v1/user"
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	userCtrl       *user.Controller
	authCtrl       *auth.Controller
	tokenBlacklist service.TokenBlacklistService
	sourceCtrl     *source.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	sourceService service.SourceService,
	captchaSvc service.CaptchaService,
	tokenBlacklist service.TokenBlacklistService,
) *Router {
	return &Router{
		userCtrl:       user.NewController(userService, tokenBlacklist),
		authCtrl:       auth.NewController(authService, userService, captchaSvc),
		tokenBlacklist: tokenBlacklist,
		sourceCtrl:     source.NewController(sourceService, tokenBlacklist),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// 静态文件服务（头像等）
	engine.Static("/uploads", "./uploads")

	// 健康检查
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "YouDaoNoteLM API is running",
		})
	})

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// 资料来源路由（需认证）
		r.sourceCtrl.RegisterRoutes(v1)
	}
}
