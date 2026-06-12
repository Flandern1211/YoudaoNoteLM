package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"YoudaoNoteLm/pkg/config"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

// NewConfiguredGenerationModel creates the main chat model for generation agents.
// It is intentionally separate from the embedding model factory used by RAG.
func NewConfiguredGenerationModel(ctx context.Context, cfg config.LLMConfig) (GenerationModel, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("main llm api_key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("main llm model is required")
	}

	arkCfg := &ark.ChatModelConfig{
		APIKey: strings.TrimSpace(cfg.APIKey),
		Model:  strings.TrimSpace(cfg.Model),
	}
	if baseURL := strings.TrimSpace(cfg.BaseURL); baseURL != "" {
		arkCfg.BaseURL = baseURL
	}
	if cfg.TimeoutSeconds > 0 {
		timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
		arkCfg.Timeout = &timeout
	}
	if cfg.MaxTokens > 0 {
		arkCfg.MaxTokens = &cfg.MaxTokens
	}
	if cfg.Temperature > 0 {
		arkCfg.Temperature = &cfg.Temperature
	}
	if cfg.TopP > 0 {
		arkCfg.TopP = &cfg.TopP
	}

	chatModel, err := ark.NewChatModel(ctx, arkCfg)
	if err != nil {
		return nil, fmt.Errorf("create main llm chat model failed: %w", err)
	}
	return NewEinoGenerationModel(chatModel), nil
}
