package integration

import (
	"testing"
	"time"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	bizerrors "YoudaoNoteLm/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ======================== TC-018: 来源列表 ========================

func TestSourceList_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.ListFunc = func(userID, notebookID uint, keyword string, page, size int) ([]*response.SourceResponse, int64, error) {
		return []*response.SourceResponse{
			{ID: 1, NotebookID: 1, Name: "文档1", Type: "file", Status: "ready"},
			{ID: 2, NotebookID: 1, Name: "文档2", Type: "url", Status: "ready"},
		}, 2, nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources?page=1&size=10", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSourceList_WithKeyword(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.ListFunc = func(userID, notebookID uint, keyword string, page, size int) ([]*response.SourceResponse, int64, error) {
		if keyword != "测试" {
			t.Errorf("期望 keyword=测试, 实际=%s", keyword)
		}
		return []*response.SourceResponse{}, 0, nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources?page=1&size=10&keyword=测试", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSourceList_InvalidNbID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/abc/sources?page=1&size=10", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestSourceList_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources?page=1&size=10", nil, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-019: 来源详情 ========================

func TestSourceGetByID_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.GetByIDFunc = func(id uint) (*entity.Source, error) {
		return &entity.Source{
			BaseEntity: entity.BaseEntity{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			UserID:     1,
			NotebookID: 1,
			Name:       "测试文档",
			Type:       "file",
			Status:     "ready",
			Vectorized: true,
		}, nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources/1", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["name"] != "测试文档" {
		t.Errorf("期望 name=测试文档, 实际=%v", data["name"])
	}
}

func TestSourceGetByID_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources/99999", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeNotFound)
}

func TestSourceGetByID_InvalidID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources/abc", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-020: 重命名来源 ========================

func TestSourceRename_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"name": "新名称"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/1/sources/1", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSourceRename_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.RenameFunc = func(id uint, name string) error {
		return bizerrors.ErrNotFound
	}

	body := JSONBody(gin.H{"name": "新名称"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/1/sources/99999", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeNotFound)
}

func TestSourceRename_EmptyName(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"name": ""})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/1/sources/1", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-021: 删除来源 ========================

func TestSourceDelete_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	w := MakeRequest(env.Engine, "DELETE", "/api/v1/notebooks/1/sources/1", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSourceDelete_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.DeleteFunc = func(id uint) error {
		return bizerrors.ErrNotFound
	}

	w := MakeRequest(env.Engine, "DELETE", "/api/v1/notebooks/1/sources/99999", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeNotFound)
}

// ======================== TC-022: 批量删除 ========================

func TestSourceBatchDelete_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"ids": []uint{1, 2, 3}})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/sources/batch-delete", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSourceBatchDelete_EmptyList(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"ids": []uint{}})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/sources/batch-delete", body, token)
	result := ParseResponse(t, w)

	// ids 为空时 binding:"required,min=1" 应该返回 400
	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestSourceBatchDelete_MissingIDs(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/sources/batch-delete", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-023: 获取内容 ========================

func TestSourceGetContent_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.GetContentFunc = func(id uint) (string, error) {
		return "# 文档标题\n\n这是文档内容...", nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources/1/content", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSourceGetContent_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.GetContentFunc = func(id uint) (string, error) {
		return "", bizerrors.ErrNotFound
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources/99999/content", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeNotFound)
}

// ======================== TC-024: 获取原格式 ========================

func TestSourceGetOriginal_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.GetOriginalContentFunc = func(id uint) (string, string, error) {
		return "原始内容...", "text/plain", nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources/1/original", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSourceGetOriginal_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Source.GetOriginalContentFunc = func(id uint) (string, string, error) {
		return "", "", bizerrors.ErrNotFound
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks/1/sources/99999/original", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeNotFound)
}
