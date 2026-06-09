package external

import "io"

// MarkitdownClient 文件解析客户端接口
type MarkitdownClient interface {
	Convert(filePath string) (string, error)
	ConvertReader(filename string, reader io.Reader) (string, error)
	ConvertFromURL(url string) (string, error)
	// CheckURL 预检 URL 是否可抓取，返回 (可抓取, Content-Type, 错误)
	CheckURL(url string) (bool, string, error)
}
