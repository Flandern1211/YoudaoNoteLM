// internal/agent/search/tools.go
package search

import (
	"context"
	"encoding/json"
	"fmt"

	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// ========== web_search 工具 ==========

type webSearchTool struct {
	configService service.ConfigService
}

func NewWebSearchTool(configService service.ConfigService) Tool {
	return &webSearchTool{configService: configService}
}

func (t *webSearchTool) Name() string { return "web_search" }
func (t *webSearchTool) Description() string {
	return "搜索网络内容。输入搜索关键词，返回搜索结果列表（标题、URL、摘要）"
}
func (t *webSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索关键词",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "返回结果数量，默认10",
			},
		},
		"required": []string{"query"},
	}
}

func (t *webSearchTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	query, _ := params["query"].(string)
	limit := 10
	if v, ok := params["limit"]; ok {
		if n, ok := v.(float64); ok {
			limit = int(n)
		}
	}

	userID := GetUserID(ctx)
	engine, err := t.configService.GetSearchEngine(userID)
	if err != nil {
		return nil, err
	}

	results, err := engine.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	logger.Info("web_search 执行成功",
		zap.String("query", query),
		zap.Int("results", len(results)),
	)

	return map[string]any{"results": results}, nil
}

// ========== analyze_results 工具 ==========

type analyzeResultsTool struct {
	llmClient external.LLMClient
}

func NewAnalyzeResultsTool(llmClient external.LLMClient) Tool {
	return &analyzeResultsTool{llmClient: llmClient}
}

func (t *analyzeResultsTool) Name() string { return "analyze_results" }
func (t *analyzeResultsTool) Description() string {
	return "分析搜索结果的相关性和质量。输入搜索结果列表和用户意图，返回带评分和推荐理由的排序结果"
}
func (t *analyzeResultsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"results": map[string]any{
				"type":        "array",
				"description": "搜索结果列表",
			},
			"user_intent": map[string]any{
				"type":        "string",
				"description": "用户的搜索意图描述",
			},
		},
		"required": []string{"results", "user_intent"},
	}
}

func (t *analyzeResultsTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	resultsJSON, _ := json.Marshal(params["results"])
	userIntent, _ := params["user_intent"].(string)

	prompt := fmt.Sprintf(`分析以下搜索结果与用户意图的相关性和质量。

用户意图：%s

搜索结果：
%s

请对每个结果评分(1-10)并说明推荐理由。
返回JSON格式：
{
  "ranked": [
    {"title": "...", "url": "...", "snippet": "...", "score": 8.5, "reason": "..."}
  ]
}
只返回JSON，不要其他内容。`, userIntent, string(resultsJSON))

	resp, err := t.llmClient.Chat([]external.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM分析失败: %w", err)
	}

	// 解析 LLM 返回的 JSON
	var result struct {
		Ranked []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Snippet string  `json:"snippet"`
			Score   float64 `json:"score"`
			Reason  string  `json:"reason"`
		} `json:"ranked"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		logger.Warn("解析analyze_results LLM响应失败", zap.String("raw", resp), zap.Error(err))
		return map[string]any{"raw": resp}, nil
	}

	logger.Info("analyze_results 执行成功", zap.Int("ranked", len(result.Ranked)))
	return result, nil
}

// ========== refine_query 工具 ==========

type refineQueryTool struct {
	llmClient external.LLMClient
}

func NewRefineQueryTool(llmClient external.LLMClient) Tool {
	return &refineQueryTool{llmClient: llmClient}
}

func (t *refineQueryTool) Name() string { return "refine_query" }
func (t *refineQueryTool) Description() string {
	return "优化搜索关键词。输入原始搜索词和上下文信息，返回优化后的关键词列表，每个关键词从不同角度切入"
}
func (t *refineQueryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"original_query": map[string]any{
				"type":        "string",
				"description": "原始搜索关键词",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "上下文信息，如之前搜索结果的质量反馈",
			},
		},
		"required": []string{"original_query"},
	}
}

func (t *refineQueryTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	original, _ := params["original_query"].(string)
	contextInfo, _ := params["context"].(string)
	if contextInfo == "" {
		contextInfo = "无额外上下文"
	}

	prompt := fmt.Sprintf(`用户想搜索"%s"，背景：%s。
请提供3个优化后的搜索关键词，每个更精准或从不同角度切入。
返回JSON格式：
{"refined_queries": ["关键词1", "关键词2", "关键词3"]}
只返回JSON，不要其他内容。`, original, contextInfo)

	resp, err := t.llmClient.Chat([]external.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM优化关键词失败: %w", err)
	}

	var result struct {
		RefinedQueries []string `json:"refined_queries"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		logger.Warn("解析refine_query LLM响应失败", zap.String("raw", resp), zap.Error(err))
		return map[string]any{"raw": resp}, nil
	}

	logger.Info("refine_query 执行成功", zap.Strings("queries", result.RefinedQueries))
	return result, nil
}

// ========== import_urls 工具 ==========

type importURLsTool struct {
	importer service.ImporterService
}

func NewImportURLsTool(importer service.ImporterService) Tool {
	return &importURLsTool{importer: importer}
}

func (t *importURLsTool) Name() string { return "import_urls" }
func (t *importURLsTool) Description() string {
	return "批量导入URL的网页内容到资料库。输入URL列表，返回导入任务ID"
}
func (t *importURLsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"urls": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "要导入的URL列表",
			},
		},
		"required": []string{"urls"},
	}
}

func (t *importURLsTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	rawURLs, ok := params["urls"].([]any)
	if !ok {
		return nil, fmt.Errorf("urls 参数格式错误")
	}

	urls := make([]string, 0, len(rawURLs))
	for _, u := range rawURLs {
		if s, ok := u.(string); ok {
			urls = append(urls, s)
		}
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("URL列表为空")
	}

	userID := GetUserID(ctx)
	notebookID := GetNotebookID(ctx)

	taskID, err := t.importer.ImportSearchResults(userID, notebookID, urls)
	if err != nil {
		return nil, fmt.Errorf("导入失败: %w", err)
	}

	logger.Info("import_urls 执行成功",
		zap.Uint("user_id", userID),
		zap.Int("url_count", len(urls)),
		zap.String("task_id", taskID),
	)

	return map[string]any{"task_id": taskID, "url_count": len(urls)}, nil
}
