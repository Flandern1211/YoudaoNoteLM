package external

import (
	"os"
	"testing"

	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/logger"
)

func TestMain(m *testing.M) {
	// 初始化日志（测试环境用 console 输出）
	_ = logger.Init(&config.LogConfig{
		Level:    "error",
		Filename: "",
	})
	os.Exit(m.Run())
}
