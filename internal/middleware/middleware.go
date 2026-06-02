// Package middleware — 中间件层
//
// 包含所有 HTTP 中间件：Recovery（panic恢复）、Logger（请求日志）、Metrics（Prometheus指标）、
// Auth（Redis Session认证）、RateLimit（Token Bucket限流）、CORS（跨域）。
// 同时定义 Prometheus 自定义指标：http_requests_total、http_request_duration_seconds、
// ai_requests_total、ai_tokens_total。
package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

// ==================== Prometheus 自定义监控指标 ====================
// 4 个核心指标：
//   http_requests_total        — HTTP 请求总数（按 method/path/status 分类）
//   http_request_duration_seconds — 请求延迟分布（P50/P90/P99）
//   ai_requests_total          — AI 调用次数（按 model/status 分类，用于成本追踪）
//   ai_tokens_total            — AI Token 消耗（按 model/prompt|completion 分类）
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	aiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_requests_total",
			Help: "Total number of AI requests",
		},
		[]string{"model", "status"},
	)
	aiTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_tokens_total",
			Help: "Total number of AI tokens used",
		},
		[]string{"model", "type"},
	)
)

// init 在包初始化时向 Prometheus 注册所有自定义指标
func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, aiRequestsTotal, aiTokensTotal)
}

// ==================== 基础中间件 ====================

// RecoveryMiddleware panic 恢复中间件 — 捕获任何未处理的 panic，返回 500 而非崩溃
// 放置在中间件链的最外层（第一个注册），确保所有 panic 都能被捕获
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[Recovery] panic: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    50000,
					"message": "系统内部错误",
				})
			}
		}()
		c.Next()
	}
}

// LoggerMiddleware 请求日志中间件 — 记录每个 HTTP 请求的方法、路径、状态码、耗时
// 额外功能：记录请求体（截断至 2000 字符），对 AI 对话调试非常有用
// 注意：需要先读取 Body 再重新放回（io.NopCloser），否则后续处理器无法读取
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// 记录请求体（对于 AI 对话很有用）
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		log.Printf("[HTTP] %s %s %d %v",
			c.Request.Method, path, statusCode, duration)

		// 记录 AI 对话请求体（截断）
		if len(bodyBytes) > 0 && len(bodyBytes) < 2000 {
			log.Printf("[HTTP-Body] %s", string(bodyBytes))
		}
	}
}

// MetricsMiddleware Prometheus 指标采集中间件 — 自动采集每个请求的计数和延迟
// 使用 c.FullPath() 获取路由模板（如 /app/:id）而非实际路径，避免指标基数爆炸
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		statusCode := fmt.Sprintf("%d", c.Writer.Status())
		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, statusCode).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// PrometheusHandler 暴露 /metrics 端点给 Prometheus 拉取指标数据
func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// ==================== Auth 认证中间件 ====================

// AuthMiddleware 基于 Redis Session 的认证中间件
// 流程：Cookie 取 token → Redis 查 key "session:{token}" → 将用户 JSON 注入 Context
// 参数 required=true 时强制登录，false 时可选登录（未登录不阻断）
func AuthMiddleware(rdb *redis.Client, required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil || token == "" {
			if required {
				c.AbortWithStatusJSON(http.StatusOK, gin.H{
					"code":    40100,
					"message": "未登录",
				})
				return
			}
			c.Next()
			return
		}

		key := fmt.Sprintf("session:%s", token)
		data, err := rdb.Get(c.Request.Context(), key).Result()
		if err != nil {
			if required {
				c.AbortWithStatusJSON(http.StatusOK, gin.H{
					"code":    40100,
					"message": "登录已过期",
				})
				return
			}
			c.Next()
			return
		}

		// 将用户信息注入 context
		c.Set("sessionData", data)
		c.Next()
	}
}

// ==================== 限流中间件（Token Bucket 算法） ====================

// RateLimiterStore 限流器存储 — 为每个 key（用户/IP）维护独立的 Token Bucket
// 内含后台 goroutine 每 5 分钟清理 10 分钟未使用的过期 limiters，防止内存泄漏
type RateLimiterStore struct {
	limiters map[string]*rateLimiterEntry
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter // golang.org/x/time/rate 的 Token Bucket 实现
	lastSeen time.Time      // 最后使用时间，用于清理判断
}

// NewRateLimiterStore 创建限流存储并启动后台清理 goroutine
func NewRateLimiterStore() *RateLimiterStore {
	s := &RateLimiterStore{
		limiters: make(map[string]*rateLimiterEntry),
	}
	// 每 5 分钟清理过期限流器
	go s.cleanupLoop(5 * time.Minute)
	return s
}

// getLimiter 获取或创建指定 key 的限流器（按需创建，惰性初始化）
func (s *RateLimiterStore) getLimiter(key string, r rate.Limit, burst int) *rate.Limiter {
	if entry, ok := s.limiters[key]; ok {
		entry.lastSeen = time.Now()
		return entry.limiter
	}
	limiter := rate.NewLimiter(r, burst)
	s.limiters[key] = &rateLimiterEntry{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

// cleanupLoop 后台清理循环 — 每隔 interval 时间检查并删除过期的限流器
// 过期判定：lastSeen 距现在超过 2 个 interval
func (s *RateLimiterStore) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now()
		for key, entry := range s.limiters {
			if now.Sub(entry.lastSeen) > interval*2 {
				delete(s.limiters, key)
			}
		}
	}
}

// RateLimitMiddleware 限流中间件 — Token Bucket 算法，按 IP 限流
// 参数 ratePerSec: 每秒允许的请求数（如 5 表示 QPS=5）
// 参数 burst: 突发容量（允许瞬间超过 ratePerSec 的请求数）
func RateLimitMiddleware(store *RateLimiterStore, keyPrefix string, ratePerSec float64, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 默认按 IP 限流
		key := keyPrefix + ":" + c.ClientIP()

		limiter := store.getLimiter(key, rate.Limit(ratePerSec), burst)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code":    42900,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		c.Next()
	}
}

// ==================== CORS 跨域中间件 ====================

// CORSMiddleware 跨域中间件 — 允许前端（不同端口/域名）访问 API
// 处理 OPTIONS 预检请求直接返回 204，避免实际请求被浏览器拦截
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ==================== AI 监控埋点（供 monitor 包调用） ====================

// RecordAIRequest 记录一次 AI 请求（按模型 + 状态埋点）
func RecordAIRequest(model, status string) {
	aiRequestsTotal.WithLabelValues(model, status).Inc()
}

// RecordAIToken 记录 AI Token 消耗（按模型 + prompt|completion 分类）
func RecordAIToken(model, tokenType string, count float64) {
	aiTokensTotal.WithLabelValues(model, tokenType).Add(count)
}
