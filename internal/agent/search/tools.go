// internal/agent/search/tools.go
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"go.uber.org/zap"
)

// stripCodeBlock 去除 LLM 返回的 markdown 代码块标记
func stripCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	// 处理 ```json ... ``` 或 ``` ... ```
	if strings.HasPrefix(s, "```") {
		// 找到第一行结束
		idx := strings.Index(s, "\n")
		if idx != -1 {
			s = s[idx+1:]
		}
		// 去掉末尾的 ```
		if strings.HasSuffix(s, "```") {
			s = s[:len(s)-3]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

// ========== web_search 工具 ==========

// WebSearchInput web_search 工具输入
type WebSearchInput struct {
	Query string `json:"query" jsonschema_description:"搜索关键词"`
	Limit int    `json:"limit" jsonschema_description:"返回结果数量，默认10"`
}

// WebSearchOutput web_search 工具输出
type WebSearchOutput struct {
	Results any `json:"results"`
}

// NewWebSearchTool 创建 web_search 工具
func NewWebSearchTool(configService service.ConfigService) (tool.InvokableTool, error) {
	return utils.InferTool("web_search", "搜索网络内容。输入搜索关键词，返回搜索结果列表（标题、URL、摘要）",
		func(ctx context.Context, input *WebSearchInput) (*WebSearchOutput, error) {
			limit := input.Limit
			if limit <= 0 {
				limit = 10
			}

			userID := GetUserID(ctx)
			engine, err := configService.GetSearchEngine(userID)
			if err != nil {
				return nil, err
			}

			results, err := engine.Search(input.Query, limit)
			if err != nil {
				return nil, fmt.Errorf("搜索失败: %w", err)
			}

			logger.Info("web_search 执行成功",
				zap.String("query", input.Query),
				zap.Int("results", len(results)),
			)

			return &WebSearchOutput{Results: results}, nil
		},
	)
}

// ========== analyze_results 工具 ==========

// AnalyzeResultsInput analyze_results 工具输入
type AnalyzeResultsInput struct {
	Results    any    `json:"results" jsonschema_description:"搜索结果列表"`
	UserIntent string `json:"user_intent" jsonschema_description:"用户的搜索意图描述"`
}

// AnalyzeRankedItem 分析后的结果项
type AnalyzeRankedItem struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason"`
}

// AnalyzeResultsOutput analyze_results 工具输出
type AnalyzeResultsOutput struct {
	Ranked []AnalyzeRankedItem `json:"ranked"`
}

// NewAnalyzeResultsTool 创建 analyze_results 工具
func NewAnalyzeResultsTool(llmClient external.LLMClient) (tool.InvokableTool, error) {
	return utils.InferTool("analyze_results", "分析搜索结果的相关性和质量。输入搜索结果列表和用户意图，返回带评分和推荐理由的排序结果",
		func(ctx context.Context, input *AnalyzeResultsInput) (*AnalyzeResultsOutput, error) {
			resultsJSON, _ := json.Marshal(input.Results)

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
只返回JSON，不要其他内容。`, input.UserIntent, string(resultsJSON))

			resp, err := llmClient.Chat([]external.Message{
				{Role: "user", Content: prompt},
			})
			if err != nil {
				return nil, fmt.Errorf("LLM分析失败: %w", err)
			}

			// 解析 LLM 返回的 JSON
			var result AnalyzeResultsOutput
			cleaned := stripCodeBlock(resp)
			if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
				logger.Warn("解析analyze_results LLM响应失败", zap.String("raw", resp), zap.Error(err))
				// 返回空结果而不是错误
				return &AnalyzeResultsOutput{}, nil
			}

			logger.Info("analyze_results 执行成功", zap.Int("ranked", len(result.Ranked)))
			return &result, nil
		},
	)
}

// ========== refine_query 工具 ==========

// RefineQueryInput refine_query 工具输入
type RefineQueryInput struct {
	OriginalQuery string `json:"original_query" jsonschema_description:"原始搜索关键词"`
	Context       string `json:"context" jsonschema_description:"上下文信息，如之前搜索结果的质量反馈"`
}

// RefineQueryOutput refine_query 工具输出
type RefineQueryOutput struct {
	RefinedQueries []string `json:"refined_queries"`
}

// NewRefineQueryTool 创建 refine_query 工具
func NewRefineQueryTool(llmClient external.LLMClient) (tool.InvokableTool, error) {
	return utils.InferTool("refine_query", "优化搜索关键词。输入原始搜索词和上下文信息，返回优化后的关键词列表，每个关键词从不同角度切入",
		func(ctx context.Context, input *RefineQueryInput) (*RefineQueryOutput, error) {
			contextInfo := input.Context
			if contextInfo == "" {
				contextInfo = "无额外上下文"
			}

			prompt := fmt.Sprintf(`用户想搜索"%s"，背景：%s。
请提供3个优化后的搜索关键词，每个更精准或从不同角度切入。
返回JSON格式：
{"refined_queries": ["关键词1", "关键词2", "关键词3"]}
只返回JSON，不要其他内容。`, input.OriginalQuery, contextInfo)

			resp, err := llmClient.Chat([]external.Message{
				{Role: "user", Content: prompt},
			})
			if err != nil {
				return nil, fmt.Errorf("LLM优化关键词失败: %w", err)
			}

			var result RefineQueryOutput
			cleaned := stripCodeBlock(resp)
			if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
				logger.Warn("解析refine_query LLM响应失败", zap.String("raw", resp), zap.Error(err))
				// 返回原始查询作为降级
				return &RefineQueryOutput{RefinedQueries: []string{input.OriginalQuery}}, nil
			}

			logger.Info("refine_query 执行成功", zap.Strings("queries", result.RefinedQueries))
			return &result, nil
		},
	)
}

// ========== import_urls 工具 ==========

// ImportURLsInput import_urls 工具输入
type ImportURLsInput struct {
	URLs []string `json:"urls" jsonschema_description:"要导入的URL列表"`
}

// ImportURLsOutput import_urls 工具输出
type ImportURLsOutput struct {
	TaskID   string `json:"task_id"`
	URLCount int    `json:"url_count"`
}

// NewImportURLsTool 创建 import_urls 工具
func NewImportURLsTool(importer service.ImporterService) (tool.InvokableTool, error) {
	return utils.InferTool("import_urls", "批量导入URL的网页内容到资料库。输入URL列表，返回导入任务ID",
		func(ctx context.Context, input *ImportURLsInput) (*ImportURLsOutput, error) {
			if len(input.URLs) == 0 {
				return nil, fmt.Errorf("URL列表为空")
			}

			userID := GetUserID(ctx)
			notebookID := GetNotebookID(ctx)

			taskID, err := importer.ImportSearchResults(userID, notebookID, input.URLs)
			if err != nil {
				return nil, fmt.Errorf("导入失败: %w", err)
			}

			logger.Info("import_urls 执行成功",
				zap.Uint("user_id", userID),
				zap.Int("url_count", len(input.URLs)),
				zap.String("task_id", taskID),
			)

			return &ImportURLsOutput{
				TaskID:   taskID,
				URLCount: len(input.URLs),
			}, nil
		},
	)
}
