package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"MindTrace/pkg/config"
)

var (
	redisClient *redis.Client
	redisOnce   sync.Once
)

// GetRedisClient 获取 Redis 客户端（单例）
func GetRedisClient() *redis.Client {
	return redisClient
}

// InitRedis 初始化 Redis 客户端
func InitRedis(cfg *config.RedisConfig) (*redis.Client, error) {
	var initErr error

	redisOnce.Do(func() {
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		})

		// 测试连接
		ctx := context.Background()
		if err := client.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("failed to connect to redis: %w", err)
			return
		}

		redisClient = client
	})

	if initErr != nil {
		return nil, initErr
	}

	return redisClient, nil
}

// MustInitRedis 初始化 Redis，失败则 panic
func MustInitRedis(cfg *config.RedisConfig) *redis.Client {
	client, err := InitRedis(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to init redis: %v", err))
	}
	return client
}

// CloseRedis 关闭 Redis 连接
func CloseRedis() error {
	if redisClient != nil {
		return redisClient.Close()
	}
	return nil
}
