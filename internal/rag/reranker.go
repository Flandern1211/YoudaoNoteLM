package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"

	"YoudaoNoteLm/pkg/logger"
)

// RerankScore Rerank 评分结果
type RerankScore struct {
	Index int     // 原始结果中的索引
	Score float32 // 相关度分数
}

// RerankerConfig Reranker 配置
type RerankerConfig struct {
	APIKey  string
	Model   string // 默认 "m3-v2-rerank"
	BaseURL string // 默认 "https://ark.cn-beijing.volces.com/api/v3"
}

// DoubaoReranker 豆包 Rerank API 封装
type DoubaoReranker struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewDoubaoReranker 创建豆包 Reranker
func NewDoubaoReranker(cfg RerankerConfig) *DoubaoReranker {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}

	model := cfg.Model
	if model == "" {
		model = "m3-v2-rerank"
	}

	return &DoubaoReranker{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// rerankRequest Rerank API 请求体
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// rerankResponse Rerank API 响应体
type rerankResponse struct {
	ID      string `json:"id"`
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float32 `json:"relevance_score"`
	} `json:"results"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Rerank 对候选文档重排序
func (r *DoubaoReranker) Rerank(ctx context.Context, query string, documents []string) ([]RerankScore, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	// 构建请求
	reqBody := rerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
		TopN:      len(documents),
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化 rerank 请求失败: %w", err)
	}

	// 发送请求
	url := r.baseURL + "/rerank"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建 rerank 请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 rerank API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 rerank 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Rerank API 返回错误",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("rerank API 返回状态码 %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var rerankResp rerankResponse
	if err := json.Unmarshal(body, &rerankResp); err != nil {
		return nil, fmt.Errorf("解析 rerank 响应失败: %w", err)
	}

	// 转换结果
	scores := make([]RerankScore, len(rerankResp.Results))
	for i, result := range rerankResp.Results {
		scores[i] = RerankScore{
			Index: result.Index,
			Score: result.RelevanceScore,
		}
	}

	logger.Debug("Rerank 完成",
		zap.Int("document_count", len(documents)),
		zap.Int("result_count", len(scores)),
		zap.Int("tokens_used", rerankResp.Usage.TotalTokens),
	)

	return scores, nil
}
