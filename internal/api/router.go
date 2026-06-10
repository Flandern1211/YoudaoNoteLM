package api

import (
	"YoudaoNoteLm/internal/api/v1/admin"
	"YoudaoNoteLm/internal/api/v1/auth"
	"YoudaoNoteLm/internal/api/v1/importn"
	"YoudaoNoteLm/internal/api/v1/notebook"
	"YoudaoNoteLm/internal/api/v1/providers"
	search "YoudaoNoteLm/internal/api/v1/search"
	"YoudaoNoteLm/internal/api/v1/source"
	"YoudaoNoteLm/internal/api/v1/user"
	"YoudaoNoteLm/internal/api/v1/user_config"
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	userCtrl       *user.Controller
	authCtrl       *auth.Controller
	notebookCtrl   *notebook.Controller
	sourceCtrl     *source.Controller
	tokenBlacklist service.TokenBlacklistService
	importCtrl     *importn.Controller
	adminCtrl      *admin.Controller
	userCfgCtrl    *user_config.Controller
	searchCtrl     *search.Controller
	providerCtrl   *providers.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	notebookService service.NotebookService,
	sourceService service.SourceService,
	importerService service.ImporterService,
	adminService service.AdminService,
	userConfigService service.UserConfigService,
	searchAgentService service.SearchAgentService,
	captchaSvc service.CaptchaService,
	tokenBlacklist service.TokenBlacklistService,
	configService service.ConfigService,
) *Router {
	return &Router{
		userCtrl:       user.NewController(userService, tokenBlacklist),
		authCtrl:       auth.NewController(authService, userService, captchaSvc),
		notebookCtrl:   notebook.NewController(notebookService),
		sourceCtrl:     source.NewController(sourceService, tokenBlacklist),
		tokenBlacklist: tokenBlacklist,
		importCtrl:     importn.NewController(importerService),
		searchCtrl:     search.NewController(searchAgentService, tokenBlacklist),
		adminCtrl:      admin.NewController(adminService),
		userCfgCtrl:    user_config.NewController(userConfigService, tokenBlacklist),
		providerCtrl:   providers.NewController(configService),
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

		// 笔记本路由
		r.notebookCtrl.RegisterRoutes(v1, r.tokenBlacklist)

		// 资料来源路由（需认证）
		r.sourceCtrl.RegisterRoutes(v1)

		// 导入路由（需认证）
		r.importCtrl.RegisterRoutes(v1, r.tokenBlacklist)

		// 后台管理路由（需认证）
		r.adminCtrl.RegisterRoutes(v1)

		// 用户配置路由（需认证）
		r.userCfgCtrl.RegisterRoutes(v1)

		// 搜索路由（需认证）
		r.searchCtrl.RegisterRoutes(v1)

		// Provider 发现路由（/active 支持可选认证）
		r.providerCtrl.RegisterRoutes(v1, middleware.OptionalAuth(r.tokenBlacklist))
	}
}
