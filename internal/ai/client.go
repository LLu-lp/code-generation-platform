// Package ai AI 核心层 — 封装 OpenAI 兼容 API 的所有交互方式
//
// 本文件包含：
//   - AIClient：统一的 AI 客户端，支持 4 种调用模式
//     1. ChatCompletion           — 同步对话（非流式）
//     2. ChatCompletionStream     — SSE 流式对话
//     3. ChatCompletionWithTools  — 带 Function Calling 的流式工具调用
//     4. ChatCompletionStructured — JSON 结构化输出（用于质检/路由等）
//   - 流式消息类型定义：用于 Vue 工程模式下前端解析 AI 响应
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/yupi/yu-ai-code-mother-go/internal/config"
)

// ==================== 流式消息类型（用于 SSE 推送） ====================

// StreamMessage 流式消息基类，所有 SSE 推送的 JSON 消息都包含 type 字段
type StreamMessage struct {
	Type string `json:"type"` // ai_response / tool_request / tool_executed
}

// AiResponseMessage AI 文本响应块，包含 AI 生成的一个 token 片段
type AiResponseMessage struct {
	Type string `json:"type"`
	Data string `json:"data"` // AI 文本内容
}

// ToolRequestMessage 工具调用请求，AI 想要调用某个工具时的通知
type ToolRequestMessage struct {
	Type string `json:"type"`
	ID   string `json:"id"`   // 工具调用唯一标识
	Name string `json:"name"` // 工具名称：writeFile/readFile 等
	Args string `json:"args"` // JSON 格式的工具参数
}

// ToolExecutedMessage 工具执行结果，告知前端工具已执行完毕
type ToolExecutedMessage struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Result string `json:"result"` // 工具执行结果
}

// ==================== AI 模型客户端（统一封装 OpenAI 兼容 API） ====================

// AIClient 统一的 AI 客户端，封装 OpenAI 兼容协议的 4 种调用方式。
// 通过配置不同的 baseURL 和 apiKey，可对接 DeepSeek/Qwen/任意兼容服务。
type AIClient struct {
	config    *config.AIModelConfig // 模型配置（baseURL/apiKey/modelName/maxTokens）
	openaiCli *openai.Client        // OpenAI Go SDK 客户端实例
}

// NewAIClient 根据配置创建 AI 客户端
func NewAIClient(cfg *config.AIModelConfig) *AIClient {
	openaiCfg := openai.DefaultConfig(cfg.APIKey)
	openaiCfg.BaseURL = cfg.BaseURL
	return &AIClient{
		config:    cfg,
		openaiCli: openai.NewClientWithConfig(openaiCfg),
	}
}

// ChatCompletion 同步对话
func (c *AIClient) ChatCompletion(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	resp, err := c.openaiCli.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.config.ModelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		MaxTokens:   c.config.MaxTokens,
		Temperature: float32(c.config.Temperature),
	})
	if err != nil {
		return "", fmt.Errorf("AI 调用失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI 返回空响应")
	}
	return resp.Choices[0].Message.Content, nil
}

// ChatCompletionStream 流式对话 — 使用回调函数
func (c *AIClient) ChatCompletionStream(
	ctx context.Context,
	systemPrompt string,
	messages []openai.ChatCompletionMessage,
	onChunk func(string) error,
) error {
	allMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}
	allMessages = append(allMessages, messages...)

	stream, err := c.openaiCli.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       c.config.ModelName,
		Messages:    allMessages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: float32(c.config.Temperature),
		Stream:      true,
	})
	if err != nil {
		return fmt.Errorf("流式调用失败: %w", err)
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("接收流式响应失败: %w", err)
		}
		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta.Content
			if delta != "" {
				if err := onChunk(delta); err != nil {
					return err
				}
			}
		}
	}
}

// ChatCompletionWithTools ⭐ 带 Function Calling 的流式对话（Vue 工程模式核心）
// AI 可在生成过程中自主决定调用哪些工具（读写文件/查目录等），系统执行后把结果反馈给 AI。
// 最多 20 轮工具调用循环，防止死循环。每轮流程：
//   流式调用 AI → 收集文本响应 + 工具调用请求 → 无工具调用则结束 →
//   执行工具 → 将结果作为 Tool Message 加入对话 → 下一轮 AI 继续
func (c *AIClient) ChatCompletionWithTools(
	ctx context.Context,
	systemPrompt string,
	messages []openai.ChatCompletionMessage,
	tools []openai.Tool,
	onChunk func(string) error,
	onToolCall func(toolCall openai.ToolCall) (string, error),
) error {
	allMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}
	allMessages = append(allMessages, messages...)

	// 最多 20 轮工具调用循环，防止无限循环
	for iteration := 0; iteration < 20; iteration++ {
		// 1. 发起流式调用，携带工具定义
		stream, err := c.openaiCli.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
			Model:       c.config.ModelName,
			Messages:    allMessages,
			MaxTokens:   c.config.MaxTokens,
			Temperature: float32(c.config.Temperature),
			Tools:       tools, // ⭐ 告诉 AI 有哪些工具可用
			Stream:      true,
		})
		if err != nil {
			return fmt.Errorf("流式调用失败: %w", err)
		}

		var fullContent strings.Builder
		var toolCalls []openai.ToolCall
		var currentToolCall *openai.ToolCall

		for {
			response, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				stream.Close()
				return fmt.Errorf("接收流式响应失败: %w", err)
			}
			if len(response.Choices) > 0 {
				delta := response.Choices[0].Delta
				if delta.Content != "" {
					fullContent.WriteString(delta.Content)
					if err := onChunk(delta.Content); err != nil {
						stream.Close()
						return err
					}
				}
				// 处理工具调用
				for _, tc := range delta.ToolCalls {
					if tc.ID != "" {
						if currentToolCall != nil {
							toolCalls = append(toolCalls, *currentToolCall)
						}
						currentToolCall = &openai.ToolCall{
							ID:   tc.ID,
							Type: tc.Type,
							Function: openai.FunctionCall{
								Name:      tc.Function.Name,
								Arguments: "",
							},
						}
					}
					if currentToolCall != nil {
						currentToolCall.Function.Arguments += tc.Function.Arguments
					}
				}
			}
		}
		stream.Close()

		// 收集最后一个工具调用
		if currentToolCall != nil {
			toolCalls = append(toolCalls, *currentToolCall)
		}

		// 如果没有工具调用，则结束
		if len(toolCalls) == 0 {
			return nil
		}

		// 添加 AI 响应消息（含工具调用）
		aiMsg := openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   fullContent.String(),
			ToolCalls: toolCalls,
		}
		allMessages = append(allMessages, aiMsg)

		// 执行工具调用并收集结果
		for _, tc := range toolCalls {
			result, err := onToolCall(tc)
			errMsg := ""
			if err != nil {
				errMsg = fmt.Sprintf("工具调用错误: %v", err)
				log.Printf("[AI-Tool] %s 执行失败: %v", tc.Function.Name, err)
			} else {
				errMsg = result
			}
			allMessages = append(allMessages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    errMsg,
				ToolCallID: tc.ID,
			})
		}
	}
	return fmt.Errorf("超过最大工具调用轮次")
}

// ChatCompletionStructured 结构化输出（用于智能路由等）
func (c *AIClient) ChatCompletionStructured(ctx context.Context, systemPrompt, userMessage string, result interface{}) error {
	resp, err := c.openaiCli.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.config.ModelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		MaxTokens:      c.config.MaxTokens,
		Temperature:    float32(c.config.Temperature),
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
	})
	if err != nil {
		return fmt.Errorf("结构化输出调用失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("结构化输出返回空")
	}
	content := resp.Choices[0].Message.Content
	return json.Unmarshal([]byte(content), result)
}
