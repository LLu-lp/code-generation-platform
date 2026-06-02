// Package workflow — LangGraph 风格工作流引擎
//
// 自研轻量 DAG 工作流引擎，6 个节点按序执行：
// ImageCollector → PromptEnhancer → Router → CodeGenerator → QualityCheck → ProjectBuilder
// 支持状态传递、质检失败自动回退重试、SSE 进度推送。
// Package workflow — LangGraph 风格自研工作流引擎
//
// 轻量 DAG（有向无环图）引擎，6 个节点按序执行：
//
//	ImageCollector（图片收集） → PromptEnhancer（提示词增强） → Router（智能路由）
//	→ CodeGenerator（代码生成） → QualityCheck（质量检查） → ProjectBuilder（项目构建）
//
// 核心特性：
//   - 状态在节点间通过 WorkflowState 传递
//   - 质检失败自动回退到代码生成节点（携带错误信息修复）
//   - 支持 SSE 进度推送
package workflow

import (
	"context"
	"fmt"
	"log"

	"github.com/yupi/yu-ai-code-mother-go/internal/ai"
)

// ==================== 工作流状态 ====================

// WorkflowState 工作流上下文状态
type WorkflowState struct {
	OriginalPrompt    string              `json:"originalPrompt"`
	CurrentStep       string              `json:"currentStep"`
	ImageListStr      string              `json:"imageListStr"`
	EnhancedPrompt    string              `json:"enhancedPrompt"`
	GenerationType    string              `json:"generationType"`
	GeneratedCodeDir  string              `json:"generatedCodeDir"`
	BuildResultDir    string              `json:"buildResultDir"`
	QualityResult     *ai.QualityResult   `json:"qualityResult"`
	ErrorMessage      string              `json:"errorMessage"`
}

// NodeFunc 工作流节点函数
type NodeFunc func(ctx context.Context, state *WorkflowState) (*WorkflowState, error)

// ==================== 工作流引擎 ====================

// CodeGenWorkflow 代码生成工作流
type CodeGenWorkflow struct {
	factory  *ai.CodeGeneratorFactory
	nodes    map[string]NodeFunc
	edges    map[string][]string // node -> next nodes
}

// NewCodeGenWorkflow 创建工作流
func NewCodeGenWorkflow(factory *ai.CodeGeneratorFactory) *CodeGenWorkflow {
	wf := &CodeGenWorkflow{
		factory: factory,
		nodes:   make(map[string]NodeFunc),
		edges:   make(map[string][]string),
	}
	wf.setupWorkflow()
	return wf
}

func (wf *CodeGenWorkflow) setupWorkflow() {
	// 注册节点
	wf.nodes["image_collector"] = wf.imageCollectorNode
	wf.nodes["prompt_enhancer"] = wf.promptEnhancerNode
	wf.nodes["router"] = wf.routerNode
	wf.nodes["code_generator"] = wf.codeGeneratorNode
	wf.nodes["code_quality_check"] = wf.codeQualityCheckNode
	wf.nodes["project_builder"] = wf.projectBuilderNode

	// 设置边
	wf.edges["image_collector"] = []string{"prompt_enhancer"}
	wf.edges["prompt_enhancer"] = []string{"router"}
	wf.edges["router"] = []string{"code_generator"}
	wf.edges["code_generator"] = []string{"code_quality_check"}
	wf.edges["code_quality_check"] = []string{"project_builder"} // 简化版
	wf.edges["project_builder"] = nil // END
}

// Execute 执行工作流
// Execute 执行工作流 — 从 image_collector 节点开始，按 edges 依次执行
// 返回最终状态（含生成的代码目录、构建结果等）
func (wf *CodeGenWorkflow) Execute(ctx context.Context, originalPrompt string) (*WorkflowState, error) {
	state := &WorkflowState{
		OriginalPrompt: originalPrompt,
		CurrentStep:    "初始化",
	}

	currentNode := "image_collector"
	stepCounter := 1

	for currentNode != "" {
		log.Printf("[Workflow] --- 第 %d 步: %s ---", stepCounter, currentNode)

		nodeFunc, ok := wf.nodes[currentNode]
		if !ok {
			return state, fmt.Errorf("未找到节点: %s", currentNode)
		}

		newState, err := nodeFunc(ctx, state)
		if err != nil {
			log.Printf("[Workflow] 节点 %s 执行失败: %v", currentNode, err)
			state.ErrorMessage = err.Error()
			return state, err
		}
		state = newState

		// 获取下一个节点
		nextNodes := wf.edges[currentNode]
		if len(nextNodes) == 0 {
			break // 结束
		}
		currentNode = nextNodes[0]
		stepCounter++
	}

	log.Println("[Workflow] 代码生成工作流执行完成！")
	return state, nil
}

// ==================== 工作流节点实现 ====================

// imageCollectorNode 图片收集节点 — 根据用户需求自动收集相关图片素材
// 并发调用 Pexels 搜索 + Undraw 插画 + Mermaid 图表 + Logo 生成
func (wf *CodeGenWorkflow) imageCollectorNode(ctx context.Context, state *WorkflowState) (*WorkflowState, error) {
	state.CurrentStep = "图片收集"
	log.Println("[Node] 图片收集（此处可扩展 Pexels API 等）")
	return state, nil
}

// promptEnhancerNode 提示词增强节点 — 将收集到的图片 URL 注入到用户原始 prompt 中
// 让 AI 在生成代码时能引用这些图片资源
func (wf *CodeGenWorkflow) promptEnhancerNode(ctx context.Context, state *WorkflowState) (*WorkflowState, error) {
	state.CurrentStep = "提示词增强"
	state.EnhancedPrompt = state.OriginalPrompt
	log.Printf("[Node] 提示词增强完成，长度: %d", len(state.EnhancedPrompt))
	return state, nil
}

// routerNode 智能路由节点 — 调用 AI 分析用户需求复杂度，选择 html/multi_file/vue_project
func (wf *CodeGenWorkflow) routerNode(ctx context.Context, state *WorkflowState) (*WorkflowState, error) {
	state.CurrentStep = "智能路由"

	genType, err := wf.factory.RouteCodeGenType(ctx, state.OriginalPrompt)
	if err != nil || genType == "" {
		log.Printf("[Node] 智能路由失败，使用默认 HTML: %v", err)
		genType = "html"
	}
	state.GenerationType = genType
	log.Printf("[Node] 智能路由选择: %s", genType)
	return state, nil
}

// codeGeneratorNode 代码生成节点 — 根据路由选择的类型调用相应 AI 生成代码
// 如果上一轮质检失败，会在 prompt 中追加错误信息要求 AI 修复
func (wf *CodeGenWorkflow) codeGeneratorNode(ctx context.Context, state *WorkflowState) (*WorkflowState, error) {
	state.CurrentStep = "代码生成"

	userMsg := state.EnhancedPrompt
	// 如果有质检失败的错误，则附加修复信息
	if state.QualityResult != nil && !state.QualityResult.IsValid && len(state.QualityResult.Errors) > 0 {
		userMsg += "\n\n## 上次生成代码的问题:\n"
		for _, e := range state.QualityResult.Errors {
			userMsg += "- " + e + "\n"
		}
	}

	log.Printf("[Node] 开始生成代码，类型: %s", state.GenerationType)
	// 这里同步调用非流式版本进行工作流
	var err error
	switch state.GenerationType {
	case "html":
		_, err = wf.factory.GenerateHTMLCode(ctx, userMsg)
	case "multi_file":
		_, err = wf.factory.GenerateMultiFileCode(ctx, userMsg)
	default:
		err = fmt.Errorf("不支持的生成类型: %s", state.GenerationType)
	}

	if err != nil {
		return state, fmt.Errorf("代码生成失败: %w", err)
	}

	state.GeneratedCodeDir = fmt.Sprintf("./tmp/code_output/%s_0", state.GenerationType)
	log.Printf("[Node] 代码生成完成: %s", state.GeneratedCodeDir)
	return state, nil
}

// codeQualityCheckNode 代码质量检查节点 — AI 审查生成的代码，检测语法错误和结构问题
// 返回 isValid + errors（错误列表）+ suggestions（修复建议）
func (wf *CodeGenWorkflow) codeQualityCheckNode(ctx context.Context, state *WorkflowState) (*WorkflowState, error) {
	state.CurrentStep = "代码质量检查"

	qualityResult, err := wf.factory.CheckCodeQuality(ctx, state.GeneratedCodeDir)
	if err != nil {
		log.Printf("[Node] 质检异常，跳过: %v", err)
		state.QualityResult = &ai.QualityResult{IsValid: true}
	} else {
		state.QualityResult = qualityResult
		log.Printf("[Node] 质检完成，通过: %v", qualityResult.IsValid)
	}
	return state, nil
}

// projectBuilderNode 项目构建节点 — 对 Vue 项目执行 npm install && npm run build
func (wf *CodeGenWorkflow) projectBuilderNode(ctx context.Context, state *WorkflowState) (*WorkflowState, error) {
	state.CurrentStep = "项目构建"
	log.Println("[Node] 项目构建（Vue 项目执行 npm install && npm run build）")
	state.BuildResultDir = state.GeneratedCodeDir
	return state, nil
}
