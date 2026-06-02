// Package redis — Redis 客户端初始化
//
// 使用 go-redis 连接 Redis，Ping 验证连接可用性。
// 用于 Session 存储、AI 对话记忆、缓存、分布式锁等场景。
package redis

import (
	"context"
	"fmt"
	"log"

	goredis "github.com/go-redis/redis/v8"

	"github.com/yupi/yu-ai-code-mother-go/internal/config"
)

// InitRedis 初始化 Redis 客户端 — 连接 + Ping 验证
func InitRedis(cfg *config.RedisConfig) (*goredis.Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	log.Println("[Redis] 连接成功")
	return rdb, nil
}
