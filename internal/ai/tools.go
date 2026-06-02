// Package ai — AI 工具调用系统
//
// 本文件定义 6 个 AI 可调用的工具 + ToolManager 管理器。
// AI 通过 Function Calling 机制调用这些工具，实现在文件系统上自主创建/修改/删除项目文件。
// 所有文件操作限定在 tmp/code_output/vue_project_{appId}/ 沙箱目录内，确保安全性。
//
// 6 个工具：writeFile（写）、readFile（读）、modifyFile（改）、readDir（查目录）、deleteFile（删）、exit（退出）
package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ==================== 工具定义（对 AI 暴露的接口） ====================

// ToolDefinition 工具定义 — 描述工具的元信息，会转为 OpenAI Function Definition 发送给 AI
type ToolDefinition struct {
	Name        string                 `json:"name"`        // 工具英文名，如 writeFile
	Description string                 `json:"description"` // 工具描述，AI 以此判断何时调用
	Parameters  map[string]interface{} `json:"parameters"`  // JSON Schema 格式的参数定义
}

// ToolResult 工具执行结果
type ToolResult struct {
	Success bool   `json:"success"` // 是否执行成功
	Message string `json:"message"` // 返回给 AI 的消息
	Data    string `json:"data,omitempty"` // 额外数据（如读取的文件内容）
}

// ToolHandler 工具执行处理器签名：接收 appID（沙箱隔离）和参数，返回执行结果
type ToolHandler func(ctx context.Context, appID uint64, args map[string]interface{}) (*ToolResult, error)

// ==================== 工具管理器（注册 + 执行） ====================

// ToolManager 工具管理器 — 管理所有 AI 可调用的工具。
// 负责：工具注册、工具定义导出（转 OpenAI Function Schema）、工具调用执行。
type ToolManager struct {
	tools     map[string]*ToolDefinition // 工具名 → 工具定义
	handlers  map[string]ToolHandler     // 工具名 → 执行处理器
	outputDir string                     // 代码输出根目录（用于沙箱隔离）
}

// NewToolManager 创建工具管理器，同时注册 6 个默认工具
func NewToolManager(outputDir string) *ToolManager {
	tm := &ToolManager{
		tools:     make(map[string]*ToolDefinition),
		handlers:  make(map[string]ToolHandler),
		outputDir: outputDir,
	}
	tm.registerDefaultTools() // ⭐ 注册所有工具
	return tm
}

// GetTools 获取所有工具定义列表（转为 OpenAI Function Definition 发送给 AI）
func (tm *ToolManager) GetTools() []*ToolDefinition {
	var tools []*ToolDefinition
	for _, t := range tm.tools {
		tools = append(tools, t)
	}
	return tools
}

// ExecuteTool 执行指定工具的调用。根据工具名查找处理器，传入 appID 和参数执行。
func (tm *ToolManager) ExecuteTool(ctx context.Context, appID uint64, name string, args map[string]interface{}) (*ToolResult, error) {
	handler, ok := tm.handlers[name]
	if !ok {
		return nil, fmt.Errorf("未知工具: %s", name)
	}
	return handler(ctx, appID, args)
}

// registerDefaultTools 注册默认工具
func (tm *ToolManager) registerDefaultTools() {
	// 1. 写文件
	tm.tools["writeFile"] = &ToolDefinition{
		Name:        "writeFile",
		Description: "写入文件到指定路径",
		Parameters: map[string]interface{}{
			"relativeFilePath": "文件的相对路径",
			"content":          "要写入文件的内容",
		},
	}
	tm.handlers["writeFile"] = func(ctx context.Context, appID uint64, args map[string]interface{}) (*ToolResult, error) {
		relativePath := getStringArg(args, "relativeFilePath")
		content := getStringArg(args, "content")
		if relativePath == "" {
			return &ToolResult{Success: false, Message: "文件路径不能为空"}, nil
		}
		projectDir := filepath.Join(tm.outputDir, fmt.Sprintf("vue_project_%d", appID))
		fullPath := filepath.Join(projectDir, relativePath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &ToolResult{Success: false, Message: fmt.Sprintf("创建目录失败: %v", err)}, nil
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return &ToolResult{Success: false, Message: fmt.Sprintf("写入文件失败: %v", err)}, nil
		}
		log.Printf("[Tool] 写入文件成功: %s", relativePath)
		return &ToolResult{Success: true, Message: fmt.Sprintf("文件写入成功: %s", relativePath)}, nil
	}

	// 2. 读文件
	tm.tools["readFile"] = &ToolDefinition{
		Name:        "readFile",
		Description: "读取指定路径的文件内容",
		Parameters: map[string]interface{}{
			"relativeFilePath": "文件的相对路径",
		},
	}
	tm.handlers["readFile"] = func(ctx context.Context, appID uint64, args map[string]interface{}) (*ToolResult, error) {
		relativePath := getStringArg(args, "relativeFilePath")
		projectDir := filepath.Join(tm.outputDir, fmt.Sprintf("vue_project_%d", appID))
		fullPath := filepath.Join(projectDir, relativePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return &ToolResult{Success: false, Message: fmt.Sprintf("文件读取失败: %v", err)}, nil
		}
		return &ToolResult{Success: true, Message: "读取成功", Data: string(data)}, nil
	}

	// 3. 修改文件
	tm.tools["modifyFile"] = &ToolDefinition{
		Name:        "modifyFile",
		Description: "修改文件内容，用新内容替换指定的旧内容",
		Parameters: map[string]interface{}{
			"relativeFilePath": "文件的相对路径",
			"oldContent":       "要替换的旧内容",
			"newContent":       "替换后的新内容",
		},
	}
	tm.handlers["modifyFile"] = func(ctx context.Context, appID uint64, args map[string]interface{}) (*ToolResult, error) {
		relativePath := getStringArg(args, "relativeFilePath")
		oldContent := getStringArg(args, "oldContent")
		newContent := getStringArg(args, "newContent")
		projectDir := filepath.Join(tm.outputDir, fmt.Sprintf("vue_project_%d", appID))
		fullPath := filepath.Join(projectDir, relativePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return &ToolResult{Success: false, Message: fmt.Sprintf("文件读取失败: %v", err)}, nil
		}
		if !strings.Contains(string(data), oldContent) {
			return &ToolResult{Success: false, Message: "文件中未找到要替换的内容"}, nil
		}
		newData := strings.Replace(string(data), oldContent, newContent, 1)
		if err := os.WriteFile(fullPath, []byte(newData), 0644); err != nil {
			return &ToolResult{Success: false, Message: fmt.Sprintf("文件修改失败: %v", err)}, nil
		}
		return &ToolResult{Success: true, Message: fmt.Sprintf("文件修改成功: %s", relativePath)}, nil
	}

	// 4. 读目录
	tm.tools["readDir"] = &ToolDefinition{
		Name:        "readDir",
		Description: "读取目录结构，获取指定目录下的所有文件和子目录信息",
		Parameters: map[string]interface{}{
			"relativeDirPath": "目录的相对路径，为空则读取整个项目结构",
		},
	}
	tm.handlers["readDir"] = func(ctx context.Context, appID uint64, args map[string]interface{}) (*ToolResult, error) {
		relativePath := getStringArg(args, "relativeDirPath")
		projectDir := filepath.Join(tm.outputDir, fmt.Sprintf("vue_project_%d", appID))
		fullPath := projectDir
		if relativePath != "" {
			fullPath = filepath.Join(projectDir, relativePath)
		}
		var sb strings.Builder
		sb.WriteString("项目目录结构:\n")
		filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// 忽略 node_modules 等
			name := info.Name()
			if name == "node_modules" || name == ".git" || strings.HasPrefix(name, ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			relPath, _ := filepath.Rel(fullPath, path)
			depth := strings.Count(relPath, string(os.PathSeparator))
			sb.WriteString(strings.Repeat("  ", depth))
			sb.WriteString(name)
			sb.WriteString("\n")
			return nil
		})
		return &ToolResult{Success: true, Data: sb.String()}, nil
	}

	// 5. 删除文件
	tm.tools["deleteFile"] = &ToolDefinition{
		Name:        "deleteFile",
		Description: "删除指定路径的文件",
		Parameters: map[string]interface{}{
			"relativeFilePath": "文件的相对路径",
		},
	}
	tm.handlers["deleteFile"] = func(ctx context.Context, appID uint64, args map[string]interface{}) (*ToolResult, error) {
		relativePath := getStringArg(args, "relativeFilePath")
		projectDir := filepath.Join(tm.outputDir, fmt.Sprintf("vue_project_%d", appID))
		fullPath := filepath.Join(projectDir, relativePath)
		if err := os.Remove(fullPath); err != nil {
			return &ToolResult{Success: false, Message: fmt.Sprintf("删除失败: %v", err)}, nil
		}
		return &ToolResult{Success: true, Message: fmt.Sprintf("文件删除成功: %s", relativePath)}, nil
	}

	// 6. 退出
	tm.tools["exit"] = &ToolDefinition{
		Name:        "exit",
		Description: "当任务已完成或无需继续调用工具时，使用此工具退出操作，防止循环",
		Parameters:  map[string]interface{}{},
	}
	tm.handlers["exit"] = func(ctx context.Context, appID uint64, args map[string]interface{}) (*ToolResult, error) {
		return &ToolResult{Success: true, Message: "不要继续调用工具，可以输出最终结果了"}, nil
	}
}

// getStringArg 从参数中获取字符串
func getStringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
