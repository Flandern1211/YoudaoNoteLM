package service

import (
	"context"
	"fmt"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service/external"
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
	GetSearchEngine(userID uint) (external.SearchEngine, error)
	GetASRService(userID uint) (external.ASRService, error)
	GetEmbeddingService(userID uint) (external.EmbeddingService, error)

	// 配置管理（带缓存失效）
	UpdateUserConfig(config *entity.UserConfig) error
	DeleteUserConfig(userID uint, configType string) error
	ClearUserConfigCache(userID uint, configType string)
}

type configService struct {
	sysConfigRepo  repository.SysConfigRepository
	userConfigRepo repository.UserConfigRepository
	cache          CacheStore
	storage        external.FileStorage // ASR 需要注入存储服务
}

func NewConfigService(
	sysConfigRepo repository.SysConfigRepository,
	userConfigRepo repository.UserConfigRepository,
	cache CacheStore,
	storage external.FileStorage,
) ConfigService {
	return &configService{
		sysConfigRepo:  sysConfigRepo,
		userConfigRepo: userConfigRepo,
		cache:          cache,
		storage:        storage,
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
				break
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
					break
				}
			}
		}
	}

	// 3. DuckDuckGo 兜底
	logger.Info("使用 DuckDuckGo 兜底搜索引擎")
	return external.NewDuckDuckGoEngine(), nil
}

func (s *configService) GetASRService(userID uint) (external.ASRService, error) {
	ctx := context.Background()

	// 1. 查用户ASR配置（先查缓存）
	cacheKey := userConfigCacheKey(userID, "asr")
	var userCfg entity.UserConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil && userCfg.Enabled {
		logger.Debug("用户ASR配置缓存命中", zap.Uint("user_id", userID))
		svc := external.NewASRServiceFromDB(userCfg.Provider, userCfg.APIURL, userCfg.APIKey, userCfg.ExtraConfig)
		if svc == nil {
			return nil, bizerrors.New(bizerrors.CodeASTranscriptionFailed, "不支持的ASR服务商: "+userCfg.Provider)
		}
		s.injectStorage(svc)
		return svc, nil
	}

	// 缓存未命中，查 DB
	userCfgPtr, err := s.userConfigRepo.FindByUserAndType(userID, "asr")
	if err == nil && userCfgPtr != nil && userCfgPtr.Enabled {
		_ = s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL)
		svc := external.NewASRServiceFromDB(userCfgPtr.Provider, userCfgPtr.APIURL, userCfgPtr.APIKey, userCfgPtr.ExtraConfig)
		if svc == nil {
			return nil, bizerrors.New(bizerrors.CodeASTranscriptionFailed, "不支持的ASR服务商: "+userCfgPtr.Provider)
		}
		s.injectStorage(svc)
		return svc, nil
	}

	// 2. 降级到系统内置配置
	sysCacheKey := sysConfigCacheKey("asr")
	var builtins []*entity.SysConfig
	if err := s.cache.Get(ctx, sysCacheKey, &builtins); err == nil {
		for _, builtin := range builtins {
			if builtin.Enabled {
				logger.Info("使用系统内置ASR配置（缓存）", zap.String("key", builtin.ConfigKey))
				break
			}
		}
	} else {
		builtins, err = s.sysConfigRepo.FindByGroup("asr")
		if err == nil {
			_ = s.cache.Set(ctx, sysCacheKey, builtins, sysConfigTTL)
		}
	}

	// 3. 无可用配置
	return nil, bizerrors.New(bizerrors.CodeASTranscriptionFailed, "未配置ASR服务")
}

func (s *configService) GetEmbeddingService(userID uint) (external.EmbeddingService, error) {
	ctx := context.Background()

	// 1. 查用户Embedding配置（先查缓存）
	cacheKey := userConfigCacheKey(userID, "embedding")
	var userCfg entity.UserConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil && userCfg.Enabled {
		logger.Debug("用户Embedding配置缓存命中", zap.Uint("user_id", userID))
		// TODO: 根据 provider 创建对应的 EmbeddingService
	}

	// 缓存未命中，查 DB
	userCfgPtr, err := s.userConfigRepo.FindByUserAndType(userID, "embedding")
	if err == nil && userCfgPtr != nil && userCfgPtr.Enabled {
		_ = s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL)
		logger.Info("使用用户自定义Embedding配置",
			zap.Uint("user_id", userID),
			zap.String("provider", userCfgPtr.Provider),
		)
	}

	// 2. 降级到系统内置配置
	sysCacheKey := sysConfigCacheKey("embedding")
	var builtins []*entity.SysConfig
	if err := s.cache.Get(ctx, sysCacheKey, &builtins); err == nil {
		for _, builtin := range builtins {
			if builtin.Enabled {
				logger.Info("使用系统内置Embedding配置（缓存）", zap.String("key", builtin.ConfigKey))
				break
			}
		}
	} else {
		builtins, err = s.sysConfigRepo.FindByGroup("embedding")
		if err == nil {
			_ = s.cache.Set(ctx, sysCacheKey, builtins, sysConfigTTL)
		}
	}

	// 3. 无可用配置
	return nil, bizerrors.New(bizerrors.CodeInternalServiceError, "未配置Embedding服务")
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

// injectStorage 注入文件存储到 ASR 服务（如果支持）
func (s *configService) injectStorage(svc external.ASRService) {
	if svc == nil || s.storage == nil {
		return
	}
	if setter, ok := svc.(interface{ SetStorage(external.FileStorage) }); ok {
		setter.SetStorage(s.storage)
	}
}
