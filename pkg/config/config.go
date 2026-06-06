package config

// Config 应用配置结构体
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Milvus   MilvusConfig   `yaml:"milvus"`
	JWT      JWTConfig      `yaml:"jwt"`
	MCP      MCPConfig      `yaml:"mcp"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	User      string `yaml:"user"`
	Password  string `yaml:"password"`
	DBName    string `yaml:"dbname"`
	Charset   string `yaml:"charset"`
	ParseTime bool   `yaml:"parse_time"`
	Loc       string `yaml:"loc"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// MilvusConfig Milvus 向量数据库配置
type MilvusConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret          string `yaml:"secret"`
	AccessTokenExp  string `yaml:"access_token_exp"`  // Access Token 过期时间，如 "15m"
	RefreshTokenExp string `yaml:"refresh_token_exp"` // Refresh Token 过期时间，如 "168h"（7天）
	Issuer          string `yaml:"issuer"`            // Token 签发者
}

// MCPConfig MCP Server 配置
type MCPConfig struct {
	MarkItDownURL string `yaml:"markitdown_url"`
}
