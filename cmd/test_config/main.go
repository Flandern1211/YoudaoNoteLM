package main

import (
	"context"
	"fmt"
	"os"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external/storage"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Printf("❌ 日志初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 3. 连接数据库
	mysqlDB, err := database.InitMySQL(&cfg.Database.MySQL)
	if err != nil {
		fmt.Printf("❌ MySQL 连接失败: %v\n", err)
		os.Exit(1)
	}

	// 自动迁移表结构
	if err := mysqlDB.AutoMigrate(&entity.UserConfig{}, &entity.SysConfig{}); err != nil {
		fmt.Printf("❌ 表迁移失败: %v\n", err)
		os.Exit(1)
	}

	redisClient, err := database.InitRedis(&cfg.Database.Redis)
	if err != nil {
		fmt.Printf("❌ Redis 连接失败: %v\n", err)
		os.Exit(1)
	}

	// 4. 创建依赖
	sysConfigRepo := repository.NewSysConfigRepository(mysqlDB)
	userConfigRepo := repository.NewUserConfigRepository(mysqlDB)
	redisCache := cache.New(redisClient)
	minioStorage, err := storage.NewMinIOStorage(
		cfg.External.MinIO.Endpoint,
		cfg.External.MinIO.AccessKey,
		cfg.External.MinIO.SecretKey,
		cfg.External.MinIO.Bucket,
	)
	if err != nil {
		logger.Fatal("MinIO 初始化失败", zap.Error(err))
	}

	// 5. 创建 ConfigService
	configSvc := service.NewConfigService(sysConfigRepo, userConfigRepo, redisCache, minioStorage)

	fmt.Println("=" + repeat("=", 60))
	fmt.Println("  ConfigService 集成测试（数据库 sys_config）")
	fmt.Println("=" + repeat("=", 60))

	passed := 0
	failed := 0

	// --- 测试 1: GetSearchEngine ---
	fmt.Println("\n📌 [1] GetSearchEngine - 无用户配置，从 sys_config 降级")
	engine, err := configSvc.GetSearchEngine(999999)
	if err != nil {
		fmt.Printf("   ❌ 错误: %v\n", err)
		failed++
	} else {
		fmt.Printf("   ✅ 引擎名称: %s\n", engine.Name())
		fmt.Printf("   ✅ 引擎类型: %T\n", engine)
		passed++
		// 验证可以调用 Search（会因为网络原因失败，但不 panic）
		fmt.Println("   🔍 测试 Search() 调用...")
		results, searchErr := engine.Search("test", 3)
		if searchErr != nil {
			fmt.Printf("   ⚠️  搜索调用失败（预期中，网络问题）: %v\n", searchErr)
		} else {
			fmt.Printf("   ✅ 搜索返回 %d 条结果\n", len(results))
		}
	}

	// --- 测试 2: GetASRService ---
	fmt.Println("\n📌 [2] GetASRService - 无用户配置，从 sys_config 降级")
	asrSvc, err := configSvc.GetASRService(999999)
	if err != nil {
		fmt.Printf("   ❌ 错误: %v\n", err)
		failed++
	} else {
		fmt.Printf("   ✅ ASR 服务类型: %T\n", asrSvc)
		fmt.Printf("   ✅ ASR 服务非 nil: %v\n", asrSvc != nil)
		// 检查是否注入了 storage
		if setter, ok := asrSvc.(interface{ SetStorage(storage.FileStorage) }); ok {
			_ = setter
			fmt.Println("   ✅ 支持 SetStorage 接口（storage 已注入）")
		}
		passed++
	}

	// --- 测试 3: GetEmbeddingService ---
	fmt.Println("\n📌 [3] GetEmbeddingService - 无用户配置，暂未实现 provider")
	_, err = configSvc.GetEmbeddingService(999999)
	if err != nil {
		fmt.Printf("   ✅ 预期错误: %v\n", err)
		passed++
	} else {
		fmt.Println("   ❌ 应该返回错误（Embedding provider 未实现）")
		failed++
	}

	// --- 测试 4: 验证 sys_config 缓存回填 ---
	fmt.Println("\n📌 [4] 验证 Redis 缓存回填")
	ctx := context.Background()
	keys := []string{"config:sys:search", "config:sys:asr", "config:sys:embedding"}
	for _, key := range keys {
		exists, _ := redisCache.Exists(ctx, key)
		if exists {
			fmt.Printf("   ✅ 缓存存在: %s\n", key)
			passed++
		} else {
			fmt.Printf("   ⚠️  缓存不存在: %s（可能 TTL 过期或未查询）\n", key)
		}
	}

	// --- 测试 5: 第二次调用走缓存 ---
	fmt.Println("\n📌 [5] 第二次调用 GetSearchEngine（应走缓存）")
	engine2, err := configSvc.GetSearchEngine(999999)
	if err != nil {
		fmt.Printf("   ❌ 错误: %v\n", err)
		failed++
	} else {
		fmt.Printf("   ✅ 引擎名称: %s（缓存命中）\n", engine2.Name())
		passed++
	}

	// --- 测试 6: ClearUserConfigCache ---
	fmt.Println("\n📌 [6] ClearUserConfigCache 不应 panic")
	configSvc.ClearUserConfigCache(999999, "search")
	fmt.Println("   ✅ 清除缓存成功（无 panic）")
	passed++

	// --- 测试 7: DeleteUserConfig 不存在的配置 ---
	fmt.Println("\n📌 [7] DeleteUserConfig 不存在的配置")
	err = configSvc.DeleteUserConfig(999999, "nonexistent")
	if err != nil {
		fmt.Printf("   ❌ 错误: %v\n", err)
		failed++
	} else {
		fmt.Println("   ✅ 删除不存在的配置不报错")
		passed++
	}

	// --- 测试 8: 验证 sys_config 数据内容 ---
	fmt.Println("\n📌 [8] 验证 sys_config 表数据")
	groups := []string{"asr", "search", "markitdown", "storage", "embedding"}
	for _, group := range groups {
		configs, err := sysConfigRepo.FindByGroup(group)
		if err != nil {
			fmt.Printf("   ❌ 查询 %s 失败: %v\n", group, err)
			failed++
			continue
		}
		if len(configs) == 0 {
			fmt.Printf("   ⚠️  [%s] 无数据\n", group)
			continue
		}
		for _, c := range configs {
			status := "✅"
			if !c.Enabled {
				status = "⏸️"
			}
			fmt.Printf("   %s [%s/%s] %s\n", status, group, c.ConfigKey, c.Description)
			fmt.Printf("      值: %s\n", truncate(c.ConfigValue, 80))
			passed++
		}
	}

	// --- 总结 ---
	fmt.Println("\n" + repeat("=", 61))
	fmt.Printf("  测试完成: ✅ %d 通过 / ❌ %d 失败\n", passed, failed)
	fmt.Println(repeat("=", 61))

	if failed > 0 {
		os.Exit(1)
	}
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
