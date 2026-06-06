// internal/service/external/searxng_engine.go
package external

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type searxngEngine struct {
	baseURL string
	client  *http.Client
}

// NewSearXNGEngine 创建 SearXNG 搜索引擎（自部署）
func NewSearXNGEngine(baseURL string) SearchEngine {
	return &searxngEngine{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (e *searxngEngine) Name() string {
	return "searxng"
}

func (e *searxngEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if limit <= 0 {
		limit = 10
	}

	searchURL := fmt.Sprintf("%s/search?q=%s&format=json", e.baseURL, url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YoudaoNoteLM/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SearXNG请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SearXNG返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var apiResp searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("解析SearXNG结果失败: %w", err)
	}

	results := make([]SearchResultItem, 0, len(apiResp.Results))
	for _, r := range apiResp.Results {
		if len(results) >= limit {
			break
		}
		results = append(results, SearchResultItem{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}

// searxngResponse SearXNG API 响应结构
type searxngResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}
