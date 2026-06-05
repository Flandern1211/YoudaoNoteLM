// internal/service/search_agent_service.go
package service

import (
	"context"
	"encoding/json"
	"strings"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type searchAgentService struct {
	configService ConfigService
	importer      ImporterService
	searchAgent   SearchAgentInterface
}

// NewSearchAgentService 创建搜索 Agent 服务
func NewSearchAgentService(
	configService ConfigService,
	importer ImporterService,
	searchAgent SearchAgentInterface,
) SearchAgentService {
	return &searchAgentService{
		configService: configService,
		importer:      importer,
		searchAgent:   searchAgent,
	}
}

// Search 智能搜索
func (s *searchAgentService) Search(userID, notebookID uint, query string) (*response.SearchResponse, error) {
	// 执行 Agent
	ctx := context.Background()
	result, err := s.searchAgent.Execute(ctx, userID, notebookID, query)
	if err != nil {
		return nil, err
	}

	// 解析 Agent 结果为 SearchResponse
	return parseAgentResult(result.Content, result.SearchRounds)
}

// ImportFromURL URL 直接导入
func (s *searchAgentService) ImportFromURL(userID, notebookID uint, url string) (*entity.Source, error) {
	taskID, err := s.importer.ImportSearchResults(userID, notebookID, []string{url})
	if err != nil {
		return nil, err
	}

	logger.Info("URL导入任务已创建",
		zap.Uint("user_id", userID),
		zap.String("url", url),
		zap.String("task_id", taskID),
	)

	// 异步任务，前端通过 taskID 轮询获取结果
	return nil, nil
}

// ImportSearchResults 批量导入
func (s *searchAgentService) ImportSearchResults(userID, notebookID uint, urls []string) (string, error) {
	return s.importer.ImportSearchResults(userID, notebookID, urls)
}

// parseAgentResult 解析 Agent 返回的内容为 SearchResponse
func parseAgentResult(content string, searchRounds int) (*response.SearchResponse, error) {
	// 尝试从 Agent 回复中提取 JSON 结果
	var result response.SearchResponse

	// 尝试直接解析整个内容为 JSON
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		return &result, nil
	}

	// 如果 Agent 没有返回结构化 JSON，则将整个内容作为 summary
	result = response.SearchResponse{
		Results:      []response.SearchResultItem{},
		Summary:      content,
		SearchRounds: searchRounds,
	}

	// 尝试从文本中提取 URL 作为结果
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			result.Results = append(result.Results, response.SearchResultItem{
				URL: line,
			})
		}
	}

	return &result, nil
}
