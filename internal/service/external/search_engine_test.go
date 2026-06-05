package external

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- 接口合规性测试 ---

func TestDuckDuckGoEngine_ImplementsInterface(t *testing.T) {
	var _ SearchEngine = NewDuckDuckGoEngine()
}

func TestCustomEngine_ImplementsInterface(t *testing.T) {
	var _ SearchEngine = NewCustomEngine("test", "http://localhost", "key")
}

func TestEmbeddingService_InterfaceExists(t *testing.T) {
	// EmbeddingService 是占位接口，验证编译期合规性
	var _ EmbeddingService = nil // 接口定义存在即可
}

// --- DuckDuckGoEngine 单元测试 ---

func TestDuckDuckGoEngine_Name(t *testing.T) {
	engine := NewDuckDuckGoEngine()
	if engine.Name() != "duckduckgo" {
		t.Errorf("expected 'duckduckgo', got '%s'", engine.Name())
	}
}

func TestDuckDuckGoEngine_Search_Integration(t *testing.T) {
	t.Skip("跳过网络集成测试（需翻墙，手动测试时去掉 Skip）")

	engine := NewDuckDuckGoEngine()
	results, err := engine.Search("golang", 5)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	t.Logf("返回 %d 条结果", len(results))
	for i, r := range results {
		t.Logf("  [%d] %s - %s", i+1, r.Title, r.URL)
	}

	// DuckDuckGo HTML 接口可能返回空结果（反爬），不强制要求 >0
	// 但结构必须合法
	for _, r := range results {
		if r.Title == "" {
			t.Error("结果 Title 不能为空")
		}
		if r.URL == "" {
			t.Error("结果 URL 不能为空")
		}
	}
}

// --- CustomEngine 单元测试（Mock HTTP Server） ---

func TestCustomEngine_Name(t *testing.T) {
	engine := NewCustomEngine("my-search", "http://localhost", "key")
	if engine.Name() != "my-search" {
		t.Errorf("expected 'my-search', got '%s'", engine.Name())
	}
}

func TestCustomEngine_Search_Success(t *testing.T) {
	// 启动 mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和头
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// 验证请求体
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["query"] != "golang" {
			t.Errorf("expected query 'golang', got '%v'", reqBody["query"])
		}

		// 返回响应
		resp := map[string]interface{}{
			"results": []SearchResultItem{
				{Title: "Go 官网", URL: "https://go.dev", Snippet: "Go 语言官方网站"},
				{Title: "Go Doc", URL: "https://pkg.go.dev", Snippet: "Go 包文档"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "test-key")
	results, err := engine.Search("golang", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "Go 官网" {
		t.Errorf("expected 'Go 官网', got '%s'", results[0].Title)
	}
	if results[0].URL != "https://go.dev" {
		t.Errorf("expected 'https://go.dev', got '%s'", results[0].URL)
	}
	if results[1].Snippet != "Go 包文档" {
		t.Errorf("expected 'Go 包文档', got '%s'", results[1].Snippet)
	}
}

func TestCustomEngine_Search_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "")
	_, err := engine.Search("test", 10)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	t.Logf("正确捕获错误: %v", err)
}

func TestCustomEngine_Search_NoAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 无 apiKey 时不应发送 Authorization 头
		if r.Header.Get("Authorization") != "" {
			t.Error("无 apiKey 时不应发送 Authorization 头")
		}
		resp := map[string]interface{}{"results": []SearchResultItem{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "") // 空 apiKey
	results, err := engine.Search("test", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if results == nil {
		t.Error("results 不应为 nil，应为空切片")
	}
}

func TestCustomEngine_Search_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "")
	_, err := engine.Search("test", 10)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	t.Logf("正确捕获 JSON 解析错误: %v", err)
}

// --- stripHTML 测试 ---

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<b>hello</b>", "hello"},
		{"<a href='url'>link</a>", "link"},
		{"no tags", "no tags"},
		{"<p>multi <b>bold</b> text</p>", "multi bold text"},
		{"", ""},
	}

	for _, tt := range tests {
		result := stripHTML(tt.input)
		if result != tt.expected {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// --- parseDuckDuckGoResults 测试 ---

func TestParseDuckDuckGoResults_EmptyHTML(t *testing.T) {
	results := parseDuckDuckGoResults("", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty HTML, got %d", len(results))
	}
}

func TestParseDuckDuckGoResults_Limit(t *testing.T) {
	// 构造包含多条结果的 HTML
	html := `
	<a class="result__a" href="https://example.com/1">Title 1</a>
	<a class="result__snippet">Snippet 1</a>
	<a class="result__a" href="https://example.com/2">Title 2</a>
	<a class="result__snippet">Snippet 2</a>
	<a class="result__a" href="https://example.com/3">Title 3</a>
	<a class="result__snippet">Snippet 3</a>
	`
	results := parseDuckDuckGoResults(html, 2)
	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}
