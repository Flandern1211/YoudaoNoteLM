package integration

import (
	"testing"

	"YoudaoNoteLm/internal/model/dto/response"
	bizerrors "YoudaoNoteLm/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ======================== TC-029: 智能搜索 ========================

func TestSearch_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.SearchFunc = func(userID, notebookID uint, query string) (*response.SearchResponse, error) {
		return &response.SearchResponse{
			Results: []response.SearchResultItem{
				{
					Title:   "2026年AI发展趋势",
					URL:     "https://example.com/ai-trends",
					Snippet: "人工智能在2026年...",
					Score:   8.5,
					Reason:  "内容相关度高，来源权威",
				},
			},
			Summary:      "根据搜索结果，2026年人工智能...",
			SearchRounds: 2,
		}, nil
	}

	body := JSONBody(gin.H{"query": "人工智能发展趋势"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["summary"] == nil {
		t.Error("summary 不能为空")
	}
	if data["search_rounds"].(float64) != 2 {
		t.Errorf("期望 search_rounds=2, 实际=%v", data["search_rounds"])
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"query": ""})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestSearch_QueryTooLong(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	longQuery := ""
	for i := 0; i < 501; i++ {
		longQuery += "搜"
	}
	body := JSONBody(gin.H{"query": longQuery})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestSearch_InvalidNbID(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"query": "test"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/abc/search", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestSearch_LLMNotConfigured(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.SearchFunc = func(userID, notebookID uint, query string) (*response.SearchResponse, error) {
		return nil, bizerrors.ErrLLMNotConfigured
	}

	body := JSONBody(gin.H{"query": "test"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeLLMNotConfigured)
}

func TestSearch_LLMCallFailed(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.SearchFunc = func(userID, notebookID uint, query string) (*response.SearchResponse, error) {
		return nil, bizerrors.ErrLLMCallFailed
	}

	body := JSONBody(gin.H{"query": "test"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeLLMCallFailed)
}

func TestSearch_Timeout(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.SearchFunc = func(userID, notebookID uint, query string) (*response.SearchResponse, error) {
		return nil, bizerrors.ErrSearchAgentTimeout
	}

	body := JSONBody(gin.H{"query": "复杂查询"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeSearchAgentTimeout)
}

func TestSearch_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"query": "test"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-030: URL 直接导入 ========================

func TestImportFromURL_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.ImportFromURLFunc = func(userID, notebookID uint, url string) (string, error) {
		return "task-uuid-123", nil
	}

	body := JSONBody(gin.H{"url": "https://example.com/article"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/url", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["task_id"] == nil {
		t.Error("task_id 不能为空")
	}
}

func TestImportFromURL_InvalidURL(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"url": "invalid-url"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/url", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestImportFromURL_ScrapeFailed(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.ImportFromURLFunc = func(userID, notebookID uint, url string) (string, error) {
		return "", bizerrors.ErrWebScrapeFailed
	}

	body := JSONBody(gin.H{"url": "https://unreachable.example.com"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/url", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeWebScrapeFailed)
}

func TestImportFromURL_DuplicateImport(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.ImportFromURLFunc = func(userID, notebookID uint, url string) (string, error) {
		return "", bizerrors.ErrDuplicateImport
	}

	body := JSONBody(gin.H{"url": "https://example.com/already-imported"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/url", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeDuplicateImport)
}

func TestImportFromURL_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"url": "https://example.com"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/url", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-031: 批量导入搜索结果 ========================

func TestImportSearchResults_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.Search.ImportSearchResultsFunc = func(userID, notebookID uint, urls []string) (string, error) {
		return "batch-task-uuid", nil
	}

	body := JSONBody(gin.H{"urls": []string{
		"https://example.com/article1",
		"https://example.com/article2",
	}})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/import", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["task_id"] == nil {
		t.Error("task_id 不能为空")
	}
}

func TestImportSearchResults_EmptyList(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"urls": []string{}})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/import", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestImportSearchResults_MissingURLs(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/import", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestImportSearchResults_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"urls": []string{"https://example.com"}})
	w := MakeRequest(env.Engine, "POST", "/api/v1/notebooks/1/search/import", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}
