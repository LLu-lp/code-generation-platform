// Package ai — Prompt 管理与安全护栏
//
// PromptManager：管理 AI 对话的 System Prompt 模板，支持内置默认 + 文件系统加载。
// InputGuardrail：输入安全审查，检测 Prompt 注入攻击、敏感词、超长输入等。
package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PromptManager Prompt 管理器
type PromptManager struct {
	cache map[string]string
	mu    sync.RWMutex
}

// NewPromptManager 创建 Prompt 管理器
func NewPromptManager() *PromptManager {
	pm := &PromptManager{
		cache: make(map[string]string),
	}
	// 预加载默认 prompts
	pm.loadDefaults()
	return pm
}

// GetPrompt 获取 Prompt 内容
func (pm *PromptManager) GetPrompt(name string) (string, error) {
	pm.mu.RLock()
	if content, ok := pm.cache[name]; ok {
		pm.mu.RUnlock()
		return content, nil
	}
	pm.mu.RUnlock()

	// 尝试从文件系统读取
	path := filepath.Join("prompts", name+"-system-prompt.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 prompt 文件失败: %s, err: %w", path, err)
	}

	content := string(data)
	pm.mu.Lock()
	pm.cache[name] = content
	pm.mu.Unlock()

	return content, nil
}

// loadDefaults 加载默认的内置 prompts
func (pm *PromptManager) loadDefaults() {
	pm.cache["codegen-html"] = `你是一位资深的 Web 前端开发专家，精通 HTML、CSS 和原生 JavaScript。
你的任务是根据用户提供的网站描述，生成一个完整、独立的单页面网站。

约束:
1. 技术栈: 只能使用 HTML、CSS 和原生 JavaScript。
2. 禁止外部依赖: 绝对不允许使用任何外部 CSS 框架、JS 库。
3. 独立文件: 将所有 CSS 内联在 <style> 中，JS 内联在 <script> 中。
4. 响应式设计: 使用 Flexbox 或 Grid 进行布局。
5. 输出格式: 只输出 HTML 代码块。`

	pm.cache["codegen-multi-file"] = `你是一位资深的 Web 前端开发专家。
你的任务是根据用户提供的网站描述，创建三个核心文件：HTML, CSS, JavaScript。

约束:
1. 技术栈: 只能使用 HTML、CSS 和原生 JavaScript。
2. 文件分离: index.html, style.css, script.js
3. 禁止外部依赖。
4. 响应式设计。

输出格式:
` + "```html" + `
... HTML ...
` + "```" + `

` + "```css" + `
... CSS ...
` + "```" + `

` + "```javascript" + `
... JavaScript ...
` + "```"

	pm.cache["codegen-vue-project"] = `你是一位资深的 Vue3 前端架构师。
你的任务是根据用户提供的项目描述，创建一个完整的、可运行的 Vue3 工程项目。

## 技术栈
- Vue 3.x（Composition API + <script setup>）
- Vite
- Vue Router 4.x

## 项目结构
project/
├── index.html
├── package.json
├── vite.config.js
├── src/
│   ├── main.js
│   ├── App.vue
│   ├── router/index.js
│   ├── components/
│   ├── pages/
│   └── styles/

## 约束
1. 使用 Composition API + <script setup>
2. 路由使用 hash 模式
3. 必须使用文件写入工具逐一创建每个文件
4. 使用 ` + "`./`" + ` 作为 base 路径`

	pm.cache["codegen-routing"] = `你是一个专业的代码生成方案路由器。
根据用户需求返回最合适的代码生成类型。

可选类型:
- html: 适合简单的静态页面，单个 HTML 文件
- multi_file: 适合多文件静态页面
- vue_project: 适合复杂的现代化前端项目

返回 JSON: {"codeGenType": "html|multi_file|vue_project"}`

	pm.cache["code-quality-check"] = `你是一个专业的代码质量检查专家。
分析代码并返回 JSON:
{"isValid": true/false, "errors": ["...", "..."], "suggestions": ["...", "..."]}

检查重点: 语法错误、结构问题、缺失依赖、代码重复、功能完整性。`

	pm.cache["image-collection-plan"] = `根据用户的网站需求，规划需要的图片素材。
返回 JSON 格式的图片收集计划。`
}

// ==================== 输入安全护栏 ====================

// GuardrailResult 护栏检查结果
type GuardrailResult struct {
	Allowed bool
	Reason  string
}

// InputGuardrail 输入安全护栏
type InputGuardrail struct {
	sensitiveWords    []string
	injectionPatterns []string
	maxInputLength    int
}

// NewInputGuardrail 创建输入护栏
func NewInputGuardrail() *InputGuardrail {
	return &InputGuardrail{
		sensitiveWords: []string{
			"忽略之前的指令", "ignore previous instructions",
			"破解", "hack", "绕过", "bypass", "越狱", "jailbreak",
		},
		injectionPatterns: []string{
			"ignore previous instructions",
			"forget everything above",
			"pretend you are",
			"system: you are",
			"new instructions:",
		},
		maxInputLength: 1000,
	}
}

// Validate 验证输入
func (g *InputGuardrail) Validate(input string) *GuardrailResult {
	if strings.TrimSpace(input) == "" {
		return &GuardrailResult{Allowed: false, Reason: "输入内容不能为空"}
	}

	if len(input) > g.maxInputLength {
		return &GuardrailResult{Allowed: false, Reason: fmt.Sprintf("输入过长，不超过%d字", g.maxInputLength)}
	}

	lowerInput := strings.ToLower(input)
	for _, word := range g.sensitiveWords {
		if strings.Contains(lowerInput, strings.ToLower(word)) {
			return &GuardrailResult{Allowed: false, Reason: "输入包含不当内容"}
		}
	}

	for _, pattern := range g.injectionPatterns {
		if strings.Contains(lowerInput, strings.ToLower(pattern)) {
			return &GuardrailResult{Allowed: false, Reason: "检测到恶意输入"}
		}
	}

	return &GuardrailResult{Allowed: true}
}
