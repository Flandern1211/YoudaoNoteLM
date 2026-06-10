// internal/service/external/document_converter.go
package document

import "io"

// DocumentServiceType 文档转换服务类型常量
const DocumentServiceType = "document"

// DocumentConverter 文档转换 Provider 接口
// 将各种格式的文件/URL 转换为 Markd
type DocumentConverter interface {
	// Convert 本地文件转 Markdown
	Convert(filePath string) (string, error)
	// ConvertReader 通过 io.Reader 上传文件转 Markdown
	ConvertReader(filename string, reader io.Reader) (string, error)
	// ConvertFromURL 网页 URL 转 Markdown
	ConvertFromURL(url string) (string, error)
	// CheckURL 预检 URL 是否可抓取，返回 (可抓取, Content-Type, 错误)
	CheckURL(url string) (bool, string, error)
	// SupportedFormats 返回支持的文件扩展名列表（如 [".docx", ".pdf"]）
	SupportedFormats() []string
}
