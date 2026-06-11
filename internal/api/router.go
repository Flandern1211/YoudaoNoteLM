package api

import (
	"YoudaoNoteLm/internal/api/v1/auth"
	"YoudaoNoteLm/internal/api/v1/importn"
	"YoudaoNoteLm/internal/api/v1/notebook"
	"YoudaoNoteLm/internal/api/v1/search"
	"YoudaoNoteLm/internal/api/v1/source"
	"YoudaoNoteLm/internal/api/v1/user"
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

// Router HTTP 路由装配器。
type Router struct {
	userCtrl       *user.Controller
	authCtrl       *auth.Controller
	notebookCtrl   *notebook.Controller
	sourceCtrl     *source.Controller
	searchCtrl     *search.Controller
	importCtrl     *importn.Controller
	tokenBlacklist service.TokenBlacklistService
}

// NewRouter 创建路由。
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	notebookService service.NotebookService,
	sourceService service.SourceService,
	searchService service.SearchService,
	importerService service.ImporterService,
	captchaSvc service.CaptchaService,
	tokenBlacklist service.TokenBlacklistService,
) *Router {
	return &Router{
		userCtrl:       user.NewController(userService, tokenBlacklist),
		authCtrl:       auth.NewController(authService, userService, captchaSvc),
		notebookCtrl:   notebook.NewController(notebookService),
		sourceCtrl:     source.NewController(sourceService, tokenBlacklist),
		searchCtrl:     search.NewController(searchService, importerService, tokenBlacklist),
		importCtrl:     importn.NewController(importerService),
		tokenBlacklist: tokenBlacklist,
	}
}

// Setup 注册所有路由。
func (r *Router) Setup(engine *gin.Engine) {
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	engine.Static("/uploads", "./uploads")

	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "YouDaoNoteLM API is running",
		})
	})

	v1 := engine.Group("/api/v1")
	{
		r.authCtrl.RegisterRoutes(v1)
		r.userCtrl.RegisterRoutes(v1)
		r.notebookCtrl.RegisterRoutes(v1, r.tokenBlacklist)
		r.sourceCtrl.RegisterRoutes(v1)
		r.searchCtrl.RegisterRoutes(v1)
		r.importCtrl.RegisterRoutes(v1, r.tokenBlacklist)
	}
}
