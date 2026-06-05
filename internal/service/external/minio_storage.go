package external

import (
	"fmt"
	"mime/multipart"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type minioStorage struct {
	bucket    string
	endpoint  string
	accessKey string
	secretKey string
}

// NewMinIOStorage 创建 MinIO 存储（预留实现，需安装 minio SDK）
func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string) FileStorage {
	return &minioStorage{
		bucket:    bucket,
		endpoint:  endpoint,
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

// Upload 上传文件（当前为占位实现，需集成 MinIO SDK）
func (s *minioStorage) Upload(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	filePath := fmt.Sprintf("uploads/%s", file.Filename)
	// TODO: 实现实际 MinIO 上传
	logger.Info("文件上传（本地存储模式）", zap.String("path", filePath))
	return filePath, nil
}

// Download 下载文件（当前为占位实现）
func (s *minioStorage) Download(filePath string) ([]byte, error) {
	// TODO: 实现实际 MinIO 下载
	return nil, fmt.Errorf("MinIO下载未实现: %s", filePath)
}

// Delete 删除文件（当前为占位实现）
func (s *minioStorage) Delete(filePath string) error {
	// TODO: 实现实际 MinIO 删除
	logger.Info("文件删除（本地存储模式）", zap.String("path", filePath))
	return nil
}
