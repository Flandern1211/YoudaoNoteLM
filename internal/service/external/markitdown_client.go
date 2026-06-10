package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

const (
	defaultTimeout     = 30 * time.Second // 默认超时
	fileConvertTimeout = 30 * time.Second // 文件转换超时
	urlConvertTimeout  = 20 * time.Second // URL 转换超时
)

type markitdownClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMarkitdownClient 创建 MarkItDown HTTP 客户端
func NewMarkitdownClient(baseURL string) MarkitdownClient {
	return &markitdownClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// Convert 本地文件转 Markdown（上传文件到 MarkItDown 服务）
func (c *markitdownClient) Convert(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), fileConvertTimeout)
	defer cancel()

	return c.ConvertReaderWithContext(ctx, filepath.Base(filePath), file)
}

// ConvertReader 通过 io.Reader 上传文件转 Markdown
func (c *markitdownClient) ConvertReader(filename string, reader io.Reader) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fileConvertTimeout)
	defer cancel()

	return c.ConvertReaderWithContext(ctx, filename, reader)
}

// ConvertReaderWithContext 通过 io.Reader 上传文件转 Markdown（带 context）
func (c *markitdownClient) ConvertReaderWithContext(ctx context.Context, filename string, reader io.Reader) (string, error) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/convert", body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("请求MarkItDown超时（%v）", fileConvertTimeout)
		}
		return "", fmt.Errorf("请求MarkItDown失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestTimeout {
		return "", fmt.Errorf("MarkItDown 服务端转换超时")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("MarkItDown返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// MarkItDown Python 服务返回 {"filename": "...", "markdown": "..."}
	var result struct {
		Markdown string `json:"markdown"`
		Cached   bool   `json:"cached"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// 降级：返回原始响应
		return string(respBody), nil
	}

	logger.Info("MarkItDown转换成功", zap.String("file", filename), zap.Bool("cached", result.Cached))
	return result.Markdown, nil
}

// ConvertFromURL 网页 URL 转 Markdown
func (c *markitdownClient) ConvertFromURL(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), urlConvertTimeout)
	defer cancel()

	return c.ConvertFromURLWithContext(ctx, url)
}

// ConvertFromURLWithContext 网页 URL 转 Markdown（带 context）
func (c *markitdownClient) ConvertFromURLWithContext(ctx context.Context, url string) (string, error) {
	// 在传入的 ctx 基础上叠加超时控制，确保单个请求不会无限等待
	ctx, cancel := context.WithTimeout(ctx, urlConvertTimeout)
	defer cancel()

	// MarkItDown 服务的 /convert_url 使用 Form 表单
	formBody := &bytes.Buffer{}
	writer := multipart.NewWriter(formBody)
	_ = writer.WriteField("url", url)
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭writer失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/convert_url", formBody)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("请求MarkItDown URL转换超时（%v）", urlConvertTimeout)
		}
		return "", fmt.Errorf("请求MarkItDown URL转换失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestTimeout {
		return "", fmt.Errorf("MarkItDown 服务端转换超时")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
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
		Cached   bool   `json:"cached"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	if result.Markdown == "" && result.Message != "" {
		logger.Warn("MarkItDown URL转换无内容", zap.String("url", url), zap.String("message", result.Message))
		return "", fmt.Errorf("%s", result.Message)
	}

	logger.Info("MarkItDown URL转换成功", zap.String("url", url), zap.Bool("cached", result.Cached))
	return result.Markdown, nil
}
