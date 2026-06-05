// internal/service/config_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

const (
	userConfigTTL = 60 * time.Second // 用户配置缓存 60s
	sysConfigTTL  = 5 * time.Minute  // 系统配置缓存 5min
)

// ConfigService 配置路由服务接口
type ConfigService interface {
	// GetSearchEngine 获取用户的搜索引擎（用户配置 → 系统内置 → DuckDuckGo 兜底）
	GetSearchEngine(userID uint) (external.SearchEngine, error)
	// GetLLMClient 获取用户的 LLM 客户端（用户必须配置，无降级）
	GetLLMClient(userID uint) (external.LLMClient, error)

	// 配置管理（带缓存失效）
	UpdateUserConfig(config *entity.UserConfig) error
	DeleteUserConfig(userID uint, configType string) error
	ClearUserConfigCache(userID uint, configType string)
}

type configService struct {
	sysConfigRepo  repository.SysConfigRepository
	userConfigRepo repository.UserConfigRepository
	cache          *cache.Cache
	searchCfg      config.SearchConfig
}

// NewConfigService 创建配置服务
func NewConfigService(
	sysConfigRepo repository.SysConfigRepository,
	userConfigRepo repository.UserConfigRepository,
	cache *cache.Cache,
	searchCfg config.SearchConfig,
) ConfigService {
	return &configService{
		sysConfigRepo:  sysConfigRepo,
		userConfigRepo: userConfigRepo,
		cache:          cache,
		searchCfg:      searchCfg,
	}
}

// --- 缓存 Key 生成 ---

func userConfigCacheKey(userID uint, configType string) string {
	return fmt.Sprintf("config:user:%d:%s", userID, configType)
}

func sysConfigCacheKey(group string) string {
	return fmt.Sprintf("config:sys:%s", group)
}

// --- 查询（带缓存） ---

// GetSearchEngine 获取用户的搜索引擎
func (s *configService) GetSearchEngine(userID uint) (external.SearchEngine, error) {
	ctx := context.Background()

	// 1. 查用户搜索配置（先查缓存）
	cacheKey := userConfigCacheKey(userID, "search")
	var userCfg entity.UserConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil && userCfg.Enabled {
		logger.Debug("用户搜索配置缓存命中", zap.Uint("user_id", userID))
		return external.NewCustomEngine(userCfg.Name, userCfg.APIURL, userCfg.APIKey), nil
	}

	// 缓存未命中，查 DB
	userCfgPtr, err := s.userConfigRepo.FindByUserAndType(userID, "search")
	if err == nil && userCfgPtr != nil && userCfgPtr.Enabled {
		// 回填缓存
		_ = s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL)
		return external.NewCustomEngine(userCfgPtr.Name, userCfgPtr.APIURL, userCfgPtr.APIKey), nil
	}

	// 2. 降级到系统内置配置（先查缓存）
	sysCacheKey := sysConfigCacheKey("search")
	var builtins []*entity.SysConfig
	if err := s.cache.Get(ctx, sysCacheKey, &builtins); err == nil {
		for _, builtin := range builtins {
			if builtin.Enabled {
				logger.Info("使用系统内置搜索配置（缓存）", zap.String("key", builtin.ConfigKey))
				return external.NewCustomEngine(builtin.ConfigKey, builtin.ConfigValue, ""), nil
			}
		}
	} else {
		// 缓存未命中，查 DB
		builtins, err = s.sysConfigRepo.FindByGroup("search")
		if err == nil {
			_ = s.cache.Set(ctx, sysCacheKey, builtins, sysConfigTTL)
			for _, builtin := range builtins {
				if builtin.Enabled {
					logger.Info("使用系统内置搜索配置", zap.String("key", builtin.ConfigKey))
					return external.NewCustomEngine(builtin.ConfigKey, builtin.ConfigValue, ""), nil
				}
			}
		}
	}

	// 3. 系统配置的搜索引擎（Bing 等）
	if s.searchCfg.Provider == "bing" && s.searchCfg.APIKey != "" {
		logger.Info("使用系统配置的 Bing 搜索引擎")
		return external.NewBingEngine(s.searchCfg.APIKey), nil
	}

	// 4. DuckDuckGo 兜底（国内不可用，仅作最后手段）
	logger.Info("使用 DuckDuckGo 兜底搜索引擎（注意：国内网络可能不可用）")
	return external.NewDuckDuckGoEngine(), nil
}

// GetLLMClient 获取用户的 LLM 客户端（用户必须配置，无系统降级）
func (s *configService) GetLLMClient(userID uint) (external.LLMClient, error) {
	ctx := context.Background()

	// 先查缓存
	cacheKey := userConfigCacheKey(userID, "llm")
	var userCfg entity.UserConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil && userCfg.Enabled {
		logger.Debug("用户LLM配置缓存命中", zap.Uint("user_id", userID))
		return s.buildLLMClient(&userCfg)
	}

	// 缓存未命中，查 DB
	userCfgPtr, err := s.userConfigRepo.FindByUserAndType(userID, "llm")
	if err != nil || userCfgPtr == nil || !userCfgPtr.Enabled {
		return nil, bizerrors.ErrLLMNotConfigured
	}

	// 回填缓存
	_ = s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL)

	return s.buildLLMClient(userCfgPtr)
}

// buildLLMClient 从 UserConfig 构建 LLMClient
func (s *configService) buildLLMClient(cfg *entity.UserConfig) (external.LLMClient, error) {
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1"
	}

	// 从 extra_config JSON 字符串中解析 model
	model := "gpt-4o" // 默认模型
	if cfg.ExtraConfig != "" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(cfg.ExtraConfig), &extra); err == nil {
			if m, ok := extra["model"].(string); ok && m != "" {
				model = m
			}
		}
	}

	logger.Info("使用用户LLM配置",
		zap.String("name", cfg.Name),
		zap.String("model", model),
	)

	return external.NewLLMClient(cfg.Name, apiURL, cfg.APIKey, model), nil
}

// --- 配置管理（写入时失效） ---

// UpdateUserConfig 更新用户配置并清除缓存
func (s *configService) UpdateUserConfig(config *entity.UserConfig) error {
	if err := s.userConfigRepo.Update(config); err != nil {
		return err
	}
	s.ClearUserConfigCache(config.UserID, config.ConfigType)
	return nil
}

// DeleteUserConfig 删除用户配置并清除缓存
func (s *configService) DeleteUserConfig(userID uint, configType string) error {
	// 先查到配置 ID
	cfg, err := s.userConfigRepo.FindByUserAndType(userID, configType)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil // 不存在，无需删除
	}
	if err := s.userConfigRepo.Delete(cfg.ID); err != nil {
		return err
	}
	s.ClearUserConfigCache(userID, configType)
	return nil
}

// ClearUserConfigCache 清除用户配置缓存
func (s *configService) ClearUserConfigCache(userID uint, configType string) {
	key := userConfigCacheKey(userID, configType)
	if err := s.cache.Delete(context.Background(), key); err != nil {
		logger.Warn("清除用户配置缓存失败",
			zap.Uint("user_id", userID),
			zap.String("config_type", configType),
			zap.Error(err),
		)
	}
}
