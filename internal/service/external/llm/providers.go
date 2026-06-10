// internal/service/external/llm/providers.go
package llm

import (
	"fmt"

	"YoudaoNoteLm/internal/service/external"
)

const ServiceType = "llm"

// openaiCompatibleFactory 创建 OpenAI 兼容的 LLM 客户端
func openaiCompatibleFactory(displayName string) external.FactoryFunc {
	return func(cfg *external.ServiceConfig) (interface{}, error) {
		model := cfg.Model
		if model == "" {
			model = cfg.GetExtraString("model")
		}
		if model == "" {
			return nil, fmt.Errorf("LLM 模型名称未配置")
		}
		return NewOpenAIClient(cfg.Provider, cfg.APIURL, cfg.APIKey, model), nil
	}
}

func init() {
	r := external.GetGlobalRegistry()

	// ========== 基础模型（用户配置可选） ==========

	// OpenAI
	r.Register(ServiceType, "openai", "OpenAI",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("OpenAI"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 gpt-4o）",
			"api_url": "API 地址（可选，用于代理）",
		})

	// Anthropic（Claude）
	r.Register(ServiceType, "anthropic", "Anthropic（Claude）",
		[]string{"api_key", "model"}, []string{"api_url"},
		func(cfg *external.ServiceConfig) (interface{}, error) {
			model := cfg.Model
			if model == "" {
				model = cfg.GetExtraString("model")
			}
			if model == "" {
				return nil, fmt.Errorf("Claude 模型名称未配置")
			}
			return NewAnthropicClient(cfg.APIURL, cfg.APIKey, model), nil
		}, map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 claude-sonnet-4-20250514）",
			"api_url": "API 地址（可选，用于代理）",
		})

	// ========== 兼容 OpenAI 的模型（系统配置可选） ==========

	// DeepSeek
	r.Register(ServiceType, "deepseek", "DeepSeek",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("DeepSeek"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 deepseek-chat）",
			"api_url": "API 地址（默认 https://api.deepseek.com）",
		})

	// 智谱 AI（GLM）
	r.Register(ServiceType, "zhipu", "智谱 AI",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("智谱 AI"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 glm-4）",
			"api_url": "API 地址（默认 https://open.bigmodel.cn/api/paas/v4）",
		})

	// 通义千问
	r.Register(ServiceType, "qwen", "通义千问",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("通义千问"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 qwen-turbo）",
			"api_url": "API 地址（默认 https://dashscope.aliyuncs.com/compatible-mode/v1）",
		})

	// 百川智能
	r.Register(ServiceType, "baichuan", "百川智能",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("百川智能"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 Baichuan4）",
			"api_url": "API 地址（默认 https://api.baichuan-ai.com/v1）",
		})

	// 月之暗面（Kimi）
	r.Register(ServiceType, "moonshot", "月之暗面（Kimi）",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("月之暗面"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 moonshot-v1-8k）",
			"api_url": "API 地址（默认 https://api.moonshot.cn/v1）",
		})

	// MiniMax
	r.Register(ServiceType, "minimax", "MiniMax",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("MiniMax"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 abab6.5-chat）",
			"api_url": "API 地址（默认 https://api.minimax.chat/v1）",
		})

	// 火山引擎（豆包）
	r.Register(ServiceType, "volcengine", "火山引擎（豆包）",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("火山引擎"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 doubao-pro-4k）",
			"api_url": "API 地址（默认 https://ark.cn-beijing.volces.com/api/v3）",
		})
}
