package external

import (
	"fmt"

	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// NewASRService 根据配置创建 ASR 服务
// 配置示例：
//
//	asr:
//	  provider: aliyun_nls
//	  params:
//	    access_key_id: "xxx"
//	    access_key_secret: "xxx"
//	    app_key: "xxx"
func NewASRService(cfg config.ASRConfig) ASRService {
	switch cfg.Provider {
	case "aliyun_nls":
		return NewAliyunNLSASRService(
			cfg.GetString("access_key_id"),
			cfg.GetString("access_key_secret"),
			cfg.GetString("app_key"),
		)
	default:
		logger.Error("不支持的 ASR provider", zap.String("provider", cfg.Provider))
		// 返回一个会报错的实例，而不是 panic
		return &unsupportedASRService{provider: cfg.Provider}
	}
}

// unsupportedASRService 不支持的 ASR 服务实现
type unsupportedASRService struct {
	provider string
}

func (s *unsupportedASRService) SetStorage(storage FileStorage) {}

func (s *unsupportedASRService) Transcribe(filePath string) (string, error) {
	return "", fmt.Errorf("不支持的 ASR provider: %s", s.provider)
}
