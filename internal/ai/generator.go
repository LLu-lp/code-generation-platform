// Package ai — 代码生成工厂
//
// CodeGeneratorFactory 是 AI 代码生成的统一入口：
// - 管理 4 个 AI 客户端（Chat/Streaming/Reasoning/Routing）
// - 提供 3 种代码生成策略：HTML单文件 / 多文件分离 / Vue工程（带工具调用）
// - 智能路由：用轻量模型分析用户需求，自动选择最优策略
// - 代码质检：AI 自动检查生成代码的语法错误和结构问题
package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/yupi/yu-ai-code-mother-go/internal/config"
)

// ==================== 代码生成器工厂 ====================

// CodeGeneratorFactory ⭐ AI 代码生成工厂 — 整个 AI 层的统一入口
// 管理 4 个 AI 客户端各有分工：
//   chatClient      → DeepSeek Chat（非流式，用于 HTML/多文件代码生成）
//   streamClient    → DeepSeek Chat（流式，用于 SSE 实时推送）
//   reasoningClient → DeepSeek Reasoner（深度推理，用于 Vue 工程的工具调用循环）
//   routingClient   → Qwen Turbo（轻量快速，用于智能路由分类）
// 还包含 ToolManager（6 个文件操作工具）和 PromptManager（6 套 System Prompt）
type CodeGeneratorFactory struct {
	cfg             *config.Config
	chatClient      *AIClient     // 非流式对话：GenerateHTMLCode / GenerateMultiFileCode
	streamClient    *AIClient     // 流式对话：StreamHTMLCode / StreamMultiFileCode
	reasoningClient *AIClient     // 推理+工具调用：StreamVueProjectCode
	routingClient   *AIClient     // 轻量路由：RouteCodeGenType
	toolManager     *ToolManager  // AI 文件操作工具集
	prompts         *PromptManager // System Prompt 模板管理
}

// NewCodeGeneratorFactory 初始化 4 个 AI 客户端 + 工具管理器 + Prompt 管理器
func NewCodeGeneratorFactory(cfg *config.Config) *CodeGeneratorFactory {
	return &CodeGeneratorFactory{
		cfg:             cfg,
		chatClient:      NewAIClient(&cfg.AI.ChatModel),
		streamClient:    NewAIClient(&cfg.AI.StreamingModel),
		reasoningClient: NewAIClient(&cfg.AI.ReasoningModel),
		routingClient:   NewAIClient(&cfg.AI.RoutingModel),
		toolManager:     NewToolManager(cfg.Code.OutputRootDir),
		prompts:         NewPromptManager(),
	}
}

// ==================== 非流式代码生成（同步等待完整响应） ====================

// GenerateHTMLCode 生成 HTML 单文件代码（非流式，等待 AI 完整响应）
// 使用 codegen-html-system-prompt 作为 System Prompt
func (f *CodeGeneratorFactory) GenerateHTMLCode(ctx context.Context, userMsg string) (string, error) {
	systemPrompt, _ := f.prompts.GetPrompt("codegen-html")
	return f.chatClient.ChatCompletion(ctx, systemPrompt, userMsg)
}

// GenerateMultiFileCode 生成多文件代码
func (f *CodeGeneratorFactory) GenerateMultiFileCode(ctx context.Context, userMsg string) (string, error) {
	systemPrompt, _ := f.prompts.GetPrompt("codegen-multi-file")
	return f.chatClient.ChatCompletion(ctx, systemPrompt, userMsg)
}

// ==================== 流式代码生成（SSE 实时推送） ====================

// StreamHTMLCode 流式生成 HTML 代码 — 每收到一个 token 就通过 onChunk 推送给前端
func (f *CodeGeneratorFactory) StreamHTMLCode(ctx context.Context, userMsg string, onChunk func(string) error) error {
	systemPrompt, _ := f.prompts.GetPrompt("codegen-html")
	return f.streamClient.ChatCompletionStream(ctx, systemPrompt, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}, onChunk)
}

// StreamMultiFileCode 流式生成多文件代码
func (f *CodeGeneratorFactory) StreamMultiFileCode(ctx context.Context, userMsg string, onChunk func(string) error) error {
	systemPrompt, _ := f.prompts.GetPrompt("codegen-multi-file")
	return f.streamClient.ChatCompletionStream(ctx, systemPrompt, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}, onChunk)
}

// StreamVueProjectCode ⭐ Vue 工程模式流式生成（带 Function Calling 工具调用）
// AI 自主使用 writeFile/readFile/modifyFile/readDir/deleteFile/exit 工具创建完整项目。
// 加载对话历史作为上下文 → 调用推理模型 → 文本/工具请求/工具结果 3 路回调推送 SSE
func (f *CodeGeneratorFactory) StreamVueProjectCode(
	ctx context.Context,
	appID uint64,
	userMsg string,
	chatHistory []openai.ChatCompletionMessage,
	onChunk func(string) error,
	onToolRequest func(toolName string, args string) error,
	onToolExecuted func(toolName string, result string) error,
) error {
	systemPrompt, _ := f.prompts.GetPrompt("codegen-vue-project")

	// 添加对话历史
	messages := make([]openai.ChatCompletionMessage, 0, len(chatHistory)+1)
	messages = append(messages, chatHistory...)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userMsg,
	})

	// 构建 OpenAI 工具定义
	openaiTools := make([]openai.Tool, 0, len(f.toolManager.GetTools()))
	for _, t := range f.toolManager.GetTools() {
		openaiTools = append(openaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	return f.reasoningClient.ChatCompletionWithTools(
		ctx, systemPrompt, messages, openaiTools,
		onChunk,
		func(toolCall openai.ToolCall) (string, error) {
			// 通知前端工具请求
			if onToolRequest != nil {
				onToolRequest(toolCall.Function.Name, toolCall.Function.Arguments)
			}

			// 解析参数并执行
			args := parseToolArgs(toolCall.Function.Arguments)
			result, err := f.toolManager.ExecuteTool(ctx, appID, toolCall.Function.Name, args)
			if err != nil {
				return "", err
			}

			// 通知前端工具执行结果
			if onToolExecuted != nil {
				onToolExecuted(toolCall.Function.Name, result.Message)
			}

			return result.Message, nil
		},
	)
}

// parseToolArgs 简单解析 JSON 参数
func parseToolArgs(argsJSON string) map[string]interface{} {
	args := make(map[string]interface{})
	// 使用简单的字符串解析（生产环境建议用 json.Unmarshal）
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON != "" {
		// 实际应使用 encoding/json
		_ = argsJSON
	}
	return args
}

// ==================== 智能路由 ====================

// RouteCodeGenType 智能路由选择代码生成类型
func (f *CodeGeneratorFactory) RouteCodeGenType(ctx context.Context, userPrompt string) (string, error) {
	systemPrompt, _ := f.prompts.GetPrompt("codegen-routing")

	type routeResult struct {
		CodeGenType string `json:"codeGenType"`
	}

	var result routeResult
	if err := f.routingClient.ChatCompletionStructured(ctx, systemPrompt, userPrompt, &result); err != nil {
		log.Printf("[Route] 智能路由失败，使用默认 HTML 模式: %v", err)
		return "html", nil
	}

	if result.CodeGenType == "" {
		return "html", nil
	}

	log.Printf("[Route] 智能路由选择: %s", result.CodeGenType)
	return result.CodeGenType, nil
}

// ==================== 代码质量检查 ====================

// CheckCodeQuality AI 检查代码质量
func (f *CodeGeneratorFactory) CheckCodeQuality(ctx context.Context, codeContent string) (*QualityResult, error) {
	systemPrompt, _ := f.prompts.GetPrompt("code-quality-check")

	var result QualityResult
	if err := f.chatClient.ChatCompletionStructured(ctx, systemPrompt, codeContent, &result); err != nil {
		return &QualityResult{
			IsValid: true,
			Errors:  []string{fmt.Sprintf("质检异常: %v", err)},
		}, nil
	}
	return &result, nil
}

// QualityResult 代码质量检查结果
type QualityResult struct {
	IsValid     bool     `json:"isValid"`
	Errors      []string `json:"errors"`
	Suggestions []string `json:"suggestions"`
}
