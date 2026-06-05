package external

// MarkitdownClient 文件解析客户端接口
type MarkitdownClient interface {
	Convert(filePath string) (string, error)
	ConvertFromURL(url string) (string, error)
}
