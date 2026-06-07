package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	baseURL    = "http://localhost:8080/api/v1"
	jwtSecret  = "YouDaoNoteBookLM-API-Web-wangzhiwei-zhaojingchao-renfuhang"
	testUserID = uint(1)

	minioEndpoint  = "60.205.184.232:9003"
	minioAccessKey = "minioadmin"
	minioSecretKey = "12345678"
	minioBucket    = "youdaonotelm"
)

var client = &http.Client{Timeout: 600 * time.Second}

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
	fmt.Println("=== MinIO 音频文件「标准录音 10_16k.wav」转写测试 ===")
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

	// 创建 MinIO 客户端
	fmt.Println("[1/4] 连接 MinIO...")
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		fmt.Printf("❌ 创建 MinIO 客户端失败: %v\n", err)
		return
	}

	// 查找文件
	objectName := "uploads/标准录音 10_16k.wav"
	fmt.Printf("[2/4] 查找文件: %s\n", objectName)

	info, err := minioClient.StatObject(context.Background(), minioBucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		fmt.Printf("❌ 文件不存在: %v\n", err)
		// 尝试其他可能的路径
		fmt.Println("  尝试其他路径...")
		altPaths := []string{
			"uploads/标准录音10_16k.wav",
			"uploads/标准录音 10-16k.wav",
			"标准录音 10_16k.wav",
		}
		for _, altPath := range altPaths {
			info, err = minioClient.StatObject(context.Background(), minioBucket, altPath, minio.StatObjectOptions{})
			if err == nil {
				objectName = altPath
				fmt.Printf("  ✅ 找到文件: %s\n", objectName)
				break
			}
		}
		if err != nil {
			fmt.Println("  列出 bucket 中的 wav 文件:")
			for obj := range minioClient.ListObjects(context.Background(), minioBucket, minio.ListObjectsOptions{Recursive: true}) {
				if obj.Err != nil {
					continue
				}
				ext := filepath.Ext(obj.Key)
				if ext == ".wav" || ext == ".mp3" {
					fmt.Printf("    - %s (%.2f MB)\n", obj.Key, float64(obj.Size)/1024/1024)
				}
			}
			return
		}
	}
	fmt.Printf("✅ 找到文件: 大小 %d bytes (%.2f MB)\n", info.Size, float64(info.Size)/1024/1024)

	// 下载文件到临时目录
	fmt.Println("[3/4] 下载文件到临时目录...")
	tmpFile := filepath.Join(os.TempDir(), "标准录音10_16k.wav")
	obj, err := minioClient.GetObject(context.Background(), minioBucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		fmt.Printf("❌ 获取文件失败: %v\n", err)
		return
	}
	defer obj.Close()

	localFile, err := os.Create(tmpFile)
	if err != nil {
		fmt.Printf("❌ 创建临时文件失败: %v\n", err)
		return
	}
	defer localFile.Close()
	defer os.Remove(tmpFile)

	written, err := io.Copy(localFile, obj)
	if err != nil {
		fmt.Printf("❌ 保存文件失败: %v\n", err)
		return
	}
	localFile.Close()
	fmt.Printf("✅ 下载完成: %d bytes (%.2f MB)\n", written, float64(written)/1024/1024)

	// 上传并转写
	fmt.Println("\n[4/4] 上传音频文件并提交 ASR 转写...")
	localFile, _ = os.Open(tmpFile)
	defer localFile.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="标准录音10_16k.wav"`)
	header.Set("Content-Type", "audio/wav")

	part, _ := writer.CreatePart(header)
	io.Copy(part, localFile)
	writer.Close()

	req, _ := http.NewRequest("POST", baseURL+"/notebooks/1/import/audio/preview", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	fmt.Println("  等待 ASR 转写（较长音频可能需要 1-2 分钟）...")
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

	// 显示结果
	fmt.Println("\n========================================")
	fmt.Printf("转写耗时: %v\n", elapsed)
	fmt.Println("========================================")

	if result.Code == 0 {
		if dataMap, ok := result.Data.(map[string]interface{}); ok {
			content, _ := dataMap["content"].(string)
			fileName, _ := dataMap["file_name"].(string)
			previewID, _ := dataMap["preview_id"].(string)

			fmt.Printf("\n✅ 转写成功！\n")
			fmt.Printf("  文件名: %s\n", fileName)
			fmt.Printf("  预览ID: %s\n", previewID)
			fmt.Printf("  内容长度: %d 字符\n\n", len(content))
			fmt.Println("--- 转写内容 ---")
			fmt.Println(content)
			fmt.Println("----------------")
		}
	} else {
		fmt.Printf("\n❌ 转写失败: %s (code: %d)\n", result.Message, result.Code)
		fmt.Printf("  完整响应: %s\n", string(body))
	}
}
