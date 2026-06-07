package main

import (
	"YoudaoNoteLm/internal/service/external"
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
	_, _ = database.InitRedis(&cfg.Database.Redis)

	// 插入 sys_config
	type SysConfig struct {
		gorm.Model
		ConfigGroup string
		ConfigKey   string
		ConfigValue string `gorm:"type:json"`
		Enabled     bool   `gorm:"default:true"`
		Description string
	}
	db.Where("config_key = ?", "searxng").Delete(&SysConfig{})
	db.Create(&SysConfig{
		ConfigGroup: "search",
		ConfigKey:   "searxng",
		ConfigValue: "http://60.205.184.232:9004",
		Enabled:     true,
		Description: "SearXNG 搜索引擎",
	})
	_ = redis.NewClient(&redis.Options{})

	// 直接测试
	engine := external.NewSearXNGEngine("http://60.205.184.232:9004")
	r, err := engine.Search("Go 语言", 3)
	if err != nil {
		panic(err)
	}
	for i, x := range r {
		println(i+1, x.Title, x.URL)
	}
	println("sys_config OK")
}
