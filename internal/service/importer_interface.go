package service

import (
	"YoudaoNoteLm/internal/model/entity"
	"mime/multipart"
)

// EmbeddingService 向量化服务接口（外部模块实现）
type EmbeddingService interface {
	Vectorize(sourceID uint, content string) error
}

// ImporterService 导入服务接口
type ImporterService interface {
	ImportFile(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error)
	// PreviewAudio 异步音频转写：上传文件后立即返回 previewID，后台执行 ASR 转写
	PreviewAudio(userID, notebookID uint, file *multipart.FileHeader) (previewID string, fileName string, err error)
	// GetAudioPreviewStatus 查询音频预览状态（前端轮询用）
	GetAudioPreviewStatus(userID uint, previewID string) (interface{}, error)
	ConfirmAudio(userID uint, previewID string, editedContent *string) (*entity.Source, error)
	// ImportSearchResults 批量导入搜索结果，返回任务 ID 和创建的 Source ID 列表
	ImportSearchResults(userID, notebookID uint, urls []string) (taskID string, sourceIDs []uint, err error)
	GetImportTask(taskID string) (interface{}, error)
	DeleteImportTask(taskID string) error // 删除导入任务
}
