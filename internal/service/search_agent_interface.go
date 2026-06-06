// internal/service/search_agent_interface.go
package service

import (
	"context"

	"YoudaoNoteLm/internal/model/dto/response"
)

// SearchAgentInterface 搜索 Agent 执行接口（由 agent/search.SearchAgent 实现）
type SearchAgentInterface interface {
	Execute(ctx context.Context, userID, notebookID uint, task string) (*SearchAgentResult, error)
}

// SearchAgentResult Agent 执行结果（与 agent/search.AgentResult 对应）
type SearchAgentResult struct {
	Content      string `json:"content"`
	SearchRounds int    `json:"search_rounds"`
}

// SearchAgentEvent Agent 流式执行事件
type SearchAgentEvent struct {
	Type         string `json:"type"` // content, tool_call, search_round, error, done
	Content      string `json:"content,omitempty"`
	Role         string `json:"role,omitempty"`
	ToolName     string `json:"tool_name,omitempty"`
	ToolArgs     string `json:"tool_args,omitempty"`
	SearchRounds int    `json:"search_rounds,omitempty"`
	Error        string `json:"error,omitempty"`
}

// SearchAgentService 搜索 Agent 服务接口
type SearchAgentService interface {
	// Search 智能搜索：Agent 自主执行多轮搜索+分析
	Search(userID, notebookID uint, query string) (*response.SearchResponse, error)
	// ImportFromURL URL 直接导入（返回任务 ID）
	ImportFromURL(userID, notebookID uint, url string) (string, error)
	// ImportSearchResults 批量导入 URL 列表
	ImportSearchResults(userID, notebookID uint, urls []string) (string, error)
}
