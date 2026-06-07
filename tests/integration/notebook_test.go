package integration

import (
	"testing"
	"time"

	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	bizerrors "YoudaoNoteLm/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ======================== TC-014: 创建笔记本 ========================

func TestCreateNotebook_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(request.CreateNotebookRequest{Name: "我的笔记本"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["name"] != "我的笔记本" {
		t.Errorf("期望 name=我的笔记本, 实际=%v", data["name"])
	}
}

func TestCreateNotebook_EmptyName(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"name": ""})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestCreateNotebook_NameTooLong(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	longName := ""
	for i := 0; i < 101; i++ {
		longName += "a"
	}
	body := JSONBody(gin.H{"name": longName})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestCreateNotebook_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"name": "test"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-015: 笔记本列表 ========================

func TestListNotebooks_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Notebook.ListFunc = func(userID uint) ([]*dto.NotebookResponse, error) {
		return []*dto.NotebookResponse{
			{ID: 1, Name: "笔记本1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: 2, Name: "笔记本2", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		}, nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestListNotebooks_Empty(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "newuser")

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestListNotebooks_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	w := MakeRequest(env.Engine, "GET", "/api/v1/notebooks", nil, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-016: 重命名笔记本 ========================

func TestRenameNotebook_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(request.RenameNotebookRequest{Name: "新名称"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/1", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestRenameNotebook_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Notebook.RenameFunc = func(userID, notebookID uint, req *request.RenameNotebookRequest) error {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "笔记本不存在")
	}

	body := JSONBody(request.RenameNotebookRequest{Name: "新名称"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/99999", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeResourceNotFound)
}

func TestRenameNotebook_EmptyName(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"name": ""})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/1", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestRenameNotebook_InvalidID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(request.RenameNotebookRequest{Name: "新名称"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/abc", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestRenameNotebook_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"name": "test"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/notebooks/1", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-017: 删除笔记本 ========================

func TestDeleteNotebook_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	w := MakeRequest(env.Engine, "DELETE", "/api/v1/notebooks/1", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestDeleteNotebook_NotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Notebook.DeleteFunc = func(userID, notebookID uint) error {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "笔记本不存在")
	}

	w := MakeRequest(env.Engine, "DELETE", "/api/v1/notebooks/99999", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeResourceNotFound)
}

func TestDeleteNotebook_InvalidID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	w := MakeRequest(env.Engine, "DELETE", "/api/v1/notebooks/abc", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestDeleteNotebook_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	w := MakeRequest(env.Engine, "DELETE", "/api/v1/notebooks/1", nil, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}
