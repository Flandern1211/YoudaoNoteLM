package external

import (
	"bytes"
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

type asrService struct {
	apiURL string
	apiKey string
	client *http.Client
}

// NewASRService 创建 ASR 语音转文本服务
func NewASRService(apiURL, apiKey string) ASRService {
	return &asrService{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 300 * time.Second},
	}
}

// Transcribe 音频文件转文本
func (s *asrService) Transcribe(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("创建表单失败: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭writer失败: %w", err)
	}

	req, err := http.NewRequest("POST", s.apiURL+"/transcribe", body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ASR请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ASR返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析ASR结果失败: %w", err)
	}

	logger.Info("ASR转写成功", zap.String("file", filePath))
	return result.Text, nil
}
