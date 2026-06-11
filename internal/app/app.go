package app

import (
	searchAgent "YoudaoNoteLm/internal/agent/search"
	"YoudaoNoteLm/internal/api"
	"Youd
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	externalMarkitdown "YoudaoNoteLm/internal/service/external/markitdown"
	externalStorage "YoudaoNoteLm/internal/service/external/storage"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	"YoudaoNoteLm/pkg/logger"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// 触发 provider 注册（各子包 init() 会自动注册到全局 Registry）
	_ "YoudaoNoteLm/internal/service/external/asr"
	_ "YoudaoNoteLm/internal/service/external/embedding"
	_ "YoudaoNoteLm/internal/service/external/llm"
	_ "YoudaoNoteLm/internal/service/external/search"
	_ "YoudaoNoteLm/internal/service/external/storage"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 应用入口。
type App struct {
	cfg          *config.Config
	mysqlDB      *gorm.DB
	redis        *redis.Client
	router       *api.Router
	server       *http.Server
	ragRetriever rag.RAGRetriever
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Initialize 初始化应用
func (a *App) Initialize() error {
	// 1. 加载配置
	if err := a.initConfig(); err != nil {
		return err
	}

	// 2. 初始化日志
	if err := a.initLogger(); err != nil {
		return err
	}

	// 3. 初始化数据库
	if err := a.initDatabase(); err != nil {
		return err
	}

	// 4. 初始化依赖
	a.initDependencies()

	// 5. 初始化路由
	a.initRouter()

	// 6. 初始化服务器
	a.initServer()

	return nil
}

// initConfig 加载配置
func (a *App) initConfig() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	a.cfg = cfg
	return nil
}

// initLogger 初始化日志
func (a *App) initLogger() error {
	if err := logger.Init(&a.cfg.Log); err != nil {
		return fmt.Errorf("日志初始化失败: %w", err)
	}

	// 打印启动横幅
	logger.Info("=========================================")
	logger.Info(fmt.Sprintf("欢迎使用 %s", a.cfg.App.Name))
	logger.Info(fmt.Sprintf("版本: %s", a.cfg.App.Version))
	logger.Info(fmt.Sprintf("模式: %s", a.cfg.App.Mode))
	logger.Info("配置加载成功")
	logger.Info("=========================================")

	return nil
}

// initDatabase 初始化数据库
func (a *App) initDatabase() error {
	// 初始化 MySQL
	mysqlDB, err := database.InitMySQL(&a.cfg.Database.MySQL)
	if err != nil {
		return fmt.Errorf("MySQL 初始化失败: %w", err)
	}
	a.mysqlDB = mysqlDB

	// 自动迁移数据库表
	logger.Info("开始数据库迁移...")
	if err := a.mysqlDB.AutoMigrate(
		&entity.User{},
		&entity.Notebook{},
		&entity.Conversation{},
		&entity.Message{},
		&entity.Source{},
		&entity.ParentBlock{},
		&entity.UserConfig{},
		&entity.UserLLMConfig{},
		&entity.YoudaoBinding{},
		&entity.SysConfig{},
		&entity.Source{},
	); err != nil {
		logger.Warn("数据库迁移警告", zap.Error(err))
	} else {
		logger.Info("数据库迁移完成")
	}

	// 初始化 Redis（可选）
	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		logger.Warn("Redis 初始化失败，将不影响核心功能", zap.Error(err))
	}
	a.redis = rs

	return nil
}

// initDependencies 初始化依赖注入
func (a *App) initDependencies() {
	// 创建 Repository
	userRepo := repository.NewUserRepository(a.mysqlDB)
	notebookRepo := repository.NewNotebookRepository(a.mysqlDB)
	sourceRepo := repository.NewSourceRepository(a.mysqlDB)
	sysConfigRepo := repository.NewSysConfigRepository(a.mysqlDB)
	userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)

	// 创建 Service
	emailSvc := service.NewEmailService()
	verifyCodeSvc := service.NewVerifyCodeService(a.redis, emailSvc)
	captchaSvc := service.NewCaptchaService(a.redis)
	tokenBlacklistSvc := service.NewTokenBlacklistService(a.redis)
	userSvc := service.NewUserService(userRepo, verifyCodeSvc, minioStorage)
	authSvc := service.NewAuthService(userRepo, userSvc, verifyCodeSvc, captchaSvc, tokenBlacklistSvc)
	notebookSvc := service.NewNotebookService(notebookRepo)

	// 创建外部服务客户端（MarkItDown 直接从 config.yaml 读取，不通过 Provider Registry）
	markitdownClient := externalMarkitdown.NewClient(a.cfg.External.MarkItDown.URL)
	minioStorage, err := externalStorage.NewMinIOStorage(
		a.cfg.External.MinIO.Endpoint,
		a.cfg.External.MinIO.AccessKey,
		a.cfg.External.MinIO.SecretKey,
		a.cfg.External.MinIO.Bucket,
	)
	if err != nil {
		return fmt.Errorf("MinIO 初始化失败: %w", err)
	}

	// 创建 Service（依赖外部客户端）
	sourceSvc := service.NewSourceService(sourceRepo, minioStorage)

	var redisCache *cache.Cache
	if a.redis != nil {
		redisCache = cache.New(a.redis)
	}
	importTaskCache := cache.NewImportTaskCache(redisCache)
	audioPreviewCache := cache.NewAudioPreviewCache(redisCache)
	searchSvc := service.NewSearchService(bochaClient, userConfigRepo, redisCache, a.cfg.External.Bocha)

	// 创建 IngestionService
	ingestionSvc := a.initIngestionService(sourceRepo)
	if ingestionSvc == nil {
		logger.Warn("ingestion service unavailable, vector ingestion disabled")
	}

	// 创建 RAGRetriever
	if ingestionSvc != nil {
		userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)
		parentBlockRepo := repository.NewParentBlockRepository(a.mysqlDB)
		embedderProvider := func(ctx context.Context, userID uint) (embedding.Embedder, error) {
			cfg, err := userConfigRepo.FindByUserAndType(userID, "embedding")
			if err != nil {
				return nil, err
			}
			if cfg == nil {
				return nil, fmt.Errorf("用户 %d 未配置 Embedding", userID)
			}
			return rag.NewEmbedder(ctx, cfg)
		}
		// 创建独立的 MilvusWriter 用于检索（Milvus 客户端轻量）
		milvusWriter, err := rag.NewMilvusWriter(context.Background(), rag.MilvusIndexerConfig{
			Address: a.cfg.External.Milvus.Address,
		})
		if err != nil {
			logger.Warn("Milvus Writer 初始化失败，RAGRetriever 不可用", zap.Error(err))
		} else {
			a.ragRetriever = rag.NewRAGRetriever(
				milvusWriter,
				parentBlockRepo,
				sourceRepo,
				embedderProvider,
				5, // defaultTopK
			)
			logger.Info("RAGRetriever 初始化成功")
		}
	}

	// 创建 SourceService（需要 IngestionService 来删除向量数据）
	sourceSvc := service.NewSourceService(sourceRepo, ingestionSvc)

	// 创建导入服务
	// 创建 ConfigService（配置路由降级，管理 ASR/Search/LLM/Embedding 等动态服务）
	configSvc := service.NewConfigService(sysConfigRepo, userConfigRepo, redisCache, minioStorage)

	// 创建导入服务（ASR 通过 ConfigService 动态获取，EmbeddingService 暂时为 nil）
	importerSvc := service.NewImporterService(
		configSvc, markitdownClient, minioStorage,
		sourceRepo, importTaskCache, audioPreviewCache, nil,
	)

// initIngestionService 初始化入库服务
// 从数据库读取用户的 Embedding 配置，创建 EmbedderProvider 和 MilvusWriter
func (a *App) initIngestionService(sourceRepo repository.SourceRepository) rag.IngestionService {
	ctx := context.Background()

	// 创建 Milvus Writer
	milvusWriter, err := rag.NewMilvusWriter(ctx, rag.MilvusIndexerConfig{
		Address: a.cfg.External.Milvus.Address,
	})
	if err != nil {
		logger.Warn("init milvus writer failed", zap.Error(err))
		return nil
	}

	// 创建 EmbedderProvider：根据 userID 从数据库读取配置
	userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)
	embedderProvider := func(ctx context.Context, userID uint) (embedding.Embedder, error) {
		cfg, err := userConfigRepo.FindByUserAndType(userID, "embedding")
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, fmt.Errorf("user %d missing embedding config", userID)
		}
		return rag.NewEmbedder(ctx, cfg)
	}

	parentRepo := repository.NewParentBlockRepository(a.mysqlDB)
	ingestionSvc := rag.NewIngestionService(sourceRepo, parentRepo, embedderProvider, milvusWriter)
	logger.Info("IngestionService 初始化成功")
	return ingestionSvc
	// 创建后台管理服务
	adminSvc := service.NewAdminService(userRepo, sysConfigRepo, configSvc)

	// 创建用户配置服务（依赖 ConfigService）
	userCfgSvc := service.NewUserConfigService(userConfigRepo, configSvc)

	// 创建搜索 Agent（LLM 客户端在每次请求时通过 ConfigService 获取）
	searchagent := searchAgent.NewSearchAgent(configSvc, importerSvc)
	searchAgentSvc := service.NewSearchAgentService(configSvc, importerSvc, searchagent)

	// 创建有道云笔记 CLI 客户端（支持 .note 格式转换）
	youdaoCLI := externalYoudao.NewCLI(a.cfg.External.Youdao.CLIPath, a.cfg.External.Youdao.ConverterScriptPath)

	// 创建有道云笔记绑定 Repository
	youdaoBindingRepo := repository.NewYoudaoBindingRepository(a.mysqlDB)

	// 创建有道云笔记服务（EmbeddingService 暂时为 nil）
	youdaoSvc := service.NewYoudaoService(youdaoCLI, youdaoBindingRepo, sourceRepo, nil, a.cfg.External.Youdao.CookiesPath)

	// 创建 Router
	a.router = api.NewRouter(userSvc, authSvc, notebookSvc, sourceSvc, importerSvc, adminSvc, userCfgSvc, searchAgentSvc, captchaSvc, tokenBlacklistSvc, configSvc, youdaoSvc)

	return nil
}

// initRouter 初始化路由
func (a *App) initRouter() {
	// 设置 Gin 模式
	gin.SetMode(a.cfg.App.Mode)
}

// initServer 初始化 HTTP 服务器
func (a *App) initServer() {
	engine := gin.New()

	// 注册路由
	a.router.Setup(engine)

	// 创建 HTTP 服务器
	a.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:        engine,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
}

// Run 运行应用
func (a *App) Run() {
	// 启动 HTTP 服务器
	go func() {
		logger.Info("HTTP 服务器启动",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.App.Mode),
		)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 优雅关闭
	a.gracefulShutdown()
}

// gracefulShutdown 优雅关闭
func (a *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭失败", zap.Error(err))
	}

	// 关闭数据库连接
	if err := database.CloseMySQL(); err != nil {
		logger.Error("关闭 MySQL 连接失败", zap.Error(err))
	}
	if err := database.CloseRedis(); err != nil {
		logger.Error("关闭 Redis 连接失败", zap.Error(err))
	}

	// 同步日志
	if err := logger.Sync(); err != nil {
		logger.Error("同步日志失败", zap.Error(err))
	}

	logger.Info("服务器已关闭")
	logger.Info("=========================================")
}
