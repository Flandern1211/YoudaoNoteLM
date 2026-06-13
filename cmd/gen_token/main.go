package main

import (
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/jwt"
	"fmt"
	"os"
)

func main() {
	// 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}
	config.SetForTest(cfg)

	// 生成 token
	token, err := jwt.GenerateAccessToken(2, "3180066912")
	if err != nil {
		fmt.Printf("生成 token 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Access Token: %s\n", token)
}