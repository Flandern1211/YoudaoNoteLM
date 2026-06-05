// internal/agent/search/agent.go
package search

import (
	"context"
	"encoding/json"
	"fmt"

	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

const maxAgentRounds = 5

// SearchAgent 搜索 Agent
type SearchAgent struct {
	configService service.ConfigService
	importer      service.ImporterService
	llmClient     external.LLMClient
}

// NewSearchAgent 创建搜索 Agent
func NewSearchAgent(
	configService service.ConfigService,
	importer service.ImporterService,
	llmClient external.LLMClient,
) *SearchAgent {
	return &SearchAgent{
		configService: configService,
		importer:      importer,
		llmClient:     llmClient,
	}
}

// Execute 执行搜索任务（协调者调用入口）
// 返回 service.SearchAgentResult 以实现 service.SearchAgentInterface
func (a *SearchAgent) Execute(ctx context.Context, userID, notebookID uint, task string) (*service.SearchAgentResult, error) {
	// 注入 userID 和 notebookID 到 context
	ctx = WithUserID(ctx, userID)
	ctx = WithNotebookID(ctx, notebookID)
	tools := []Tool{
		NewWebSearchTool(a.configService),
		NewAnalyzeResultsTool(a.llmClient),
		NewRefineQueryTool(a.llmClient),
		NewImportURLsTool(a.importer),
	}

	// 构建工具定义
	toolDefs := make([]external.ToolDef, len(tools))
	for i, t := range tools {
		toolDefs[i] = external.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		}
	}

	// 初始消息
	messages := []external.Message{
		{Role: "system", Content: SearchSystemPrompt},
		{Role: "user", Content: task},
	}

	searchRounds := 0

	// Agent 循环：LLM 决定调用哪些工具
	for round := 0; round < maxAgentRounds; round++ {
		logger.Info("Agent 循环", zap.Int("round", round+1))

		resp, err := a.llmClient.ChatWithTools(messages, toolDefs)
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "LLM调用失败", err)
		}

		// 如果没有工具调用，Agent 结束
		if len(resp.ToolCalls) == 0 {
			return &service.SearchAgentResult{
				Content:      resp.Content,
				SearchRounds: searchRounds,
			}, nil
		}

		// 记录 assistant 消息（含 tool_calls）
		messages = append(messages, external.Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		// 执行每个工具调用
		for _, tc := range resp.ToolCalls {
			tool := findTool(tools, tc.Name)
			if tool == nil {
				messages = append(messages, external.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("错误：未知工具 %s", tc.Name),
				})
				continue
			}

			if tc.Name == "web_search" {
				searchRounds++
			}

			result, err := tool.Execute(ctx, tc.Arguments)
			if err != nil {
				messages = append(messages, external.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("工具执行失败: %v", err),
				})
				continue
			}

			resultJSON, _ := json.Marshal(result)
			messages = append(messages, external.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    string(resultJSON),
			})
		}
	}

	// 超过最大轮数，强制返回
	logger.Warn("Agent 达到最大轮数，强制返回", zap.Int("maxRounds", maxAgentRounds))

	finalResp, err := a.llmClient.Chat(messages)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "最终回复生成失败", err)
	}

	return &service.SearchAgentResult{
		Content:      finalResp,
		SearchRounds: searchRounds,
	}, nil
}

// findTool 根据名称查找工具
func findTool(tools []Tool, name string) Tool {
	for _, t := range tools {
		if t.Name() == name {
			return t
		}
	}
	return nil
}
