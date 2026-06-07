// internal/model/dto/request/search.go
package request

// SearchRequest 智能搜索请求
type SearchRequest struct {
	Query string `json:"query" binding:"required,max=500"`
}

// URLImportRequest URL 直接导入请求
type URLImportRequest struct {
	URL string `json:"url" binding:"required,url"`
}

// SearchImportRequest 搜索结果批量导入请求
type SearchImportRequest struct {
	URLs []string `json:"urls" binding:"required,min=1"`
}
