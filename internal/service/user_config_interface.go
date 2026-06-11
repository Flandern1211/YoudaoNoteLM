package service

import "YoudaoNoteLm/internal/model/entity"

// UserConfigService 用户配置服务接口
type UserConfigService interface {
	// LLM 配置（基础模型，必需）
	ListLLMConfigs(userID uint) ([]*entity.UserConfig, error)
	CreateLLMConfig(userID uint, config *entity.UserConfig) error
	UpdateLLMConfig(id uint, config *entity.UserConfig) error
	DeleteLLMConfig(id uint) error

	// 搜索配置
	ListSearchConfigs(userID uint) ([]*entity.UserConfig, error)
	CreateSearchConfig(userID uint, config *entity.UserConfig) error
	UpdateSearchConfig(id uint, config *entity.UserConfig) error
	DeleteSearchConfig(id uint) error

	// ASR 配置
	ListASRConfigs(userID uint) ([]*entity.UserConfig, error)
	CreateASRConfig(userID uint, config *entity.UserConfig) error
	UpdateASRConfig(id uint, config *entity.UserConfig) error
	DeleteASRConfig(id uint) error

	// Embedding 配置
	ListEmbeddingConfigs(userID uint) ([]*entity.UserConfig, error)
	CreateEmbeddingConfig(userID uint, config *entity.UserConfig) error
	UpdateEmbeddingConfig(id uint, config *entity.UserConfig) error
	DeleteEmbeddingConfig(id uint) error

	// 获取当前生效的配置（用户配置 > 系统配置 > 默认值）
	GetActiveConfig(userID uint, configType string) (*entity.UserConfig, error)

	// TestConfig 测试配置连通性（保存前验证）
	TestConfig(configType string, config *entity.UserConfig) *HealthCheckResult
}
