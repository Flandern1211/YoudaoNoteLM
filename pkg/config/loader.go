package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var globalConfig *Config

// GetConfig 获取全局配置
func GetConfig() *Config {
	return globalConfig
}

// LoadConfig 从 YAML 文件加载配置
func LoadConfig(path string) (*Config, error) {
	var config Config

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	// 验证配置
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	globalConfig = &config
	return &config, nil
}

// MustLoadConfig 加载配置，失败则 panic
func MustLoadConfig(path string) *Config {
	config, err := LoadConfig(path)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return config
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Database.Host == "" {
		config.Database.Host = "localhost"
	}
	if config.Database.Port == 0 {
		config.Database.Port = 3306
	}
	if config.Database.Charset == "" {
		config.Database.Charset = "utf8mb4"
	}
	if config.Database.Loc == "" {
		config.Database.Loc = "Local"
	}
	if config.Redis.Host == "" {
		config.Redis.Host = "localhost"
	}
	if config.Redis.Port == 0 {
		config.Redis.Port = 6379
	}
	if config.Milvus.Host == "" {
		config.Milvus.Host = "localhost"
	}
	if config.Milvus.Port == 0 {
		config.Milvus.Port = 19530
	}
	if config.JWT.AccessTokenExp == "" {
		config.JWT.AccessTokenExp = "15m" // 15 分钟
	}
	if config.JWT.RefreshTokenExp == "" {
		config.JWT.RefreshTokenExp = "168h" // 7 天
	}
	if config.MCP.MarkItDownURL == "" {
		config.MCP.MarkItDownURL = "http://localhost:3001"
	}
}

// validate 验证配置
func validate(config *Config) error {
	if config.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if config.Database.DBName == "" {
		return fmt.Errorf("database name is required")
	}
	if config.JWT.Secret == "" {
		return fmt.Errorf("jwt secret is required")
	}
	return nil
}
