// Package service — 项目下载服务
//
// 将应用代码目录打包为 ZIP 文件，通过 HTTP ResponseWriter 流式输出给客户端下载。
package service

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yupi/yu-ai-code-mother-go/pkg/utils"
)

// ProjectDownloadService 项目下载服务
type ProjectDownloadService struct{}

// NewProjectDownloadService 创建下载服务
func NewProjectDownloadService() *ProjectDownloadService {
	return &ProjectDownloadService{}
}

// DownloadProjectAsZip 将项目目录打包为 ZIP 并写入 HTTP 响应
func (s *ProjectDownloadService) DownloadProjectAsZip(sourceDir, downloadFileName string, w http.ResponseWriter) error {
	// 1. 检查源目录是否存在
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("项目目录不存在: %s", sourceDir)
	}

	// 2. 设置响应头
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", downloadFileName))
	w.Header().Set("Cache-Control", "no-cache")

	// 3. 打包并写入响应
	if err := utils.ZipDirectory(sourceDir, w); err != nil {
		log.Printf("[Download] ZIP 打包失败: %v", err)
		return fmt.Errorf("打包下载失败: %w", err)
	}

	log.Printf("[Download] 项目下载成功: %s", filepath.Base(sourceDir))
	return nil
}
