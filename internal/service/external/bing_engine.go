// internal/service/external/bing_engine.go
package external

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type bingEngine struct {
	apiKey string
	client *http.Client
}

// NewBingEngine 创建 Bing 搜索引擎（国内可访问）
func NewBingEngine(apiKey string) SearchEngine {
	return &bingEngine{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (e *bingEngine) Name() string {
	return "bing"
}

func (e *bingEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if limit <= 0 {
		limit = 10
	}

	searchURL := fmt.Sprintf(
		"https://api.bing.microsoft.com/v7.0/search?q=%s&count=%d&mkt=zh-CN",
		url.QueryEscape(query), limit,
	)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Bing搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Bing搜索返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var apiResp bingResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("解析Bing搜索结果失败: %w", err)
	}

	results := make([]SearchResultItem, 0, len(apiResp.WebPages.Value))
	for _, page := range apiResp.WebPages.Value {
		if len(results) >= limit {
			break
		}
		results = append(results, SearchResultItem{
			Title:   page.Name,
			URL:     page.URL,
			Snippet: page.Snippet,
		})
	}

	return results, nil
}

// bingResponse Bing API 响应结构
type bingResponse struct {
	WebPages struct {
		Value []struct {
			Name    string `json:"name"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
		} `json:"value"`
	} `json:"webPages"`
}
