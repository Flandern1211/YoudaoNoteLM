package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	baseURL    = "http://localhost:8080/api/v1"
	jwtSecret  = "YouDaoNoteBookLM-API-Web-wangzhiwei-zhaojingchao-renfuhang"
	testUserID = uint(1)
)

var client = &http.Client{Timeout: 300 * time.Second}

type CustomClaims struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	fmt.Println("=== 音频文件转写功能测试 ===")
	fmt.Println()

	// 生成 token
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
	token, _ := tokenObj.SignedString([]byte(jwtSecret))

	// 测试 1: 通过 preview 接口上传并转写
	fmt.Println("--- 测试 1: 上传并转写 nls-sample-16k.wav ---")
	testUploadAndTranscribe(token)

	fmt.Println()
	fmt.Println("--- 测试 2: 使用 MinIO 中已有的文件测试 ASR 服务 ---")
	testDirectASR(token)
}

func testUploadAndTranscribe(token string) {
	// 先检查本地是否有 nls-sample-16k.wav
	// 如果没有，从阿里云示例下载
	fmt.Println("  使用阿里云官方示例音频进行转写测试...")

	downloadURL := "https://gw.alipayobjects.com/os/bmw-prod/0574ee2e-f494-45a5-820f-63aee583045a.wav"
	fmt.Printf("  下载音频: %s\n", downloadURL)

	resp, err := client.Get(downloadURL)
	if err != nil {
		fmt.Printf("❌ 下载失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取音频失败: %v\n", err)
		return
	}
	fmt.Printf("  音频大小: %d bytes\n", len(audioData))

	// 构建 multipart 上传
	var buf bytes.Buffer
	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"nls-sample-16k.wav\"\r\n")
	buf.WriteString("Content-Type: audio/wav\r\n\r\n")
	buf.Write(audioData)
	buf.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	req, err := http.NewRequest("POST", baseURL+"/notebooks/1/import/audio/preview", &buf)
	if err != nil {
		fmt.Printf("❌ 创建请求失败: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.Header.Set("Authorization", "Bearer "+token)

	fmt.Println("  提交音频预览请求（等待 ASR 转写）...")
	start := time.Now()
	resp2, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp2.Body.Close()

	body, _ := io.ReadAll(resp2.Body)
	elapsed := time.Since(start)

	fmt.Printf("  耗时: %v\n", elapsed)
	fmt.Printf("  状态码: %d\n", resp2.StatusCode)
	fmt.Printf("  响应: %s\n", string(body))

	var result Response
	json.Unmarshal(body, &result)
	if result.Code != 0 {
		fmt.Printf("❌ 转写失败: %s\n", result.Message)
		return
	}

	if dataMap, ok := result.Data.(map[string]interface{}); ok {
		content, _ := dataMap["content"].(string)
		fmt.Printf("\n✅ 转写成功！\n")
		fmt.Printf("  识别结果: %s\n", content)
	}
}

func testDirectASR(token string) {
	// 这个测试需要先知道 MinIO 中的文件路径
	// 假设文件路径是 nls-sample-16k.wav 或 uploads/xxx.wav
	// 我们通过 preview 接口来测试，因为它会自动调用 ASR

	fmt.Println("  通过 preview 接口测试 ASR 服务...")
	fmt.Println("  （ASR 服务会从 MinIO 获取预签名 URL 并调用阿里云）")

	// 使用一个简单的音频来测试完整流程
	downloadURL := "https://gw.alipayobjects.com/os/bmw-prod/0574ee2e-f494-45a5-820f-63aee583045a.wav"

	resp, err := client.Get(downloadURL)
	if err != nil {
		fmt.Printf("❌ 下载失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	audioData, _ := io.ReadAll(resp.Body)

	// 构建 multipart
	var buf bytes.Buffer
	boundary := "----TestBoundary"
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"test-asr.wav\"\r\n")
	buf.WriteString("Content-Type: audio/wav\r\n\r\n")
	buf.Write(audioData)
	buf.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	req, _ := http.NewRequest("POST", baseURL+"/notebooks/1/import/audio/preview", &buf)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.Header.Set("Authorization", "Bearer "+token)

	fmt.Println("  提交请求...")
	start := time.Now()
	resp2, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp2.Body.Close()

	body, _ := io.ReadAll(resp2.Body)
	elapsed := time.Since(start)

	var result Response
	json.Unmarshal(body, &result)

	fmt.Printf("  耗时: %v\n", elapsed)
	if result.Code == 0 {
		if dataMap, ok := result.Data.(map[string]interface{}); ok {
			content, _ := dataMap["content"].(string)
			fmt.Printf("✅ ASR 转写成功！\n")
			fmt.Printf("  识别结果: %s\n", content)
		}
	} else {
		fmt.Printf("❌ ASR 转写失败: %s (code: %d)\n", result.Message, result.Code)
	}
}
