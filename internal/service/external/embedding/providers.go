// internal/service/external/embedding/providers.go
package embedding

import (
	"fmt"

	"YoudaoNoteLm/internal/service/external"
)

const ServiceType = "embedding"

// openaiCompatibleEmbeddingFactory 创建 OpenAI 兼容的 Embedding 客户端
func openaiCompatibleEmbeddingFactory(displayName string) external.FactoryFunc {
	return func(cfg *external.ServiceConfig) (interface{}, error) {
		model := cfg.Model
		if model == "" {
			model = cfg.GetExtraString("model")
		}
		if model == "" {
			return nil, fmt.Errorf("Embedding 模型名称未配置")
		}
		return NewOpenAIEmbedding(cfg.APIURL, cfg.APIKey, model), nil
	}
}

func init() {
	r := external.GetGlobalRegistry()

	// 火山引擎（豆包）Embedding（唯一支持的 Embedding 服务商）
	r.Register(ServiceType, "volcengine", "火山引擎（豆包）Embedding",
		[]string{"api_key", "model"}, []string{},
		openaiCompatibleEmbeddingFactory("火山引擎"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称或接入点 ID",
		})
}
