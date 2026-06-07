// internal/agent/search/agent.go
package search

import (
	"context"
	"encoding/json"

	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const maxAgentRounds = 5

// SearchAgent 搜索 Agent（基于 Eino 框架）
type SearchAgent struct {
	configService service.ConfigService
	importer      service.ImporterService
}

// NewSearchAgent 创建搜索 Agent
func NewSearchAgent(
	configService service.ConfigService,
	importer service.ImporterService,
) *SearchAgent {
	return &SearchAgent{
		configService: configService,
		importer:      importer,
	}
}

// einoChatModelAdapter 将 external.LLMClient 适配为 Eino 的 model.ToolCallingChatModel
type einoChatModelAdapter struct {
	llmClient external.LLMClient
	tools     []*schema.ToolInfo
}

// Generate 实现 Eino model.BaseModel 接口
func (a *einoChatModelAdapter) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// 将 Eino Message 转换为 external.Message
	messages := make([]external.Message, len(input))
	for i, msg := range input {
		messages[i] = external.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		// 处理 ToolCalls
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]external.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				// 将 Arguments 从 string 转换为 map[string]any
				var argsMap map[string]any
				if tc.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsMap); err != nil {
						argsMap = make(map[string]any)
					}
				}
				messages[i].ToolCalls[j] = external.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: argsMap,
				}
			}
		}
		// 处理 ToolCallID
		if msg.ToolCallID != "" {
			messages[i].ToolCallID = msg.ToolCallID
		}
	}

	// 转换工具定义
	var toolDefs []external.ToolDef
	if len(a.tools) > 0 {
		toolDefs = make([]external.ToolDef, len(a.tools))
		for i, t := range a.tools {
			toolDefs[i] = external.ToolDef{
				Name:        t.Name,
				Description: t.Desc,
			}
			// 转换参数
			if t.ParamsOneOf != nil {
				jsonSchema, err := t.ParamsOneOf.ToJSONSchema()
				if err == nil && jsonSchema != nil {
					// 将 jsonschema 转换为 map[string]any
					schemaJSON, err := json.Marshal(jsonSchema)
					if err == nil {
						var paramsMap map[string]any
						json.Unmarshal(schemaJSON, &paramsMap)
						toolDefs[i].Parameters = paramsMap
					}
				}
			}
		}
	}

	// 调用 LLM
	resp, err := a.llmClient.ChatWithTools(messages, toolDefs)
	if err != nil {
		return nil, err
	}

	// 转换响应为 Eino Message
	result := &schema.Message{
		Role:    schema.Assistant,
		Content: resp.Content,
	}

	// 处理 ToolCalls
	if len(resp.ToolCalls) > 0 {
		result.ToolCalls = make([]schema.ToolCall, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			// 将 Arguments 从 map[string]any 转换为 string
			argsJSON := ""
			if tc.Arguments != nil {
				if b, err := json.Marshal(tc.Arguments); err == nil {
					argsJSON = string(b)
				}
			}
			result.ToolCalls[i] = schema.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      tc.Name,
					Arguments: argsJSON,
				},
			}
		}
	}

	return result, nil
}

// Stream 实现 Eino model.BaseModel 接口（流式）
func (a *einoChatModelAdapter) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// 对于简单实现，先调用 Generate 然后包装为流
	msg, err := a.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

// WithTools 实现 Eino model.ToolCallingChatModel 接口
func (a *einoChatModelAdapter) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	// 创建新的适配器实例，避免并发问题
	return &einoChatModelAdapter{
		llmClient: a.llmClient,
		tools:     tools,
	}, nil
}

// Execute 执行搜索任务（协调者调用入口）
// 返回 service.SearchAgentResult 以实现 service.SearchAgentInterface
func (a *SearchAgent) Execute(ctx context.Context, userID, notebookID uint, task string) (*service.SearchAgentResult, error) {
	// 注入 userID 和 notebookID 到 context
	ctx = WithUserID(ctx, userID)
	ctx = WithNotebookID(ctx, notebookID)

	// 通过 ConfigService 获取用户的 LLM 客户端
	llmClient, err := a.configService.GetLLMClient(userID)
	if err != nil {
		return nil, err
	}

	// 创建 Eino 工具
	tools := make([]tool.BaseTool, 0, 4)

	webSearchTool, err := NewWebSearchTool(a.configService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 web_search 工具失败", err)
	}
	tools = append(tools, webSearchTool)

	analyzeTool, err := NewAnalyzeResultsTool(llmClient)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 analyze_results 工具失败", err)
	}
	tools = append(tools, analyzeTool)

	refineTool, err := NewRefineQueryTool(llmClient)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 refine_query 工具失败", err)
	}
	tools = append(tools, refineTool)

	importTool, err := NewImportURLsTool(a.importer)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 import_urls 工具失败", err)
	}
	tools = append(tools, importTool)

	// 创建 Eino ChatModel 适配器
	chatModel := &einoChatModelAdapter{llmClient: llmClient}

	// 创建 Eino Agent
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "SearchAgent",
		Description: "网络搜索助手，帮助用户搜索、分析和导入网络内容",
		Instruction: SearchSystemPrompt,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
	})
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 Eino Agent 失败", err)
	}

	// 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 执行查询
	searchRounds := 0
	var finalContent string

	iter := runner.Query(ctx, task)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			logger.Error("Agent 执行错误", zap.Error(event.Err))
			return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "Agent 执行失败", event.Err)
		}

		// 处理事件
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				logger.Warn("获取消息失败", zap.Error(err))
				continue
			}

			// 统计搜索轮数
			if event.Output.MessageOutput.ToolName == "web_search" {
				searchRounds++
			}

			// 获取最终内容
			if msg.Role == schema.Assistant && len(msg.ToolCalls) == 0 {
				finalContent = msg.Content
			}
		}
	}

	logger.Info("Agent 执行完成",
		zap.Int("searchRounds", searchRounds),
		zap.Int("contentLength", len(finalContent)),
	)

	return &service.SearchAgentResult{
		Content:      finalContent,
		SearchRounds: searchRounds,
	}, nil
}

// ExecuteStream 执行搜索任务（流式返回）
func (a *SearchAgent) ExecuteStream(ctx context.Context, userID, notebookID uint, task string) ([]*service.SearchAgentEvent, error) {
	// 注入 userID 和 notebookID 到 context
	ctx = WithUserID(ctx, userID)
	ctx = WithNotebookID(ctx, notebookID)

	// 通过 ConfigService 获取用户的 LLM 客户端
	llmClient, err := a.configService.GetLLMClient(userID)
	if err != nil {
		return nil, err
	}

	// 创建 Eino 工具
	tools := make([]tool.BaseTool, 0, 4)

	webSearchTool, err := NewWebSearchTool(a.configService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 web_search 工具失败", err)
	}
	tools = append(tools, webSearchTool)

	analyzeTool, err := NewAnalyzeResultsTool(llmClient)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 analyze_results 工具失败", err)
	}
	tools = append(tools, analyzeTool)

	refineTool, err := NewRefineQueryTool(llmClient)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 refine_query 工具失败", err)
	}
	tools = append(tools, refineTool)

	importTool, err := NewImportURLsTool(a.importer)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 import_urls 工具失败", err)
	}
	tools = append(tools, importTool)

	// 创建 Eino ChatModel 适配器
	chatModel := &einoChatModelAdapter{llmClient: llmClient}

	// 创建 Eino Agent
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "SearchAgent",
		Description: "网络搜索助手，帮助用户搜索、分析和导入网络内容",
		Instruction: SearchSystemPrompt,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
	})
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 Eino Agent 失败", err)
	}

	// 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	// 执行查询并收集事件
	iter := runner.Query(ctx, task)

	var events []*service.SearchAgentEvent
	searchRounds := 0

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			logger.Error("Agent 流式执行错误", zap.Error(event.Err))
			events = append(events, &service.SearchAgentEvent{
				Type:  "error",
				Error: event.Err.Error(),
			})
			return events, nil
		}

		// 处理事件
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				logger.Warn("获取消息失败", zap.Error(err))
				continue
			}

			// 统计搜索轮数
			if event.Output.MessageOutput.ToolName == "web_search" {
				searchRounds++
				events = append(events, &service.SearchAgentEvent{
					Type:         "search_round",
					SearchRounds: searchRounds,
				})
			}

			// 发送内容事件
			if msg.Content != "" {
				events = append(events, &service.SearchAgentEvent{
					Type:    "content",
					Content: msg.Content,
					Role:    string(msg.Role),
				})
			}

			// 工具调用事件
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					events = append(events, &service.SearchAgentEvent{
						Type:     "tool_call",
						ToolName: tc.Function.Name,
						ToolArgs: tc.Function.Arguments,
					})
				}
			}
		}
	}

	// 发送完成事件
	events = append(events, &service.SearchAgentEvent{
		Type:         "done",
		SearchRounds: searchRounds,
	})

	logger.Info("Agent 流式执行完成",
		zap.Int("searchRounds", searchRounds),
		zap.Int("events", len(events)),
	)

	return events, nil
}
