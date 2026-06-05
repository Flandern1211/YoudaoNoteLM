package request

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []int `json:"ids" binding:"required,min=1"`
}
