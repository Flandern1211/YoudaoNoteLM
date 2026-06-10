// internal/service/external/document/providers.go
package document

import (
	"YoudaoNoteLm/internal/service/external"
)

func init() {
	r := external.GetGlobalRegistry()

	// MarkItDown（HTTP 服务）
	r.Register(DocumentServiceType, "markitdown", "MarkItDown（HTTP 服务）",
		[]string{"api_url"}, nil,
		func(cfg *external.ServiceConfig) (interface{}, error) {
			return NewMarkitdownClient(cfg.APIURL), nil
		}, map[string]string{
			"api_url": "MarkItDown 服务地址",
		})
}
