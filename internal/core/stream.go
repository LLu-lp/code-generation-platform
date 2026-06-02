// Package core — SSE 流处理器
//
// 处理 AI 流式输出，支持两种模式：
// SimpleTextProcessor：简单透传（HTML/多文件模式）
// JSONMessageProcessor：解析 JSON 格式的流消息（Vue工程模式，含工具调用可视化）
package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yupi/yu-ai-code-mother-go/internal/model/enums"
)

// ==================== 流式消息类型 ====================

// StreamMessageType 流消息类型常量
const (
	MsgTypeAIResponse   = "ai_response"
	MsgTypeToolRequest  = "tool_request"
	MsgTypeToolExecuted = "tool_executed"
)

// StreamMessage 流式消息基类
type StreamMessage struct {
	Type string `json:"type"`
}

// AIResponseMsg AI 文本响应块
type AIResponseMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// ToolRequestMsg 工具调用请求
type ToolRequestMsg struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Args string `json:"args"`
}

// ToolExecutedMsg 工具执行结果
type ToolExecutedMsg struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Result string `json:"result"`
}

// ==================== 流处理器 ====================

// StreamProcessor 流处理器接口
type StreamProcessor interface {
	// ProcessChunk 处理每个流数据块，返回要发给前端的字符串
	ProcessChunk(chunk string) string
	// GetCollectedContent 获取收集到的完整内容（用于保存到对话历史）
	GetCollectedContent() string
	// GetCollectedToolCalls 获取收集到的工具调用记录
	GetCollectedToolCalls() []string
}

// ==================== 简单文本流处理器（HTML / 多文件模式） ====================

// SimpleTextProcessor 简单文本流处理器
type SimpleTextProcessor struct {
	builder          strings.Builder
	collectedToolCalls []string
}

// NewSimpleTextProcessor 创建简单文本处理器
func NewSimpleTextProcessor() *SimpleTextProcessor {
	return &SimpleTextProcessor{}
}

func (p *SimpleTextProcessor) ProcessChunk(chunk string) string {
	p.builder.WriteString(chunk)
	return chunk // 直接透传
}

func (p *SimpleTextProcessor) GetCollectedContent() string {
	return p.builder.String()
}

func (p *SimpleTextProcessor) GetCollectedToolCalls() []string {
	return p.collectedToolCalls
}

// ==================== JSON 消息流处理器（Vue 工程模式） ====================

// JSONMessageProcessor JSON 消息流处理器
// 处理 VUE_PROJECT 类型的复杂流式响应，包含工具调用信息
type JSONMessageProcessor struct {
	builder           strings.Builder
	collectedToolCalls []string
	toolManager       interface{} // *ai.ToolManager，接口引用
}

// NewJSONMessageProcessor 创建 JSON 消息处理器
func NewJSONMessageProcessor(tm interface{}) *JSONMessageProcessor {
	return &JSONMessageProcessor{
		toolManager: tm,
	}
}

// ProcessChunk 处理 JSON 格式的流数据块
// 可能包含 AI 文本响应 / 工具请求 / 工具执行结果
func (p *JSONMessageProcessor) ProcessChunk(chunk string) string {
	var msg StreamMessage
	if err := json.Unmarshal([]byte(chunk), &msg); err != nil {
		// 非 JSON 格式，可能是纯文本，透传
		p.builder.WriteString(chunk)
		return chunk
	}

	switch msg.Type {
	case MsgTypeAIResponse:
		var aiMsg AIResponseMsg
		json.Unmarshal([]byte(chunk), &aiMsg)
		p.builder.WriteString(aiMsg.Data)
		return aiMsg.Data

	case MsgTypeToolRequest:
		var trMsg ToolRequestMsg
		json.Unmarshal([]byte(chunk), &trMsg)
		record := fmt.Sprintf("[工具调用] %s(%s)", trMsg.Name, trMsg.Args)
		p.collectedToolCalls = append(p.collectedToolCalls, record)
		p.builder.WriteString("\n\n" + record + "\n\n")
		return fmt.Sprintf("\n\n%s\n\n", record)

	case MsgTypeToolExecuted:
		var teMsg ToolExecutedMsg
		json.Unmarshal([]byte(chunk), &teMsg)
		record := fmt.Sprintf("[工具执行] %s → %s", teMsg.Name, truncate(teMsg.Result, 200))
		p.collectedToolCalls = append(p.collectedToolCalls, record)
		return "" // 工具执行结果不需要展示给用户

	default:
		p.builder.WriteString(chunk)
		return chunk
	}
}

func (p *JSONMessageProcessor) GetCollectedContent() string {
	return p.builder.String()
}

func (p *JSONMessageProcessor) GetCollectedToolCalls() []string {
	return p.collectedToolCalls
}

// ==================== 流处理器工厂 ====================

// NewStreamProcessor 根据代码生成类型创建合适的流处理器
func NewStreamProcessor(codeGenType string, toolManager interface{}) StreamProcessor {
	switch codeGenType {
	case enums.CodeGenTypeVueProject.Value:
		return NewJSONMessageProcessor(toolManager)
	default:
		return NewSimpleTextProcessor()
	}
}

// ==================== 辅助函数 ====================

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
