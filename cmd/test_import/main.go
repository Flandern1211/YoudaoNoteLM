package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	baseURL    = "http://localhost:8080/api/v1"
	jwtSecret  = "YouDaoNoteBookLM-API-Web-wangzhiwei-zhaojingchao-renfuhang"
	testUserID = uint(1)
)

var (
	client = &http.Client{Timeout: 120 * time.Second}
	token  string
)

// CustomClaims JWT 自定义声明
type CustomClaims struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	fmt.Println("=== 导入模块接口完整测试 ===")
	fmt.Println()

	// 0. 生成测试 token（绕过验证码）
	if err := setupAuth(); err != nil {
		fmt.Printf("❌ 认证初始化失败: %v\n", err)
		return
	}

	// 测试用例列表
	tests := []struct {
		name string
		fn   func() error
	}{
		{"1. 文件导入 - 正常 .txt 文件", testImportFileTxt},
		{"2. 文件导入 - 正常 .md 文件", testImportFileMd},
		{"3. 文件导入 - 不支持的文件格式 (.jpg)", testImportFileUnsupported},
		{"4. 文件导入 - 无文件上传", testImportFileNoFile},
		{"5. 文件导入 - 无效笔记本ID", testImportFileInvalidNbId},
		{"6. 音频预览 - 正常 .wav 文件", testPreviewAudioWav},
		{"7. 音频预览 - 不支持的音频格式 (.flac)", testPreviewAudioUnsupported},
		{"8. 音频预览 - 无文件上传", testPreviewAudioNoFile},
		{"9. 音频确认 - 正常确认", testConfirmAudio},
		{"10. 音频确认 - 无效 preview_id", testConfirmAudioInvalidPreviewId},
		{"11. 任务查询 - 查询不存在的任务", testGetTaskNotFound},
	}

	passed := 0
	failed := 0
	for _, tt := range tests {
		fmt.Printf("--- %s ---\n", tt.name)
		if err := tt.fn(); err != nil {
			fmt.Printf("❌ 失败: %v\n\n", err)
			failed++
		} else {
			fmt.Println("✅ 通过")
			passed++
		}
	}

	fmt.Println("========================================")
	fmt.Printf("测试完成: %d 通过, %d 失败\n", passed, failed)
	if failed == 0 {
		fmt.Println("🎉 所有测试通过！")
	} else {
		fmt.Println("⚠️  部分测试失败，请检查日志")
	}
}

// setupAuth 生成测试 JWT token
func setupAuth() error {
	fmt.Println("--- 初始化认证 ---")

	// 直接生成 JWT token（用于测试，绕过验证码）
	claims := CustomClaims{
		UserID:    testUserID,
		Username:  "test_user",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "youdaonotelm",
		},
	}

	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	var err error
	token, err = tokenObj.SignedString([]byte(jwtSecret))
	if err != nil {
		return fmt.Errorf("生成 token 失败: %w", err)
	}

	fmt.Printf("  Token 生成成功: %s...\n\n", token[:30])
	return nil
}

// doRequest 带认证的请求
func doRequest(method, url string, body io.Reader, contentType string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, respBody, nil
}

// createTempFile 创建临时测试文件
func createTempFile(name, content string) (string, error) {
	tmpDir := os.TempDir()
	filePath := filepath.Join(tmpDir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	return filePath, err
}

// createMultipartFile 创建 multipart 上传
func createMultipartFile(filePath, fieldName string) (*bytes.Buffer, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 创建文件头
	ext := filepath.Ext(filePath)
	mimeType := "application/octet-stream"
	switch ext {
	case ".txt":
		mimeType = "text/plain"
	case ".md":
		mimeType = "text/markdown"
	case ".wav":
		mimeType = "audio/wav"
	case ".mp3":
		mimeType = "audio/mpeg"
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filepath.Base(filePath)))
	header.Set("Content-Type", mimeType)

	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, "", err
	}

	writer.Close()
	return &buf, writer.FormDataContentType(), nil
}

// ============ 测试函数 ============

// testImportFileTxt 测试导入 txt 文件
func testImportFileTxt() error {
	filePath, err := createTempFile("test_import.txt", "这是一段测试文本内容。\n用于验证文件导入功能。")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(filePath)

	buf, contentType, err := createMultipartFile(filePath, "file")
	if err != nil {
		return fmt.Errorf("创建 multipart 失败: %w", err)
	}

	resp, body, err := doRequest("POST", baseURL+"/notebooks/1/import/file", buf, contentType)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	if result.Code != 0 {
		return fmt.Errorf("业务错误码: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testImportFileMd 测试导入 md 文件
func testImportFileMd() error {
	filePath, err := createTempFile("test_import.md", "# 测试标题\n\n这是 markdown 内容。\n\n- 列表项1\n- 列表项2")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(filePath)

	buf, contentType, err := createMultipartFile(filePath, "file")
	if err != nil {
		return fmt.Errorf("创建 multipart 失败: %w", err)
	}

	resp, body, err := doRequest("POST", baseURL+"/notebooks/1/import/file", buf, contentType)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	if result.Code != 0 {
		return fmt.Errorf("业务错误码: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testImportFileUnsupported 测试导入不支持的文件格式
func testImportFileUnsupported() error {
	filePath, err := createTempFile("test.jpg", "fake image content")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(filePath)

	buf, contentType, err := createMultipartFile(filePath, "file")
	if err != nil {
		return fmt.Errorf("创建 multipart 失败: %w", err)
	}

	resp, body, err := doRequest("POST", baseURL+"/notebooks/1/import/file", buf, contentType)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	// 应该返回错误码 40001 (CodeUnsupportedFormat)
	if result.Code != 40001 {
		return fmt.Errorf("期望错误码 40001，实际: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testImportFileNoFile 测试不上传文件
func testImportFileNoFile() error {
	resp, body, err := doRequest("POST", baseURL+"/notebooks/1/import/file", nil, "")
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	// 应该返回错误码 400
	if result.Code != 400 {
		return fmt.Errorf("期望错误码 400，实际: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testImportFileInvalidNbId 测试无效的笔记本ID
func testImportFileInvalidNbId() error {
	filePath, err := createTempFile("test.txt", "test content")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(filePath)

	buf, contentType, err := createMultipartFile(filePath, "file")
	if err != nil {
		return fmt.Errorf("创建 multipart 失败: %w", err)
	}

	resp, body, err := doRequest("POST", baseURL+"/notebooks/abc/import/file", buf, contentType)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	// 应该返回错误码 400
	if result.Code != 400 {
		return fmt.Errorf("期望错误码 400，实际: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testPreviewAudioWav 测试音频预览（使用阿里云示例音频）
func testPreviewAudioWav() error {
	// 使用阿里云官方示例音频 URL 下载后上传
	downloadURL := "https://gw.alipayobjects.com/os/bmw-prod/0574ee2e-f494-45a5-820f-63aee583045a.wav"
	fmt.Printf("  下载示例音频: %s\n", downloadURL)

	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("下载音频失败: %w", err)
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取音频数据失败: %w", err)
	}

	// 保存到临时文件
	tmpFile := filepath.Join(os.TempDir(), "test_audio.wav")
	if err := os.WriteFile(tmpFile, audioData, 0644); err != nil {
		return fmt.Errorf("保存临时音频失败: %w", err)
	}
	defer os.Remove(tmpFile)

	buf, contentType, err := createMultipartFile(tmpFile, "file")
	if err != nil {
		return fmt.Errorf("创建 multipart 失败: %w", err)
	}

	fmt.Println("  提交音频预览请求（可能需要等待 ASR 转写）...")
	resp2, body, err := doRequest("POST", baseURL+"/notebooks/1/import/audio/preview", buf, contentType)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp2.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	if result.Code != 0 {
		return fmt.Errorf("业务错误码: %d, 消息: %s", result.Code, result.Message)
	}

	// 保存 preview_id 供后续测试使用
	if dataMap, ok := result.Data.(map[string]interface{}); ok {
		if previewID, ok := dataMap["preview_id"].(string); ok {
			fmt.Printf("  PreviewID: %s\n", previewID)
			// 写入文件供后续测试使用
			os.WriteFile(filepath.Join(os.TempDir(), "preview_id.txt"), []byte(previewID), 0644)
		}
	}

	return nil
}

// testPreviewAudioUnsupported 测试不支持的音频格式
func testPreviewAudioUnsupported() error {
	filePath, err := createTempFile("test.flac", "fake audio content")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(filePath)

	buf, contentType, err := createMultipartFile(filePath, "file")
	if err != nil {
		return fmt.Errorf("创建 multipart 失败: %w", err)
	}

	resp, body, err := doRequest("POST", baseURL+"/notebooks/1/import/audio/preview", buf, contentType)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	// 应该返回错误码 40001 (CodeUnsupportedFormat)
	if result.Code != 40001 {
		return fmt.Errorf("期望错误码 40001，实际: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testPreviewAudioNoFile 测试不上传音频文件
func testPreviewAudioNoFile() error {
	resp, body, err := doRequest("POST", baseURL+"/notebooks/1/import/audio/preview", nil, "")
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	// 应该返回错误码 400
	if result.Code != 400 {
		return fmt.Errorf("期望错误码 400，实际: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testConfirmAudio 测试确认音频导入
func testConfirmAudio() error {
	// 读取之前保存的 preview_id
	previewIDBytes, err := os.ReadFile(filepath.Join(os.TempDir(), "preview_id.txt"))
	if err != nil {
		fmt.Println("  跳过: 无 preview_id（前置测试可能失败）")
		return nil
	}
	previewID := strings.TrimSpace(string(previewIDBytes))
	fmt.Printf("  使用 PreviewID: %s\n", previewID)

	confirmBody := map[string]interface{}{
		"preview_id":  previewID,
		"notebook_id": 1,
		"content":     "这是编辑后的转写文本内容。",
	}
	data, _ := json.Marshal(confirmBody)

	resp, body, err := doRequest("POST", baseURL+"/import/audio/confirm", bytes.NewReader(data), "application/json")
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	if result.Code != 0 {
		return fmt.Errorf("业务错误码: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testConfirmAudioInvalidPreviewId 测试无效的 preview_id
func testConfirmAudioInvalidPreviewId() error {
	confirmBody := map[string]interface{}{
		"preview_id":  "invalid-preview-id-12345",
		"notebook_id": 1,
	}
	data, _ := json.Marshal(confirmBody)

	resp, body, err := doRequest("POST", baseURL+"/import/audio/confirm", bytes.NewReader(data), "application/json")
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	// 应该返回错误码 404
	if result.Code != 404 {
		return fmt.Errorf("期望错误码 404，实际: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}

// testGetTaskNotFound 测试查询不存在的任务
func testGetTaskNotFound() error {
	resp, body, err := doRequest("GET", baseURL+"/import/tasks/non-existent-task-id", nil, "")
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	fmt.Printf("  状态码: %d\n", resp.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	// 应该返回错误码 404
	if result.Code != 404 {
		return fmt.Errorf("期望错误码 404，实际: %d, 消息: %s", result.Code, result.Message)
	}
	return nil
}
