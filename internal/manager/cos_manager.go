// Package manager — 腾讯云 COS 对象存储管理器
//
// 封装 COS Go SDK：文件上传、字节上传，自动构造访问 URL。
// 用于截图上传、用户头像等静态资源管理。
// Package manager — 腾讯云 COS 对象存储管理器
//
// 封装 COS Go SDK：文件上传（本地文件/字节内容）、自动构造 CDN 访问 URL。
// 用于截图上传、用户头像管理等静态资源持久化场景。
// 若 COS 配置不完整则跳过初始化，降级为无操作（不影响主流程）。
package manager

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/tencentyun/cos-go-sdk-v5"

	"github.com/yupi/yu-ai-code-mother-go/internal/config"
)

// CosManager 腾讯云 COS 对象存储管理器
type CosManager struct {
	client *cos.Client
	host   string
}

// NewCosManager 创建 COS 管理器
func NewCosManager(cfg *config.COSConfig) (*CosManager, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		log.Println("[COS] 配置不完整，跳过初始化")
		return &CosManager{}, nil
	}

	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("解析 COS URL 失败: %w", err)
	}

	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &CosManager{
		client: client,
		host:   cfg.Host,
	}, nil
}

// UploadFile 上传文件到 COS
// cosKey: 对象键（如 /screenshots/2025/06/02/abc.jpg）
// localFilePath: 本地文件路径
// 返回: 访问 URL
// UploadFile 上传本地文件到 COS
// cosKey：对象键（如 /screenshots/2025/06/02/abc.jpg）
// localFilePath：本地文件绝对路径
// 返回：CDN 访问 URL
func (m *CosManager) UploadFile(cosKey string, localFilePath string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("COS 客户端未初始化")
	}

	// 打开本地文件
	file, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer file.Close()

	// 上传
	_, err = m.client.Object.Put(context.Background(), cosKey, file, nil)
	if err != nil {
		return "", fmt.Errorf("上传 COS 失败: %w", err)
	}

	// 构造访问 URL
	accessURL := fmt.Sprintf("https://%s%s", m.host, cosKey)
	log.Printf("[COS] 上传成功: %s", accessURL)
	return accessURL, nil
}

// UploadBytes 上传字节内容到 COS
func (m *CosManager) UploadBytes(cosKey string, data []byte, contentType string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("COS 客户端未初始化")
	}

	opt := &cos.ObjectPutOptions{}
	if contentType != "" {
		opt.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		}
	}

	_, err := m.client.Object.Put(context.Background(), cosKey, bytesToReader(data), opt)
	if err != nil {
		return "", fmt.Errorf("上传 COS 失败: %w", err)
	}

	accessURL := fmt.Sprintf("https://%s%s", m.host, cosKey)
	return accessURL, nil
}

func bytesToReader(data []byte) *readerWrapper {
	return &readerWrapper{data: data, pos: 0}
}

type readerWrapper struct {
	data []byte
	pos  int
}

func (r *readerWrapper) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
