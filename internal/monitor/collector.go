// Package monitor — AI 模型监控指标收集
//
// AiModelMetricsCollector：统计 AI 调用次数、成功率、Token 消耗、响应时间。
// 数据通过 Prometheus Counter/Histogram 暴露，Grafana 可视化展示。
// Package monitor — AI 模型调用指标收集器
//
// AiModelMetricsCollector 统计 AI 调用次数、成功率、Token 消耗、响应时间。
// 监控数据通过 middleware 包中的 Prometheus Counter/Histogram 暴露给 Grafana。
package monitor

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/yupi/yu-ai-code-mother-go/internal/middleware"
)

// AiModelMetricsCollector AI 模型指标收集器
// AiModelMetricsCollector AI 模型指标收集器 — 使用 atomic 保证并发安全
// 统计维度：总请求数、成功/失败数、Prompt/Completion Token 消耗、平均响应时间
type AiModelMetricsCollector struct {
	// 按模型统计请求计数
	requestCount  atomic.Int64
	successCount  atomic.Int64
	failCount     atomic.Int64
	// 总 Token 消耗
	totalPromptTokens     atomic.Int64
	totalCompletionTokens atomic.Int64
	// 总响应时间（微秒）
	totalResponseTimeUs atomic.Int64
	// 最后调用时间
	lastCallTime atomic.Value // time.Time
}

// NewAiModelMetricsCollector 创建收集器
func NewAiModelMetricsCollector() *AiModelMetricsCollector {
	c := &AiModelMetricsCollector{}
	c.lastCallTime.Store(time.Now())
	return c
}

// RecordRequest 记录 AI 请求
func (c *AiModelMetricsCollector) RecordRequest(userID, appID, model, status string) {
	c.requestCount.Add(1)
	c.lastCallTime.Store(time.Now())

	switch status {
	case "success":
		c.successCount.Add(1)
		middleware.RecordAIRequest(model, "success")
	case "failed":
		c.failCount.Add(1)
		middleware.RecordAIRequest(model, "failed")
	default:
		middleware.RecordAIRequest(model, "started")
	}
}

// RecordTokenUsage 记录 Token 使用
func (c *AiModelMetricsCollector) RecordTokenUsage(model string, promptTokens, completionTokens int) {
	c.totalPromptTokens.Add(int64(promptTokens))
	c.totalCompletionTokens.Add(int64(completionTokens))
	middleware.RecordAIToken(model, "prompt", float64(promptTokens))
	middleware.RecordAIToken(model, "completion", float64(completionTokens))
}

// RecordResponseTime 记录响应时间
func (c *AiModelMetricsCollector) RecordResponseTime(model string, duration time.Duration) {
	c.totalResponseTimeUs.Add(duration.Microseconds())
}

// GetStats 获取统计信息
// GetStats 获取当前统计快照（用于健康检查或调试）
func (c *AiModelMetricsCollector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"totalRequests":        c.requestCount.Load(),
		"successCount":         c.successCount.Load(),
		"failCount":            c.failCount.Load(),
		"totalPromptTokens":    c.totalPromptTokens.Load(),
		"totalCompletionTokens": c.totalCompletionTokens.Load(),
		"avgResponseTimeMs":    c.calcAvgResponseTime(),
		"lastCallTime":         c.lastCallTime.Load().(time.Time).Format(time.RFC3339),
	}
}

func (c *AiModelMetricsCollector) calcAvgResponseTime() float64 {
	total := c.requestCount.Load()
	if total == 0 {
		return 0
	}
	return float64(c.totalResponseTimeUs.Load()/total) / 1000.0 // 微秒 → 毫秒
}

// ==================== 监控上下文（同请求内传递） ====================

// MonitorContextHolder 监控上下文持有者
type MonitorContextHolder struct {
	UserID string
	AppID  string
}

// NewMonitorContextHolder 创建
func NewMonitorContextHolder() *MonitorContextHolder {
	return &MonitorContextHolder{}
}

// Set 设置上下文
func (h *MonitorContextHolder) Set(userID, appID string) {
	h.UserID = userID
	h.AppID = appID
}

// LogContext 打印监控信息
func (h *MonitorContextHolder) LogContext() {
	log.Printf("[Monitor] userID=%s, appID=%s", h.UserID, h.AppID)
}
