package repository

import "YoudaoNoteLm/internal/model/entity"

// UserConfigRepository 用户配置仓储接口（搜索/ASR/Embedding统一）
type UserConfigRepository interface {
	FindByUserAndType(userID int, configType string) (*entity.UserConfig, error)
	FindByID(id int) (*entity.UserConfig, error)
	Create(config *entity.UserConfig) error
	Update(config *entity.UserConfig) error
	Delete(id int) error
}
