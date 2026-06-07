package integration

import (
	"mime/multipart"
	"testing"

	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/entity"
	bizerrors "YoudaoNoteLm/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ======================== TC-025: 文件导入 ========================

func TestImportFile_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.ImportFileFunc = func(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error) {
		return &entity.Source{
			BaseEntity: entity.BaseEntity{ID: 1},
			UserID:     1,
			NotebookID: 1,
			Name:       "document.pdf",
			Type:       "file",
			Status:     "pending",
		}, nil
	}

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/notebooks/1/import/file", "file", "document.pdf", []byte("fake-pdf-data"), token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestImportFile_NoFile(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	req := newRequest("POST", "/api/v1/notebooks/1/import/file", nil, token)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := doRequest(env.Engine, req)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestImportFile_UnsupportedFormat(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.ImportFileFunc = func(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error) {
		return nil, bizerrors.ErrUnsupportedFormat
	}

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/notebooks/1/import/file", "file", "malware.exe", []byte("bad"), token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnsupportedFormat)
}

func TestImportFile_TooLarge(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.ImportFileFunc = func(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error) {
		return nil, bizerrors.ErrFileTooLarge
	}

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/notebooks/1/import/file", "file", "huge.pdf", []byte("data"), token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeFileTooLarge)
}

func TestImportFile_InvalidNbID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/notebooks/abc/import/file", "file", "test.pdf", []byte("data"), token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestImportFile_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/notebooks/1/import/file", "file", "test.pdf", []byte("data"), "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-026: 音频预览 ========================

func TestPreviewAudio_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.PreviewAudioFunc = func(userID, notebookID uint, file *multipart.FileHeader) (string, string, string, error) {
		return "preview-uuid", "转写文本内容...", "audio.mp3", nil
	}

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/notebooks/1/import/audio/preview", "file", "audio.mp3", []byte("fake-audio"), token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["preview_id"] == nil {
		t.Error("preview_id 不能为空")
	}
	if data["content"] == nil {
		t.Error("content 不能为空")
	}
}

func TestPreviewAudio_UnsupportedFormat(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.PreviewAudioFunc = func(userID, notebookID uint, file *multipart.FileHeader) (string, string, string, error) {
		return "", "", "", bizerrors.ErrUnsupportedFormat
	}

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/notebooks/1/import/audio/preview", "file", "document.txt", []byte("not audio"), token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnsupportedFormat)
}

func TestPreviewAudio_NoFile(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	req := newRequest("POST", "/api/v1/notebooks/1/import/audio/preview", nil, token)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := doRequest(env.Engine, req)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-027: 确认音频导入 ========================

func TestConfirmAudio_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.ConfirmAudioFunc = func(userID uint, previewID string, editedContent *string) (*entity.Source, error) {
		return &entity.Source{
			BaseEntity: entity.BaseEntity{ID: 1},
			UserID:     1,
			Name:       "audio.mp3",
			Type:       "audio",
			Status:     "pending",
		}, nil
	}

	content := "修改后的转写文本..."
	body := JSONBody(request.AudioConfirmRequest{
		PreviewID:  "preview-uuid",
		Content:    &content,
		NotebookID: 1,
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/import/audio/confirm", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestConfirmAudio_InvalidPreviewID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.ConfirmAudioFunc = func(userID uint, previewID string, editedContent *string) (*entity.Source, error) {
		return nil, bizerrors.ErrPreviewExpired
	}

	body := JSONBody(request.AudioConfirmRequest{
		PreviewID:  "invalid-uuid",
		NotebookID: 1,
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/import/audio/confirm", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodePreviewExpired)
}

func TestConfirmAudio_MissingPreviewID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"notebook_id": 1})
	w := MakeRequest(env.Engine, "POST", "/api/v1/import/audio/confirm", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestConfirmAudio_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(request.AudioConfirmRequest{
		PreviewID:  "preview-uuid",
		NotebookID: 1,
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/import/audio/confirm", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-028: 查询任务进度 ========================

func TestGetTask_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.GetImportTaskFunc = func(taskID string) (interface{}, error) {
		return gin.H{
			"task_id":       taskID,
			"task_type":     "file_import",
			"total_count":   5,
			"success_count": 3,
			"fail_count":    1,
			"status":        "processing",
		}, nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/import/tasks/test-task-uuid", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestGetTask_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Importer.GetImportTaskFunc = func(taskID string) (interface{}, error) {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "任务不存在")
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/import/tasks/nonexistent", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeResourceNotFound)
}

func TestGetTask_EmptyTaskID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	// 空 taskId 会被路由匹配为 /tasks/ 但可能 404
	w := MakeRequest(env.Engine, "GET", "/api/v1/import/tasks/", nil, token)
	// 404 或 400 都算合理
	code := w.Code
	if code != 404 && code != 400 {
		result := ParseResponse(t, w)
		AssertCode(t, result, bizerrors.CodeBadRequest)
	}
}
