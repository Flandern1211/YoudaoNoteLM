// internal/service/external/duckduckgo_engine.go
package external

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type duckDuckGoEngine struct {
	httpClient *http.Client
}

// NewDuckDuckGoEngine 创建 DuckDuckGo 搜索引擎（兜底方案）
func NewDuckDuckGoEngine() SearchEngine {
	return &duckDuckGoEngine{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (e *duckDuckGoEngine) Name() string {
	return "duckduckgo"
}

func (e *duckDuckGoEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if limit <= 0 {
		limit = 10
	}

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DuckDuckGo请求失败: %w", err)
	}
	defer resp.Body.Close()

	body := make([]byte, 1024*1024)
	n, _ := resp.Body.Read(body)
	html := string(body[:n])

	results := parseDuckDuckGoResults(html, limit)
	return results, nil
}

func parseDuckDuckGoResults(html string, limit int) []SearchResultItem {
	var results []SearchResultItem

	titleRe := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`)
	snippetRe := regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>([^<]*(?:<[^>]*>[^<]*)*)</a>`)

	titles := titleRe.FindAllStringSubmatch(html, -1)
	snippets := snippetRe.FindAllStringSubmatch(html, -1)

	for i, match := range titles {
		if len(results) >= limit {
			break
		}
		if len(match) < 3 {
			continue
		}

		item := SearchResultItem{
			URL:   strings.TrimSpace(match[1]),
			Title: strings.TrimSpace(stripHTML(match[2])),
		}
		if i < len(snippets) && len(snippets[i]) > 1 {
			item.Snippet = strings.TrimSpace(stripHTML(snippets[i][1]))
		}

		if item.URL != "" && item.Title != "" {
			results = append(results, item)
		}
	}

	return results
}

func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}
