package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type markitdownClient struct {
	baseURL        string
	httpClient     *http.Client
	prefetchClient *http.Client
}

// NewMarkitdownClient 创建 MarkItDown HTTP 客户端
func NewMarkitdownClient(baseURL string) DocumentConverter {
	return &markitdownClient{
		baseURL:        baseURL,
		httpClient:     &http.Client{Timeout: 60 * time.Second},
		prefetchClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Convert 本地文件转 Markdown（上传文件到 MarkItDown 服务）
func (c *markitdownClient) Convert(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return c.ConvertReader(filepath.Base(filePath), file)
}

// ConvertReader 通过 io.Reader 上传文件转 Markdown
func (c *markitdownClient) ConvertReader(filename string, reader io.Reader) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("创建表单文件失败: %w", err)
	}
	if _, err := io.Copy(part, reader); err != nil {
		return "", fmt.Errorf("写入文件内容失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭multipart writer失败: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/convert", writer.FormDataContentType(), body)
	if err != nil {
		return "", fmt.Errorf("请求MarkItDown失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("MarkItDown返回错误 %d, 且读取响应体失败: %w", resp.StatusCode, readErr)
		}
		return "", fmt.Errorf("MarkItDown返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// MarkItDown Python 服务返回 {"filename": "...", "markdown": "..."}
	var result struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// 降级：返回原始响应
		return string(respBody), nil
	}

	logger.Info("MarkItDown转换成功", zap.String("file", filename))
	return result.Markdown, nil
}

// ConvertFromURL 网页 URL 转 Markdown
func (c *markitdownClient) ConvertFromURL(url string) (string, error) {
	// MarkItDown 服务的 /convert_url 使用 Form 表单
	formBody := &bytes.Buffer{}
	writer := multipart.NewWriter(formBody)
	if err := writer.WriteField("url", url); err != nil {
		return "", fmt.Errorf("写入URL表单字段失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭writer失败: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/convert_url", writer.FormDataContentType(), formBody)
	if err != nil {
		return "", fmt.Errorf("请求MarkItDown URL转换失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("MarkItDown URL转换返回错误 %d, 且读取响应体失败: %w", resp.StatusCode, readErr)
		}
		return "", fmt.Errorf("MarkItDown URL转换返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// MarkItDown Python 服务返回 {"url": "...", "markdown": "..."} 或 {"url": "...", "markdown": "", "message": "..."}
	var result struct {
		Markdown string `json:"markdown"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	if result.Markdown == "" && result.Message != "" {
		logger.Warn("MarkItDown URL转换无内容", zap.String("url", url), zap.String("message", result.Message))
		return "", fmt.Errorf("URL转换失败: %s", result.Message)
	}

	if result.Markdown == "" {
		return "", fmt.Errorf("URL转换失败: 未能从该网页提取内容")
	}

	logger.Info("MarkItDown URL转换成功", zap.String("url", url))
	return result.Markdown, nil
}

// CheckURL 预检 URL 是否可抓取
// 返回 (可抓取, 内容类型, 错误)
// 判断逻辑：
// 1. 状态码 2xx
// 2. Content-Type 为 text/html、text/plain 等文本类型
// 3. 非文本内容（视频、音频、二进制等）直接拒绝
func (c *markitdownClient) CheckURL(url string) (bool, string, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, "", fmt.Errorf("创建预检请求失败: %w", err)
	}
	resp, err := c.prefetchClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("预检请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", fmt.Errorf("HTTP状态码 %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	ct = strings.ToLower(ct)

	// 可解析的文本类型
	supportedTypes := []string{
		"text/html",
		"text/plain",
		"application/xhtml+xml",
		"application/xml",
		"text/xml",
	}

	for _, st := range supportedTypes {
		if strings.Contains(ct, st) {
			return true, ct, nil
		}
	}

	// 未提供 Content-Type 时默认尝试
	if ct == "" {
		return true, "unknown", nil
	}

	return false, ct, fmt.Errorf("不支持的内容类型: %s", ct)
}

// SupportedFormats 返回支持的文件扩展名列表
func (c *markitdownClient) SupportedFormats() []string {
	return []string{
		".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt",
		".pdf", ".html", ".htm", ".txt", ".csv", ".json",
		".xml", ".epub", ".rst", ".org", ".mediawiki",
		".tsv", ".rss", ".atom", ".odt", ".ods", ".odp",
	}
}
