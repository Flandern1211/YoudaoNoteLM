package main

import (
	"context"
	"fmt"
	"os"

	searchAgent "YoudaoNoteLm/internal/agent/search"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	"YoudaoNoteLm/pkg/logger"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func main() {
	cfg, _ := config.Load("")
	_ = logger.Init(&cfg.Log)
	db, _ := database.InitMySQL(&cfg.Database.MySQL)
	rs, _ := database.InitRedis(&cfg.Database.Redis)

	sysConfigRepo := repository.NewSysConfigRepository(db)
	userConfigRepo := repository.NewUserConfigRepository(db)
	redisCache := cache.New(rs)

	// 用真正的 ConfigService
	configSvc := service.NewConfigService(sysConfigRepo, userConfigRepo, redisCache)

	// mock ImporterService
	agent := searchAgent.NewSearchAgent(configSvc, nil)
	_ = &entity.Source{}
	_ = &gorm.DB{}
	_ = redis.NewClient(&redis.Options{})

	ctx := context.Background()
	result, err := agent.Execute(ctx, 1, 1, "搜索 Go 语言 1.22 版本的新特性，用中文总结")
	if err != nil {
		fmt.Printf("❌ 搜索 Agent 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 搜索轮数: %d\n\n", result.SearchRounds)
	fmt.Println(result.Content)
}
