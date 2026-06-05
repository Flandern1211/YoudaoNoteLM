package repository

import "YoudaoNoteLm/internal/model/entity"

// SourceRepository 资料来源仓储接口
type SourceRepository interface {
	FindByID(id int) (*entity.Source, error)
	Create(source *entity.Source) error
	Update(source *entity.Source) error
	Delete(id int) error
	BatchDelete(ids []int) error
	ListByNotebook(userID, notebookID int, keyword string, offset, limit int) ([]*entity.Source, int64, error)
	UpdateStatus(id int, status string, errMsg string) error
	SetVectorized(id int) error
}
