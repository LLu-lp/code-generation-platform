// Package service — 截图服务实现
//
// 使用 chromedp（Chrome DevTools Protocol）截取网页全页面截图，
// 通过腾讯云 COS SDK 上传到对象存储，返回可访问的图片 URL。
package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"

	"github.com/yupi/yu-ai-code-mother-go/internal/config"
	"github.com/yupi/yu-ai-code-mother-go/internal/manager"
)

// ScreenshotServiceImpl 截图服务实现
type ScreenshotServiceImpl struct {
	cfg        *config.Config
	cosManager *manager.CosManager
}

// NewScreenshotServiceImpl 创建截图服务
func NewScreenshotServiceImpl(cfg *config.Config, cosMgr *manager.CosManager) *ScreenshotServiceImpl {
	return &ScreenshotServiceImpl{
		cfg:        cfg,
		cosManager: cosMgr,
	}
}

// GenerateAndUploadScreenshot 生成网页截图并上传到 COS
func (s *ScreenshotServiceImpl) GenerateAndUploadScreenshot(webURL string) string {
	if webURL == "" {
		log.Println("[Screenshot] URL 为空")
		return ""
	}

	log.Printf("[Screenshot] 开始截图: %s", webURL)

	// 1. 使用 chromedp 截图
	localPath, err := s.captureScreenshot(webURL)
	if err != nil {
		log.Printf("[Screenshot] 截图失败: %v", err)
		return ""
	}
	defer cleanupLocalFile(localPath)

	// 2. 上传到 COS
	cosURL, err := s.uploadToCOS(localPath)
	if err != nil {
		log.Printf("[Screenshot] 上传 COS 失败: %v", err)
		return ""
	}

	log.Printf("[Screenshot] 截图完成: %s", cosURL)
	return cosURL
}

// captureScreenshot 使用 chromedp 捕获网页截图
func (s *ScreenshotServiceImpl) captureScreenshot(webURL string) (string, error) {
	// 创建临时文件
	tmpDir := os.TempDir()
	fileName := fmt.Sprintf("screenshot_%s.png", uuid.New().String()[:8])
	localPath := filepath.Join(tmpDir, fileName)

	// 创建 chromedp 上下文
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// 设置超时
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	// 截取全页面截图
	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(webURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // 等待动画加载
		chromedp.FullScreenshot(&buf, 80), // 80% 质量
	)

	if err != nil {
		return "", fmt.Errorf("chromedp 截图失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(localPath, buf, 0644); err != nil {
		return "", fmt.Errorf("写入截图文件失败: %w", err)
	}

	return localPath, nil
}

// uploadToCOS 上传截图到 COS
func (s *ScreenshotServiceImpl) uploadToCOS(localPath string) (string, error) {
	if s.cosManager == nil {
		return "", fmt.Errorf("COS 管理器未初始化")
	}

	fileName := fmt.Sprintf("%s_compressed.jpg", uuid.New().String()[:8])
	datePath := time.Now().Format("2006/01/02")
	cosKey := fmt.Sprintf("/screenshots/%s/%s", datePath, fileName)

	return s.cosManager.UploadFile(cosKey, localPath)
}

// cleanupLocalFile 清理本地临时文件
func cleanupLocalFile(path string) {
	if path != "" {
		if err := os.Remove(path); err != nil {
			log.Printf("[Screenshot] 清理临时文件失败: %v", err)
		}
	}
}
