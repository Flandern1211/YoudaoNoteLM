package service

import (
	"YoudaoNoteLm/internal/service/external/asr"
	"YoudaoNoteLm/internal/service/external/document"
	"YoudaoNoteLm/internal/service/external/storage"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var allowedFileTypes = map[string]bool{
	".txt": true, ".md": true, ".docx": true, ".pdf": true, ".pptx": true,
}

var allowedAudioTypes = map[string]bool{
	".mp3": true, ".wav": true,
}

const maxFileSize int64 = 30 << 20   // 30MB
const maxAudioSize int64 = 300 << 20 // 300MB

type importerService struct {
	configSvc    ConfigService
	converter    document.DocumentConverter // 兜底默认
	storage      storage.FileStorage
	sourceRepo   repository.SourceRepository
	importCache  *cache.ImportTaskCache
	previewCache *cache.AudioPreviewCache
	embedding    EmbeddingService
}

// NewImporterService 创建导入服务
func NewImporterService(
	configSvc ConfigService,
	converter document.DocumentConverter,
	storage storage.FileStorage,
	sourceRepo repository.SourceRepository,
	importCache *cache.ImportTaskCache,
	previewCache *cache.AudioPreviewCache,
	embedding EmbeddingService,
) ImporterService {
	return &importerService{
		configSvc:    configSvc,
		converter:    converter,
		storage:      storage,
		sourceRepo:   sourceRepo,
		importCache:  importCache,
		previewCache: previewCache,
		embedding:    embedding,
	}
}

// getConverter 获取文档转换器（直接使用配置文件中的 markitdown 客户端）
func (s *importerService) getConverter() document.DocumentConverter {
	return s.converter
}

// getASR 获取 ASR 服务（从 ConfigService 动态获取）
func (s *importerService) getASR() (asr.ASRService, error) {
	if s.configSvc != nil {
		return s.configSvc.GetASRService(0) // userID=0 表示系统级
	}
	return nil, fmt.Errorf("未配置 ConfigService")
}

// ImportFile 文件上传导入
func (s *importerService) ImportFile(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedFileTypes[ext] {
		return nil, bizerrors.ErrUnsupportedFormat
	}
	if file.Size > maxFileSize {
		return nil, bizerrors.ErrFileTooLarge
	}

	// 读取文件内容（用于 MarkItDown 转换）
	src, err := file.Open()
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "打开上传文件失败", err)
	}
	fileBytes, err := io.ReadAll(src)
	src.Close()
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "读取上传文件失败", err)
	}

	// 上传到 MinIO 存储
	if s.storage == nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "文件存储服务未配置", nil)
	}
	// 重新打开文件用于上传
	src2, err := file.Open()
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "重新打开文件失败", err)
	}
	defer src2.Close()
	filePath, err := s.storage.Upload(file)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "文件上传失败", err)
	}

	// 获取文档转换器（优先从 ConfigService 读取数据库配置）
	converter := s.getConverter()

	// 通过 io.Reader 传给文档转换服务
	markdown, err := converter.ConvertReader(file.Filename, bytes.NewReader(fileBytes))
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeFileParseFailed, "文件解析失败", err)
	}

	source := &entity.Source{
		UserID:          userID,
		NotebookID:      notebookID,
		Name:            file.Filename,
		Type:            "file",
		FilePath:        filePath,
		FileSize:        file.Size,
		MimeType:        file.Header.Get("Content-Type"),
		MarkdownContent: markdown,
		Status:          "ready",
	}

	if err := s.sourceRepo.Create(source); err != nil {
		return nil, err
	}

	// 异步向量化
	if s.embedding != nil {
		go func() {
			if err := s.embedding.Vectorize(source.ID, markdown); err != nil {
				logger.Warn("向量化失败", zap.Uint("source_id", source.ID), zap.Error(err))
			} else {
				if err := s.sourceRepo.SetVectorized(source.ID); err != nil {
					logger.Warn("标记向量化状态失败", zap.Uint("source_id", source.ID), zap.Error(err))
				}
			}
		}()
	}

	return source, nil
}

// PreviewAudio 音频上传转写预览
func (s *importerService) PreviewAudio(userID, notebookID uint, file *multipart.FileHeader) (string, string, string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedAudioTypes[ext] {
		return "", "", "", bizerrors.ErrUnsupportedFormat
	}
	if file.Size > maxAudioSize {
		return "", "", "", bizerrors.ErrFileTooLarge
	}

	// 上传原始文件到 MinIO
	if s.storage == nil {
		return "", "", "", bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "文件存储服务未配置", nil)
	}
	filePath, err := s.storage.Upload(file)
	if err != nil {
		return "", "", "", bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "音频上传失败", err)
	}

	// 转换音频为阿里云 ASR 兼容格式（16kHz 单声道 WAV）
	asrFilePath := filePath
	convertedData, convErr := s.convertAudioForASR(file, filePath, ext)
	if convErr != nil {
		logger.Warn("音频格式转换失败，将使用原始文件", zap.String("file", filePath), zap.Error(convErr))
	} else if convertedData != nil {
		// 上传转换后的文件到 MinIO
		asrPath := strings.TrimSuffix(filePath, ext) + "_16k.wav"
		if uploadErr := s.storage.UploadBytes(asrPath, convertedData, "audio/wav"); uploadErr != nil {
			logger.Warn("上传转换后音频失败，将使用原始文件", zap.String("file", filePath), zap.Error(uploadErr))
		} else {
			asrFilePath = asrPath
			logger.Info("音频格式转换成功",
				zap.String("original", filePath),
				zap.String("converted", asrPath),
			)
		}
	}

	// 获取 ASR 服务（从数据库配置动态加载）
	asrSvc, err := s.getASR()
	if err != nil {
		return "", "", "", bizerrors.NewWithErr(bizerrors.CodeASTranscriptionFailed, "未配置 ASR 服务", err)
	}

	text, err := asrSvc.Transcribe(asrFilePath)
	if err != nil {
		logger.Error("ASR转写失败", zap.String("file", filePath), zap.Error(err))
		return "", "", "", bizerrors.NewWithErr(bizerrors.CodeASTranscriptionFailed, "音频转写失败", err)
	}

	previewID := uuid.New().String()
	preview := &cache.AudioPreview{
		PreviewID:       previewID,
		UserID:          userID,
		NotebookID:      notebookID,
		FileName:        file.Filename,
		FilePath:        filePath,
		FileSize:        file.Size,
		TranscribedText: text,
		Status:          "pending",
		ExpiresAt:       time.Now().Add(30 * time.Minute).Unix(),
	}

	ctx := context.Background()
	if err := s.previewCache.Save(ctx, preview); err != nil {
		return "", "", "", err
	}

	return previewID, text, file.Filename, nil
}

// ConfirmAudio 确认音频导入
func (s *importerService) ConfirmAudio(userID uint, previewID string, editedContent *string) (*entity.Source, error) {
	ctx := context.Background()
	preview, err := s.previewCache.Get(ctx, previewID)
	if err != nil {
		return nil, bizerrors.ErrNotFound
	}
	if preview == nil {
		return nil, bizerrors.ErrNotFound
	}
	if preview.UserID != userID {
		return nil, bizerrors.ErrForbidden
	}
	if time.Now().Unix() > preview.ExpiresAt {
		return nil, bizerrors.ErrPreviewExpired
	}

	content := preview.TranscribedText
	if editedContent != nil && *editedContent != "" {
		content = *editedContent
	}

	source := &entity.Source{
		UserID:          userID,
		NotebookID:      preview.NotebookID,
		Name:            preview.FileName,
		Type:            "audio",
		FilePath:        preview.FilePath,
		FileSize:        preview.FileSize,
		MarkdownContent: content,
		Status:          "ready",
	}

	if err := s.sourceRepo.Create(source); err != nil {
		return nil, err
	}

	if err := s.previewCache.UpdateStatus(ctx, previewID, "confirmed"); err != nil {
		logger.Warn("更新预览状态失败", zap.String("preview_id", previewID), zap.Error(err))
	}

	// 异步向量化
	if s.embedding != nil {
		go func() {
			if err := s.embedding.Vectorize(source.ID, content); err != nil {
				logger.Warn("音频向量化失败", zap.Uint("source_id", source.ID), zap.Error(err))
			} else {
				if err := s.sourceRepo.SetVectorized(source.ID); err != nil {
					logger.Warn("标记向量化状态失败", zap.Uint("source_id", source.ID), zap.Error(err))
				}
			}
		}()
	}

	return source, nil
}

// convertAudioForASR 转换音频为 ASR 兼容格式
// 如果已经是 16kHz 单声道则返回 nil（无需转换）
func (s *importerService) convertAudioForASR(file *multipart.FileHeader, filePath, ext string) ([]byte, error) {
	// 读取文件内容
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer src.Close()

	audioData, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("读取音频文件失败: %w", err)
	}

	// 转换为 16kHz 单声道 WAV
	converted, err := utils.ConvertBytesToASRFormat(audioData, ext)
	if err != nil {
		return nil, fmt.Errorf("音频转换失败: %w", err)
	}

	return converted, nil
}

// ImportSearchResults 批量导入搜索结果
func (s *importerService) ImportSearchResults(userID, notebookID uint, urls []string) (string, error) {
	taskID := uuid.New().String()
	task := &cache.ImportTask{
		TaskID:     taskID,
		UserID:     userID,
		NotebookID: notebookID,
		TaskType:   "search",
		TotalCount: len(urls),
		Status:     "running",
		CreatedAt:  time.Now().Unix(),
	}

	ctx := context.Background()
	if err := s.importCache.Save(ctx, task); err != nil {
		return "", err
	}

	go s.processURLs(taskID, userID, notebookID, urls)
	return taskID, nil
}

// processURLs 异步处理 URL 列表
func (s *importerService) processURLs(taskID string, userID, notebookID uint, urls []string) {
	ctx := context.Background()
	for i, url := range urls {
		// 检查任务是否已被取消或删除
		task, err := s.importCache.Get(ctx, taskID)
		if err != nil || task == nil {
			logger.Info("导入任务已被取消或删除，停止处理", zap.String("task_id", taskID))
			return
		}
		if task.Status == "cancelled" {
			logger.Info("导入任务已取消，停止处理", zap.String("task_id", taskID))
			return
		}

		// 更新进度
		task.Status = "running"
		task.ProcessedCount = i
		if err := s.importCache.Save(ctx, task); err != nil {
			logger.Warn("更新任务进度失败", zap.String("task_id", taskID), zap.Error(err))
		}

		// 获取文档转换器
		converter := s.getConverter()

		// 预检：判断 URL 是否可抓取
		ok, ct, checkErr := converter.CheckURL(url)
		if !ok {
			logger.Warn("URL预检不通过，跳过", zap.String("url", url), zap.String("content_type", ct), zap.Error(checkErr))
			s.incrementTaskFail(ctx, taskID, fmt.Sprintf("%s: 不可抓取 (Content-Type: %s)", url, ct))
			continue
		}
		logger.Info("URL预检通过，开始转换", zap.String("url", url), zap.String("content_type", ct))

		markdown, err := converter.ConvertFromURL(url)
		if err != nil {
			logger.Warn("URL转换失败", zap.String("url", url), zap.Error(err))
			s.incrementTaskFail(ctx, taskID, fmt.Sprintf("%s: %v", url, err))
			continue
		}

		// 从 URL 提取名称，截断到 255 字符以内
		name := url
		if len(name) > 200 {
			name = name[:200] + "..."
		}

		source := &entity.Source{
			UserID:          userID,
			NotebookID:      notebookID,
			Name:            name,
			Type:            "url",
			OriginalURL:     url,
			MarkdownContent: markdown,
			Status:          "ready",
		}
		if err := s.sourceRepo.Create(source); err != nil {
			s.incrementTaskFail(ctx, taskID, fmt.Sprintf("%s: %v", url, err))
			continue
		}

		// 异步向量化
		if s.embedding != nil {
			go func(srcID uint, content string) {
				if err := s.embedding.Vectorize(srcID, content); err != nil {
					logger.Warn("向量化失败", zap.Uint("source_id", srcID), zap.Error(err))
				} else {
					if err := s.sourceRepo.SetVectorized(srcID); err != nil {
						logger.Warn("标记向量化状态失败", zap.Uint("source_id", srcID), zap.Error(err))
					}
				}
			}(source.ID, markdown)
		}

		s.incrementTaskSuccess(ctx, taskID)
	}

	// 更新任务最终状态
	task, err := s.importCache.Get(ctx, taskID)
	if err != nil {
		logger.Error("获取导入任务失败", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task != nil && task.Status != "cancelled" {
		if task.FailCount > 0 && task.SuccessCount > 0 {
			task.Status = "partial_failed"
		} else if task.FailCount > 0 {
			task.Status = "failed"
		} else {
			task.Status = "completed"
		}
		if err := s.importCache.Save(ctx, task); err != nil {
			logger.Error("保存导入任务最终状态失败", zap.String("task_id", taskID), zap.Error(err))
		}
	}
}

// incrementTaskFail 增加失败计数
func (s *importerService) incrementTaskFail(ctx context.Context, taskID string, errMsg string) {
	task, err := s.importCache.Get(ctx, taskID)
	if err != nil {
		logger.Error("获取导入任务失败", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task == nil {
		logger.Warn("导入任务不存在", zap.String("task_id", taskID))
		return
	}
	task.FailCount++
	if task.ErrorDetail != "" {
		task.ErrorDetail = task.ErrorDetail + "|" + errMsg
	} else {
		task.ErrorDetail = errMsg
	}
	if err := s.importCache.Save(ctx, task); err != nil {
		logger.Error("保存导入任务失败计数失败", zap.String("task_id", taskID), zap.Error(err))
	}
}

// incrementTaskSuccess 增加成功计数
func (s *importerService) incrementTaskSuccess(ctx context.Context, taskID string) {
	task, err := s.importCache.Get(ctx, taskID)
	if err != nil {
		logger.Error("获取导入任务失败", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task == nil {
		logger.Warn("导入任务不存在", zap.String("task_id", taskID))
		return
	}
	task.SuccessCount++
	if err := s.importCache.Save(ctx, task); err != nil {
		logger.Error("保存导入任务成功计数失败", zap.String("task_id", taskID), zap.Error(err))
	}
}

// GetImportTask 获取导入任务状态
func (s *importerService) GetImportTask(taskID string) (interface{}, error) {
	ctx := context.Background()
	task, err := s.importCache.Get(ctx, taskID)
	if err != nil {
		return nil, bizerrors.ErrNotFound
	}
	if task == nil {
		return nil, bizerrors.ErrNotFound
	}
	return task, nil
}

// DeleteImportTask 删除导入任务
func (s *importerService) DeleteImportTask(taskID string) error {
	ctx := context.Background()

	// 先检查任务是否存在
	task, err := s.importCache.Get(ctx, taskID)
	if err != nil {
		return bizerrors.ErrNotFound
	}
	if task == nil {
		return bizerrors.ErrNotFound
	}

	// 如果任务正在运行中，标记为取消状态
	if task.Status == "running" {
		task.Status = "cancelled"
		if err := s.importCache.Save(ctx, task); err != nil {
			logger.Warn("更新任务状态为取消失败", zap.String("task_id", taskID), zap.Error(err))
		}
	}

	// 删除任务缓存
	return s.importCache.Delete(ctx, taskID)
}
