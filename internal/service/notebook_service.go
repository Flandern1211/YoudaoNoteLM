package service

import (
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/logger"
	"context"
	"time"

	bizerrors "YoudaoNoteLm/pkg/errors"
	"go.uber.org/zap"
)

// notebookService 笔记本服务实现
type notebookService struct {
	notebookRepo repository.NotebookRepository
	sourceRepo   repository.SourceRepository
	ingestionSvc rag.IngestionService
}

// NewNotebookService 创建笔记本服务
func NewNotebookService(notebookRepo repository.NotebookRepository, sourceRepo repository.SourceRepository, ingestionSvc rag.IngestionService) NotebookService {
	return &notebookService{
		notebookRepo: notebookRepo,
		sourceRepo:   sourceRepo,
		ingestionSvc: ingestionSvc,
	}
}

// Create 创建笔记本
func (s *notebookService) Create(userID uint, req *request.CreateNotebookRequest) (*response.NotebookResponse, error) {
	// 检查是否存在同名笔记本
	exists, err := s.notebookRepo.ExistsByName(userID, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, bizerrors.New(bizerrors.CodeConflict, "已存在同名笔记本")
	}

	notebook := &entity.Notebook{
		UserID: userID,
		Name:   req.Name,
	}

	if err := s.notebookRepo.Create(notebook); err != nil {
		return nil, err
	}

	return s.toResponse(notebook), nil
}

// List 查询用户的所有笔记本
func (s *notebookService) List(userID uint) ([]*response.NotebookResponse, error) {
	notebooks, err := s.notebookRepo.ListByUserID(userID)
	if err != nil {
		return nil, err
	}

	result := make([]*response.NotebookResponse, 0, len(notebooks))
	for _, nb := range notebooks {
		result = append(result, s.toResponse(nb))
	}
	return result, nil
}

// Rename 重命名笔记本
func (s *notebookService) Rename(userID, notebookID uint, req *request.RenameNotebookRequest) error {
	notebook, err := s.notebookRepo.FindByID(notebookID)
	if err != nil {
		return err
	}
	if notebook == nil {
		return bizerrors.ErrNotFound
	}

	// 检查权限
	if notebook.UserID != userID {
		return bizerrors.ErrForbidden
	}

	// 检查是否存在同名笔记本（排除自身）
	if notebook.Name != req.Name {
		exists, err := s.notebookRepo.ExistsByName(userID, req.Name)
		if err != nil {
			return err
		}
		if exists {
			return bizerrors.New(bizerrors.CodeConflict, "已存在同名笔记本")
		}
	}

	notebook.Name = req.Name
	return s.notebookRepo.Update(notebook)
}

// Delete 删除笔记本
func (s *notebookService) Delete(userID, notebookID uint) error {
	notebook, err := s.notebookRepo.FindByID(notebookID)
	if err != nil {
		return err
	}
	if notebook == nil {
		return bizerrors.ErrNotFound
	}

	// 检查权限
	if notebook.UserID != userID {
		return bizerrors.ErrForbidden
	}

	// 删除该笔记本下所有 source 的向量数据
	if s.ingestionSvc != nil && s.sourceRepo != nil {
		s.deleteNotebookVectors(userID, notebookID)
	}

	return s.notebookRepo.Delete(notebookID)
}

// deleteNotebookVectors 删除笔记本下所有 source 的向量数据
func (s *notebookService) deleteNotebookVectors(userID, notebookID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 查询该笔记本下所有 source
	sources, _, err := s.sourceRepo.ListByNotebook(userID, notebookID, "", 0, 10000)
	if err != nil {
		logger.Error("查询笔记本关联的 source 失败",
			zap.Uint("notebook_id", notebookID),
			zap.Error(err),
		)
		return
	}

	// 逐个删除向量数据
	for _, source := range sources {
		if source.Vectorized {
			if err := s.ingestionSvc.DeleteSource(ctx, userID, source.ID); err != nil {
				logger.Error("删除笔记本关联的源向量数据失败",
					zap.Uint("notebook_id", notebookID),
					zap.Uint("source_id", source.ID),
					zap.Error(err),
				)
			}
		}
	}
}

// toResponse 转换为响应 DTO
func (s *notebookService) toResponse(notebook *entity.Notebook) *response.NotebookResponse {
	return &response.NotebookResponse{
		ID:        notebook.ID,
		Name:      notebook.Name,
		CreatedAt: notebook.CreatedAt,
		UpdatedAt: notebook.UpdatedAt,
	}
}
