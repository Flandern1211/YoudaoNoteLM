package external

import "mime/multipart"

// FileStorage 文件存储接口
type FileStorage interface {
	Upload(file *multipart.FileHeader) (string, error)
	UploadBytes(objectName string, data []byte, contentType string) error
	Download(filePath string) ([]byte, error)
	Delete(filePath string) error
}
