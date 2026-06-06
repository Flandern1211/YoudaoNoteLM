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
	PreviewAudio(userID, notebookID uint, file *multipart.FileHeader) (previewID string, content string, fileName string, err error)
	ConfirmAudio(userID uint, previewID string, editedContent *string) (*entity.Source, error)
	ImportSearchResults(userID, notebookID uint, urls []string) (taskID string, err error)
	GetImportTask(taskID string) (interface{}, error)
}
