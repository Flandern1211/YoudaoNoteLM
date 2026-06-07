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
	fmt.Println("=== 搜索模块接口测试 ===")
	fmt.Println()

	// 生成测试 token
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
	token, err := tokenObj.SignedString([]byte(jwtSecret))
	if err != nil {
		fmt.Printf("❌ 生成 token 失败: %v\n", err)
		return
	}

	client := &http.Client{Timeout: 180 * time.Second}

	// 测试搜索 API
	fmt.Println("--- 1. 测试搜索 API ---")
	searchBody := map[string]interface{}{
		"query": "Go语言1.22版本新特性",
	}
	data, _ := json.Marshal(searchBody)

	req, _ := http.NewRequest("POST", baseURL+"/notebooks/1/search", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("  请求: POST %s/notebooks/1/search\n", baseURL)
	fmt.Printf("  Body: %s\n", string(data))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  ❌ 请求失败: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  状态码: %d\n", resp.StatusCode)

		var result Response
		json.Unmarshal(body, &result)
		fmt.Printf("  响应: %s\n", string(body))

		// 检查搜索结果
		if resultMap, ok := result.Data.(map[string]interface{}); ok {
			if results, ok := resultMap["results"].([]interface{}); ok {
				fmt.Printf("\n  搜索结果数量: %d\n", len(results))
				for i, r := range results {
					if item, ok := r.(map[string]interface{}); ok {
						fmt.Printf("  %d. %s\n", i+1, item["title"])
						fmt.Printf("     URL: %s\n", item["url"])
						fmt.Printf("     评分: %.1f\n", item["score"])
						fmt.Printf("     理由: %s\n", item["reason"])
					}
				}
			}
		}
	}

	// 测试 URL 导入 API
	fmt.Println("\n--- 2. 测试 URL 导入 API ---")
	urlImportBody := map[string]interface{}{
		"url": "https://go.dev/blog/go1.22",
	}
	data, _ = json.Marshal(urlImportBody)

	req, _ = http.NewRequest("POST", baseURL+"/notebooks/1/search/url", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("  请求: POST %s/notebooks/1/search/url\n", baseURL)
	fmt.Printf("  Body: %s\n", string(data))

	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("  ❌ 请求失败: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  状态码: %d\n", resp.StatusCode)
		fmt.Printf("  响应: %s\n", string(body))
	}

	// 测试批量导入 API
	fmt.Println("\n--- 3. 测试批量导入 API ---")
	batchImportBody := map[string]interface{}{
		"urls": []string{
			"https://go.dev/blog/go1.22",
			"https://go.dev/doc/go1.22",
		},
	}
	data, _ = json.Marshal(batchImportBody)

	req, _ = http.NewRequest("POST", baseURL+"/notebooks/1/search/import", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("  请求: POST %s/notebooks/1/search/import\n", baseURL)
	fmt.Printf("  Body: %s\n", string(data))

	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("  ❌ 请求失败: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  状态码: %d\n", resp.StatusCode)
		fmt.Printf("  响应: %s\n", string(body))
	}

	fmt.Println("\n=== 测试完成 ===")
}
