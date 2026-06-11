package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"YoudaoNoteLm/internal/api"
	"YoudaoNoteLm/internal/ingestion"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 应用入口。
type App struct {
	cfg     *config.Config
	mysqlDB *gorm.DB
	redis   *redis.Client
	router  *api.Router
	server  *http.Server
}

// NewApp 创建应用。
func NewApp() *App {
	return &App{}
}

// Initialize 初始化应用依赖。
func (a *App) Initialize() error {
	if err := a.initConfig(); err != nil {
		return err
	}
	if err := a.initLogger(); err != nil {
		return err
	}
	if err := a.initDatabase(); err != nil {
		return err
	}

	a.initDependencies()
	a.initRouter()
	a.initServer()
	return nil
}

func (a *App) initConfig() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	a.cfg = cfg
	return nil
}

func (a *App) initLogger() error {
	if err := logger.Init(&a.cfg.Log); err != nil {
		return fmt.Errorf("init logger failed: %w", err)
	}

	logger.Info("=========================================")
	logger.Info(fmt.Sprintf("starting %s", a.cfg.App.Name))
	logger.Info(fmt.Sprintf("version: %s", a.cfg.App.Version))
	logger.Info(fmt.Sprintf("mode: %s", a.cfg.App.Mode))
	logger.Info("config loaded")
	logger.Info("=========================================")
	return nil
}

func (a *App) initDatabase() error {
	mysqlDB, err := database.InitMySQL(&a.cfg.Database.MySQL)
	if err != nil {
		return fmt.Errorf("init mysql failed: %w", err)
	}
	a.mysqlDB = mysqlDB

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
	); err != nil {
		logger.Warn("database migration failed", zap.Error(err))
	}

	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		logger.Warn("init redis failed, continue without redis", zap.Error(err))
	}
	a.redis = rs
	return nil
}

func (a *App) initDependencies() {
	userRepo := repository.NewUserRepository(a.mysqlDB)
	notebookRepo := repository.NewNotebookRepository(a.mysqlDB)
	sourceRepo := repository.NewSourceRepository(a.mysqlDB)
	userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)

	emailSvc := service.NewEmailService()
	verifyCodeSvc := service.NewVerifyCodeService(a.redis, emailSvc)
	captchaSvc := service.NewCaptchaService(a.redis)
	tokenBlacklistSvc := service.NewTokenBlacklistService(a.redis)

	markitdownClient := external.NewMarkitdownClient(a.cfg.External.MarkItDown.URL)
	minioStorage := external.NewMinIOStorage(
		a.cfg.External.MinIO.Endpoint,
		a.cfg.External.MinIO.AccessKey,
		a.cfg.External.MinIO.SecretKey,
		a.cfg.External.MinIO.Bucket,
	)
	bochaClient := external.NewBochaSearchClient(&http.Client{}, a.cfg.External.Bocha.Endpoint)

	userSvc := service.NewUserService(userRepo, verifyCodeSvc, minioStorage)
	authSvc := service.NewAuthService(userRepo, userSvc, verifyCodeSvc, captchaSvc, tokenBlacklistSvc)
	notebookSvc := service.NewNotebookService(notebookRepo)

	asrSvc := external.NewASRService(a.cfg.External.ASR)
	if setter, ok := asrSvc.(interface{ SetStorage(external.FileStorage) }); ok {
		setter.SetStorage(minioStorage)
	}

	var redisCache *cache.Cache
	if a.redis != nil {
		redisCache = cache.New(a.redis)
	}
	importTaskCache := cache.NewImportTaskCache(redisCache)
	audioPreviewCache := cache.NewAudioPreviewCache(redisCache)
	searchSvc := service.NewSearchService(bochaClient, userConfigRepo, redisCache, a.cfg.External.Bocha)

	ingestionSvc := a.initIngestionService(sourceRepo)
	if ingestionSvc == nil {
		logger.Warn("ingestion service unavailable, vector ingestion disabled")
	}

	sourceSvc := service.NewSourceService(sourceRepo, ingestionSvc)
	importerSvc := service.NewImporterService(
		markitdownClient,
		asrSvc,
		minioStorage,
		sourceRepo,
		importTaskCache,
		audioPreviewCache,
		ingestionSvc,
	)

	a.router = api.NewRouter(
		userSvc,
		authSvc,
		notebookSvc,
		sourceSvc,
		searchSvc,
		importerSvc,
		captchaSvc,
		tokenBlacklistSvc,
	)
}

func (a *App) initIngestionService(sourceRepo repository.SourceRepository) ingestion.IngestionService {
	ctx := context.Background()

	milvusWriter, err := ingestion.NewMilvusWriter(ctx, ingestion.MilvusIndexerConfig{
		Address: a.cfg.External.Milvus.Address,
	})
	if err != nil {
		logger.Warn("init milvus writer failed", zap.Error(err))
		return nil
	}

	embedderProvider := func(ctx context.Context, userID uint) (embedding.Embedder, error) {
		userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)
		cfg, err := userConfigRepo.FindByUserAndType(userID, "embedding")
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, fmt.Errorf("user %d missing embedding config", userID)
		}
		return ingestion.NewEmbedder(ctx, cfg)
	}

	parentRepo := repository.NewParentBlockRepository(a.mysqlDB)
	return ingestion.NewIngestionService(sourceRepo, parentRepo, embedderProvider, milvusWriter)
}

func (a *App) initRouter() {
	gin.SetMode(a.cfg.App.Mode)
}

func (a *App) initServer() {
	engine := gin.New()
	a.router.Setup(engine)

	a.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:        engine,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

// Run 启动应用。
func (a *App) Run() {
	go func() {
		logger.Info("http server started",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.App.Mode),
		)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http server failed", zap.Error(err))
		}
	}()

	a.gracefulShutdown()
}

func (a *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("shutdown server failed", zap.Error(err))
	}
	if err := database.CloseMySQL(); err != nil {
		logger.Error("close mysql failed", zap.Error(err))
	}
	if err := database.CloseRedis(); err != nil {
		logger.Error("close redis failed", zap.Error(err))
	}
	_ = logger.Sync()
}
