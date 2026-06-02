// Package core — 代码解析与保存
//
// 负责将 AI 输出的 Markdown 代码块解析为结构化文件并保存到磁盘。
// ParseCode：从 AI 输出中提取各语言代码块（html/css/js）
// SaveCodeToDir：将解析后的文件写入沙箱目录
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yupi/yu-ai-code-mother-go/internal/exception"
)

// ==================== 代码解析 ====================

// ParsedFile 解析后的文件
type ParsedFile struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

// ParseCode 解析 AI 输出的代码块，提取各文件内容
// 支持 `html` / `css` / `javascript` / `vue` 等语言标记
func ParseCode(codeGenType, aiOutput string) ([]ParsedFile, error) {
	switch codeGenType {
	case "html":
		return parseHTMLCode(aiOutput)
	case "multi_file":
		return parseMultiFileCode(aiOutput)
	default:
		return nil, exception.NewBizError(exception.ParamsError, "不支持的代码生成类型: "+codeGenType)
	}
}

// parseHTMLCode 解析 HTML 单文件（提取 ```html 代码块）
func parseHTMLCode(aiOutput string) ([]ParsedFile, error) {
	html := extractCodeBlock(aiOutput, "html")
	if html == "" {
		// 尝试直接使用整个输出
		html = aiOutput
	}
	return []ParsedFile{
		{FileName: "index.html", Content: html},
	}, nil
}

// parseMultiFileCode 解析多文件（提取 html/css/js 代码块）
func parseMultiFileCode(aiOutput string) ([]ParsedFile, error) {
	var files []ParsedFile
	if html := extractCodeBlock(aiOutput, "html"); html != "" {
		files = append(files, ParsedFile{FileName: "index.html", Content: html})
	}
	if css := extractCodeBlock(aiOutput, "css"); css != "" {
		files = append(files, ParsedFile{FileName: "style.css", Content: css})
	}
	if js := extractCodeBlock(aiOutput, "javascript"); js != "" {
		files = append(files, ParsedFile{FileName: "script.js", Content: js})
	}
	if len(files) == 0 {
		return nil, exception.NewBizError(exception.ParamsError, "未能解析出任何代码文件")
	}
	return files, nil
}

// extractCodeBlock 从 Markdown 中提取指定语言的代码块
func extractCodeBlock(content, language string) string {
	marker := "```" + language
	start := strings.Index(content, marker)
	if start == -1 {
		// 尝试其他变体
		for _, alt := range []string{"```" + language, "``` " + language} {
			start = strings.Index(content, alt)
			if start != -1 {
				break
			}
		}
	}
	if start == -1 {
		return ""
	}
	start += len(marker)
	// 跳过换行符
	for start < len(content) && (content[start] == '\n' || content[start] == '\r') {
		start++
	}
	end := strings.Index(content[start:], "```")
	if end == -1 {
		return strings.TrimSpace(content[start:])
	}
	return strings.TrimSpace(content[start : start+end])
}

// ==================== 代码保存 ====================

// SaveCodeToDir 将解析后的文件保存到目录
func SaveCodeToDir(codeGenType string, appID uint64, outputRoot string, files []ParsedFile) (string, error) {
	dirName := fmt.Sprintf("%s_%d", codeGenType, appID)
	targetDir := filepath.Join(outputRoot, dirName)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	for _, f := range files {
		filePath := filepath.Join(targetDir, f.FileName)
		// 确保子目录存在
		if dir := filepath.Dir(filePath); dir != targetDir {
			os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(filePath, []byte(f.Content), 0644); err != nil {
			return "", fmt.Errorf("写入文件 %s 失败: %w", f.FileName, err)
		}
	}

	return targetDir, nil
}

// ==================== Vue 项目构建 ====================

// BuildVueProject Vue 项目构建（npm install + npm run build）
func BuildVueProject(projectPath string) error {
	// 检查 package.json 是否存在
	pkgJSON := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(pkgJSON); os.IsNotExist(err) {
		return fmt.Errorf("package.json 不存在: %s", projectPath)
	}

	// 此处需要 golang 执行外部命令
	// npm install 和 npm run build 的具体实现在 builder.go 中
	return nil
}
