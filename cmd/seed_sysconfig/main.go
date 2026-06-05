package main

import (
	"encoding/json"
	"fmt"
	"os"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	"YoudaoNoteLm/pkg/logger"

	"gorm.io/gorm"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 2. 连接数据库
	db, err := database.InitMySQL(&cfg.Database.MySQL)
	if err != nil {
		fmt.Printf("连接数据库失败: %v\n", err)
		os.Exit(1)
	}

	// 3. 自动迁移表结构
	if err := db.AutoMigrate(&entity.SysConfig{}); err != nil {
		fmt.Printf("迁移表结构失败: %v\n", err)
		os.Exit(1)
	}

	// 4. 灌入配置数据
	seedConfigs := buildSeedConfigs(cfg)

	for _, sc := range seedConfigs {
		if err := upsertSysConfig(db, sc); err != nil {
			fmt.Printf("写入配置失败 [%s/%s]: %v\n", sc.ConfigGroup, sc.ConfigKey, err)
			os.Exit(1)
		}
		fmt.Printf("✅ [%s/%s] %s\n", sc.ConfigGroup, sc.ConfigKey, sc.Description)
	}

	fmt.Println("\n所有系统配置已写入 sys_config 表")
}

func buildSeedConfigs(cfg *config.Config) []entity.SysConfig {
	var configs []entity.SysConfig

	// --- ASR 配置 ---
	asrValue, _ := json.Marshal(map[string]interface{}{
		"provider":            cfg.External.ASR.Provider,
		"access_key_id":       cfg.External.ASR.GetString("access_key_id"),
		"access_key_secret":   cfg.External.ASR.GetString("access_key_secret"),
		"app_key":             cfg.External.ASR.GetString("app_key"),
	})
	configs = append(configs, entity.SysConfig{
		ConfigGroup: "asr",
		ConfigKey:   cfg.External.ASR.Provider,
		ConfigValue: string(asrValue),
		Enabled:     true,
		Description: "阿里云智能语音 ASR 服务",
	})

	// --- MarkItDown 配置 ---
	markitdownValue, _ := json.Marshal(map[string]interface{}{
		"url": cfg.External.MarkItDown.URL,
	})
	configs = append(configs, entity.SysConfig{
		ConfigGroup: "markitdown",
		ConfigKey:   "default",
		ConfigValue: string(markitdownValue),
		Enabled:     true,
		Description: "MarkItDown 文档转换服务",
	})

	// --- MinIO 存储配置 ---
	minioValue, _ := json.Marshal(map[string]interface{}{
		"endpoint":   cfg.External.MinIO.Endpoint,
		"access_key": cfg.External.MinIO.AccessKey,
		"secret_key": cfg.External.MinIO.SecretKey,
		"bucket":     cfg.External.MinIO.Bucket,
	})
	configs = append(configs, entity.SysConfig{
		ConfigGroup: "storage",
		ConfigKey:   "minio",
		ConfigValue: string(minioValue),
		Enabled:     true,
		Description: "MinIO 对象存储服务",
	})

	// --- 搜索引擎默认配置（留空，DuckDuckGo 作为兜底不需要配置） ---
	// 如需添加自定义搜索 API，在此追加：
	// searchValue, _ := json.Marshal(map[string]interface{}{
	//     "name":    "serper",
	//     "api_url": "https://google.serper.dev/search",
	//     "api_key": "your-api-key",
	// })
	// configs = append(configs, entity.SysConfig{...})

	return configs
}

func upsertSysConfig(db *gorm.DB, sc entity.SysConfig) error {
	var existing entity.SysConfig
	err := db.Where("config_group = ? AND config_key = ?", sc.ConfigGroup, sc.ConfigKey).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		return db.Create(&sc).Error
	}
	if err != nil {
		return err
	}

	// 更新已有记录
	existing.ConfigValue = sc.ConfigValue
	existing.Enabled = sc.Enabled
	existing.Description = sc.Description
	return db.Save(&existing).Error
}
