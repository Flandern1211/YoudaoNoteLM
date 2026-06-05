package service

import (
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
)

// SourceService 资料来源服务接口
type SourceService interface {
	List(userID, notebookID int, keyword string, page, size int) ([]*response.SourceResponse, int64, error)
	GetByID(id int) (*entity.Source, error)
	Rename(id int, name string) error
	Delete(id int) error
	BatchDelete(ids []int) error
	GetContent(id int) (string, error)
	GetOriginalContent(id int) (content string, contentType string, err error)
}
