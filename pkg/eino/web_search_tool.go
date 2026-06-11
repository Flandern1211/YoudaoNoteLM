package eino

import (
	"context"

	"YoudaoNoteLm/internal/service"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

const webSearchToolName = "web_search"

// NewWebSearchTool 将统一搜索服务封装为 Eino Tool。
func NewWebSearchTool(searchService service.SearchService) (tool.InvokableTool, error) {
	return toolutils.InferTool[service.SearchRequest, *service.SearchResponse](
		webSearchToolName,
		"Execute unified web search through the project's Bocha-backed search service.",
		func(ctx context.Context, input service.SearchRequest) (*service.SearchResponse, error) {
			return searchService.Search(ctx, &input)
		},
	)
}
