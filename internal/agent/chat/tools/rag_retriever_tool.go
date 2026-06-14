package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/rag"
)

// RAGRetrieverTool 知识库检索工具
type RAGRetrieverTool struct {
	retriever  rag.RAGRetriever
	userID     uint
	sourceIDs  []uint
	references *[]response.Reference // 引用收集器
	mu         sync.Mutex            // 保护 references 和 offset
	offset     int                   // 多次检索时的编号偏移
}

// NewRAGRetrieverTool 创建检索工具
func NewRAGRetrieverTool(retriever rag.RAGRetriever, userID uint, sourceIDs []uint, references *[]response.Reference) tool.InvokableTool {
	return &RAGRetrieverTool{
		retriever:  retriever,
		userID:     userID,
		sourceIDs:  sourceIDs,
		references: references,
	}
}

// Info 返回工具元信息
func (t *RAGRetrieverTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "search_knowledge",
		Desc: "从用户的知识库中检索相关资料。当需要查找文档、笔记、资料内容时使用此工具。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "搜索查询词，应该是具体、明确的关键词",
				Required: true,
			},
			"top_k": {
				Type: schema.Integer,
				Desc: "返回结果数量，默认 5，最大 10",
			},
		}),
	}, nil
}

// InvokableRun 执行检索
func (t *RAGRetrieverTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if params.Query == "" {
		return "错误：query 参数不能为空", nil
	}
	if params.TopK <= 0 || params.TopK > 10 {
		params.TopK = 5
	}

	results, err := t.retriever.Retrieve(ctx, &rag.RetrieveRequest{
		Query:     params.Query,
		UserID:    t.userID,
		SourceIDs: t.sourceIDs,
		TopK:      params.TopK,
	})
	fmt.Println("------------------", results)
	if err != nil {
		return "检索失败: " + err.Error(), nil
	}

	// 追加引用，多次检索时编号连续
	t.mu.Lock()
	currentOffset := t.offset
	if t.references != nil {
		for _, r := range results {
			*t.references = append(*t.references, response.Reference{
				SourceID:      r.SourceID,
				SourceName:    r.SourceName,
				ParentBlockID: r.ParentBlockID,
				ChunkContent:  r.Content,
				Score:         r.Score,
			})
		}
	}
	t.offset += len(results)
	t.mu.Unlock()

	fmt.Println("---------------", t, t.references)
	return FormatRetrievalResults(results, currentOffset), nil
}
