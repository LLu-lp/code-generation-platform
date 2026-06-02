// Package core — Vue 项目构建器
//
// VueProjectBuilder 负责执行 npm install + npm run build
// 支持超时控制、Windows/Linux 跨平台、CI 模式（无交互输出）。
// BuildProject：同步构建 + 验证 dist 目录
// BuildProjectAsync：使用 goroutine 异步构建（用于部署后的后台任务）
package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// ==================== Vue 项目构建器 ====================

// VueProjectBuilder Vue 项目构建器 — 在 Go 中调用系统 npm 命令
// installTimeout: npm install 超时（默认 5 分钟，大型项目依赖多）
// buildTimeout: npm run build 超时（默认 3 分钟）
type VueProjectBuilder struct {
	installTimeout time.Duration
	buildTimeout   time.Duration
}

// NewVueProjectBuilder 创建构建器，默认超时：install 5min / build 3min
func NewVueProjectBuilder() *VueProjectBuilder {
	return &VueProjectBuilder{
		installTimeout: 5 * time.Minute,
		buildTimeout:   3 * time.Minute,
	}
}

// BuildProject 构建 Vue 项目
func (b *VueProjectBuilder) BuildProject(projectPath string) error {
	projectDir, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("无效的项目路径: %w", err)
	}

	// 检查路径是否存在
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return fmt.Errorf("项目目录不存在: %s", projectDir)
	}

	// 检查 package.json
	pkgJSON := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(pkgJSON); os.IsNotExist(err) {
		return fmt.Errorf("package.json 不存在: %s", pkgJSON)
	}

	log.Printf("[Builder] 开始构建 Vue 项目: %s", projectDir)

	// 1. npm install
	if err := b.executeCommand(projectDir, "npm", []string{"install"}, b.installTimeout); err != nil {
		return fmt.Errorf("npm install 失败: %w", err)
	}

	// 2. npm run build
	if err := b.executeCommand(projectDir, "npm", []string{"run", "build"}, b.buildTimeout); err != nil {
		return fmt.Errorf("npm run build 失败: %w", err)
	}

	// 3. 验证 dist 目录
	distDir := filepath.Join(projectDir, "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return fmt.Errorf("构建完成但 dist 目录未生成")
	}

	log.Printf("[Builder] Vue 项目构建成功，dist 目录: %s", distDir)
	return nil
}

// BuildProjectAsync 异步构建
func (b *VueProjectBuilder) BuildProjectAsync(projectPath string) {
	go func() {
		if err := b.BuildProject(projectPath); err != nil {
			log.Printf("[Builder] 异步构建失败: %v", err)
		}
	}()
}

// executeCommand 执行系统命令
func (b *VueProjectBuilder) executeCommand(workDir, command string, args []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Windows 下 npm 使用 .cmd
	cmdName := command
	if runtime.GOOS == "windows" {
		cmdName = command + ".cmd"
	}

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CI=true") // CI 模式，避免交互式输出

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("命令执行超时 (%v): %s %v", timeout, command, args)
		}
		// 截断输出避免日志过大
		outputStr := string(output)
		if len(outputStr) > 1000 {
			outputStr = outputStr[len(outputStr)-1000:]
		}
		return fmt.Errorf("命令失败: %s %v, 输出: %s", command, args, outputStr)
	}

	return nil
}
