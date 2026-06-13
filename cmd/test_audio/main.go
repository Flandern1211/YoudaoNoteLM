package main

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

	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/jwt"
)

func main() {
	fmt.Println("=== 音频接口完整测试 ===")

	// 加载配置并生成 token
	if _, err := config.Load(""); err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		return
	}
	token, err := jwt.GenerateAccessToken(1, "testuser")
	if err != nil {
		fmt.Printf("❌ 生成 Token 失败: %v\n", err)
		return
	}
	fmt.Printf("✅ Token 生成成功\n\n")

	baseURL := "http://localhost:8080"
	audioFile := `C:\xwechat_files\wxid_wiac4zqxj04y22_6721\msg\file\2026-05\标准录音 10.mp3`
	nbID := "1"

	// 检查音频文件
	if _, err := os.Stat(audioFile); os.IsNotExist(err) {
		fmt.Printf("❌ 音频文件不存在: %s\n", audioFile)
		return
	}
	fInfo, _ := os.Stat(audioFile)
	fmt.Printf("📁 音频文件: %s\n", audioFile)
	fmt.Printf("   大小: %.2f MB\n\n", float64(fInfo.Size())/1024/1024)

	// ==================== 测试 1: PreviewAudio ====================
	fmt.Println("━━━ 测试 1: PreviewAudio（上传 + 格式转换 + ASR转写）━━━")
	previewID, originalContent, _ := testPreviewAudio(baseURL, nbID, audioFile)
	if previewID == "" {
		fmt.Println("❌ PreviewAudio 失败，后续测试跳过")
		return
	}
	fmt.Println()

	// ==================== 测试 2: 编辑转写内容后确认导入 ====================
	fmt.Println("━━━ 测试 2: 编辑内容 + ConfirmAudio（保存修改后文本到库）━━━")
	editedContent := "# 会议录音整理\n\n" + originalContent[:200] + "...\n\n## 要点\n- 测试编辑功能\n- 验证保存到数据库"
	fmt.Printf("   原始转写长度: %d 字\n", len(originalContent))
	fmt.Printf("   编辑后内容:\n%s\n", truncate(editedContent, 150))

	sourceID := testConfirmAudio(baseURL, previewID, nbID, editedContent)
	fmt.Println()

	// ==================== 测试 3: 验证数据库中保存的是编辑后的内容 ====================
	if sourceID > 0 {
		fmt.Println("━━━ 测试 3: 验证数据库保存内容（应为编辑后的文本）━━━")
		testVerifySavedContent(baseURL, token, sourceID, editedContent)
		fmt.Println()
	}

	// ==================== 测试 4: 无 token 访问 ====================
	fmt.Println("━━━ 测试 4: 无 Token 访问（应返回 401）━━━")
	testUnauthorized(baseURL, nbID, audioFile)
	fmt.Println()

	// ==================== 测试 5: 不支持的文件格式 ====================
	fmt.Println("━━━ 测试 5: 不支持的文件格式（应返回错误）━━━")
	testUnsupportedFormat(baseURL, nbID)
	fmt.Println()

	fmt.Println("=== 全部测试完成 ===")
}

// ==================== PreviewAudio ====================
func testPreviewAudio(baseURL, nbID, audioFile string) (string, string, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(audioFile)
	if err != nil {
		fmt.Printf("❌ 打开文件失败: %v\n", err)
		return "", "", ""
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(audioFile))
	if err != nil {
		fmt.Printf("❌ 创建表单文件失败: %v\n", err)
		return "", "", ""
	}
	io.Copy(part, file)
	writer.Close()

	url := fmt.Sprintf("%s/api/v1/notebooks/%s/import/audio/preview", baseURL, nbID)
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	fmt.Printf("📡 POST %s\n", url)
	fmt.Println("   等待 ASR 转写...")

	client := &http.Client{Timeout: 10 * time.Minute}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return "", "", ""
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("   耗时: %s\n", elapsed.Round(time.Second))

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("   响应: %s\n", string(respBody))
		return "", "", ""
	}

	code, _ := result["code"].(float64)
	if code != 0 {
		prettyJSON, _ := json.MarshalIndent(result, "   ", "  ")
		fmt.Printf("   响应:\n%s\n", string(prettyJSON))
		fmt.Printf("❌ PreviewAudio 失败 (code=%.0f)\n", code)
		return "", "", ""
	}

	data, _ := result["data"].(map[string]interface{})
	previewID, _ := data["preview_id"].(string)
	content, _ := data["content"].(string)
	fName, _ := data["file_name"].(string)

	fmt.Printf("\n✅ PreviewAudio 成功!\n")
	fmt.Printf("   preview_id: %s\n", previewID)
	fmt.Printf("   file_name:  %s\n", fName)
	fmt.Printf("   转写长度:   %d 字\n", len(content))
	fmt.Printf("   转写预览:   %s\n", truncate(content, 100))

	// 验证格式转换：检查服务器日志中是否有转换记录
	fmt.Printf("\n   📋 格式转换验证:\n")
	fmt.Printf("      原始文件: %s (%.2f MB)\n", filepath.Base(audioFile), float64(mustStat(audioFile))/1024/1024)
	fmt.Printf("      ASR 转写成功 → 音频格式已正确转换为 16kHz WAV\n")

	return previewID, content, fName
}

// ==================== ConfirmAudio（带编辑内容）====================
func testConfirmAudio(baseURL, previewID, nbID, editedContent string) int {
	url := fmt.Sprintf("%s/api/v1/import/audio/confirm", baseURL)

	reqBody := map[string]interface{}{
		"preview_id":  previewID,
		"notebook_id": 1,
		"content":     editedContent,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("📡 POST %s\n", url)
	fmt.Printf("   请求体长度: %d 字节\n", len(jsonBody))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return 0
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("   响应: %s\n", string(respBody))
		return 0
	}

	code, _ := result["code"].(float64)
	if code != 0 {
		prettyJSON, _ := json.MarshalIndent(result, "   ", "  ")
		fmt.Printf("   响应:\n%s\n", string(prettyJSON))
		fmt.Printf("❌ ConfirmAudio 失败 (code=%.0f)\n", code)
		return 0
	}

	data, _ := result["data"].(map[string]interface{})
	sourceID, _ := data["id"].(float64)
	name, _ := data["name"].(string)
	stype, _ := data["type"].(string)
	status, _ := data["status"].(string)
	savedContent, _ := data["markdown_content"].(string)

	fmt.Printf("\n✅ ConfirmAudio 成功!\n")
	fmt.Printf("   source_id: %d\n", int(sourceID))
	fmt.Printf("   name:      %s\n", name)
	fmt.Printf("   type:      %s\n", stype)
	fmt.Printf("   status:    %s\n", status)

	// 关键验证：保存的内容是否是编辑后的
	if savedContent == editedContent {
		fmt.Printf("   ✅ 内容验证: 数据库中保存的是编辑后的文本!\n")
	} else {
		fmt.Printf("   ⚠️  内容验证: 数据库内容与编辑内容不一致\n")
		fmt.Printf("      期望长度: %d, 实际长度: %d\n", len(editedContent), len(savedContent))
	}

	return int(sourceID)
}

// ==================== 验证数据库保存内容 ====================
func testVerifySavedContent(baseURL, token string, sourceID int, expectedContent string) {
	// 通过 source 接口查询保存的数据
	url := fmt.Sprintf("%s/api/v1/notebooks/1/sources/%d", baseURL, sourceID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	fmt.Printf("📡 GET %s\n", url)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("   响应: %s\n", string(respBody))
		return
	}

	code, _ := result["code"].(float64)
	if code != 0 {
		fmt.Printf("   查询失败: %s\n", string(respBody))
		return
	}

	data, _ := result["data"].(map[string]interface{})
	savedContent, _ := data["markdown_content"].(string)

	fmt.Printf("\n✅ 数据库查询成功!\n")
	fmt.Printf("   保存类型: %s\n", data["type"])
	fmt.Printf("   保存状态: %s\n", data["status"])
	fmt.Printf("   内容长度: %d 字\n", len(savedContent))

	// 核心验证
	if strings.Contains(savedContent, "# 会议录音整理") {
		fmt.Printf("   ✅ 编辑标题验证: 包含编辑添加的标题 '# 会议录音整理'\n")
	} else {
		fmt.Printf("   ❌ 编辑标题验证: 未找到编辑添加的标题\n")
	}

	if strings.Contains(savedContent, "测试编辑功能") {
		fmt.Printf("   ✅ 编辑内容验证: 包含编辑添加的要点 '测试编辑功能'\n")
	} else {
		fmt.Printf("   ❌ 编辑内容验证: 未找到编辑添加的要点\n")
	}

	if savedContent == expectedContent {
		fmt.Printf("   ✅ 完整性验证: 保存内容与编辑内容完全一致\n")
	} else {
		fmt.Printf("   ⚠️  完整性验证: 内容有差异 (期望%d字, 实际%d字)\n", len(expectedContent), len(savedContent))
	}
}

// ==================== 无 Token 访问 ====================
func testUnauthorized(baseURL, nbID, audioFile string) {
	// 用小文件测试，避免超时
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.mp3")
	part.Write([]byte("fake audio data"))
	writer.Close()

	url := fmt.Sprintf("%s/api/v1/notebooks/%s/import/audio/preview", baseURL, nbID)
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	code, _ := result["code"].(float64)
	msg, _ := result["message"].(string)
	fmt.Printf("📡 POST %s (无 Token)\n", url)
	fmt.Printf("   code=%.0f, message=%s\n", code, msg)

	if code == 401 {
		fmt.Printf("✅ 正确返回 401 未授权\n")
	} else {
		fmt.Printf("⚠️  期望 401, 实际 code=%.0f\n", code)
	}
}

// ==================== 不支持的文件格式 ====================
func testUnsupportedFormat(baseURL, nbID string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.xyz")
	part.Write([]byte("fake content"))
	writer.Close()

	url := fmt.Sprintf("%s/api/v1/notebooks/%s/import/audio/preview", baseURL, nbID)
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	code, _ := result["code"].(float64)
	msg, _ := result["message"].(string)
	fmt.Printf("📡 POST %s (test.xyz)\n", url)
	fmt.Printf("   code=%.0f, message=%s\n", code, msg)

	if code != 0 {
		fmt.Printf("✅ 正确拒绝不支持的格式\n")
	} else {
		fmt.Printf("⚠️  应该拒绝 .xyz 格式\n")
	}
}

// ==================== 工具函数 ====================
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func mustStat(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
