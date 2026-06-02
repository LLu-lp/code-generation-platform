// Package workflow — 工作流工具与 AI 服务
//
// 包含工作流专用工具：图片搜索（Pexels API）、Mermaid 架构图生成、Logo 生成、插画搜索。
// 以及 AI 服务：代码质检服务（CodeQualityCheckService）、图片收集规划服务（ImagePlanService）。
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/yupi/yu-ai-code-mother-go/internal/ai"
)

// ==================== 图片资源 ====================

// ImageCategory 图片类别
type ImageCategory string

const (
	ImageCategoryContent      ImageCategory = "内容图片"
	ImageCategoryIllustration ImageCategory = "插画"
	ImageCategoryDiagram      ImageCategory = "架构图"
	ImageCategoryLogo         ImageCategory = "Logo"
)

// ImageResource 图片资源
type ImageResource struct {
	Category    ImageCategory `json:"category"`
	Description string        `json:"description"`
	URL         string        `json:"url"`
}

// ImageCollectionPlan 图片收集计划
type ImageCollectionPlan struct {
	ContentImageTasks []ImageSearchTask   `json:"contentImageTasks"`
	IllustrationTasks []IllustrationTask  `json:"illustrationTasks"`
	DiagramTasks      []DiagramTask       `json:"diagramTasks"`
	LogoTasks         []LogoTask          `json:"logoTasks"`
}

type ImageSearchTask struct {
	Query string `json:"query"`
}

type IllustrationTask struct {
	Query string `json:"query"`
}

type DiagramTask struct {
	MermaidCode string `json:"mermaidCode"`
	Description string `json:"description"`
}

type LogoTask struct {
	Description string `json:"description"`
}

// ==================== LangGraph 工作流工具 ====================

// ImageSearchTool 图片搜索工具（Pexels API）
type ImageSearchTool struct {
	pexelsAPIKey string
}

func NewImageSearchTool(apiKey string) *ImageSearchTool {
	return &ImageSearchTool{pexelsAPIKey: apiKey}
}

func (t *ImageSearchTool) SearchContentImages(ctx context.Context, query string) ([]ImageResource, error) {
	if t.pexelsAPIKey == "" {
		return nil, nil
	}
	// 调用 Pexels API
	url := fmt.Sprintf("https://api.pexels.com/v1/search?query=%s&per_page=12&page=1", query)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", t.pexelsAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Pexels API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Photos []struct {
			Src struct {
				Medium string `json:"medium"`
			} `json:"src"`
			Alt string `json:"alt"`
		} `json:"photos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var images []ImageResource
	for _, photo := range result.Photos {
		desc := photo.Alt
		if desc == "" {
			desc = query
		}
		images = append(images, ImageResource{
			Category:    ImageCategoryContent,
			Description: desc,
			URL:         photo.Src.Medium,
		})
	}
	log.Printf("[ImageSearch] 搜索 '%s' 找到 %d 张图片", query, len(images))
	return images, nil
}

// UndrawIllustrationTool 插画搜索工具
type UndrawIllustrationTool struct{}

func NewUndrawIllustrationTool() *UndrawIllustrationTool {
	return &UndrawIllustrationTool{}
}

func (t *UndrawIllustrationTool) SearchIllustrations(ctx context.Context, query string) ([]ImageResource, error) {
	// undraw.co 的 SVG 插画
	svgURL := fmt.Sprintf("https://undraw.co/api/illustrations?search=%s", strings.ReplaceAll(query, " ", "+"))
	log.Printf("[Illustration] 搜索插画: %s", svgURL)
	// 简化为占位实现
	return nil, nil
}

// MermaidDiagramTool Mermaid 架构图生成工具
type MermaidDiagramTool struct{}

func NewMermaidDiagramTool() *MermaidDiagramTool {
	return &MermaidDiagramTool{}
}

func (t *MermaidDiagramTool) GenerateMermaidDiagram(ctx context.Context, mermaidCode, description string) ([]ImageResource, error) {
	// 使用 mermaid.ink 将 Mermaid 代码转为图片 URL
	encodedCode := strings.ReplaceAll(mermaidCode, "\n", "%0A")
	imgURL := fmt.Sprintf("https://mermaid.ink/img/%s", encodedCode)
	log.Printf("[Mermaid] 生成架构图: %s", description)
	return []ImageResource{{
		Category:    ImageCategoryDiagram,
		Description: description,
		URL:         imgURL,
	}}, nil
}

// LogoGeneratorTool Logo 生成工具
type LogoGeneratorTool struct {
	dashscopeAPIKey  string
	dashscopeModel   string
}

func NewLogoGeneratorTool(apiKey, model string) *LogoGeneratorTool {
	return &LogoGeneratorTool{dashscopeAPIKey: apiKey, dashscopeModel: model}
}

func (t *LogoGeneratorTool) GenerateLogos(ctx context.Context, description string) ([]ImageResource, error) {
	log.Printf("[Logo] 生成 Logo: %s", description)
	// 调用阿里云 DashScope 文生图 API
	// 此处为简化实现
	return nil, nil
}

// ==================== AI 服务 ====================

// WorkflowAIServices 工作流 AI 服务集合
type WorkflowAIServices struct {
	CodeQualityCheck *CodeQualityCheckService
	ImagePlan        *ImagePlanService
	Factory          *ai.CodeGeneratorFactory
}

// CodeQualityCheckService 代码质检服务
type CodeQualityCheckService struct {
	factory *ai.CodeGeneratorFactory
}

func NewCodeQualityCheckService(factory *ai.CodeGeneratorFactory) *CodeQualityCheckService {
	return &CodeQualityCheckService{factory: factory}
}

func (s *CodeQualityCheckService) CheckCode(ctx context.Context, codeContent string) (*ai.QualityResult, error) {
	return s.factory.CheckCodeQuality(ctx, codeContent)
}

// ImagePlanService 图片收集规划服务
type ImagePlanService struct {
	factory *ai.CodeGeneratorFactory
}

func NewImagePlanService(factory *ai.CodeGeneratorFactory) *ImagePlanService {
	return &ImagePlanService{factory: factory}
}

func (s *ImagePlanService) PlanCollection(ctx context.Context, userPrompt string) (*ImageCollectionPlan, error) {
	log.Printf("[ImagePlan] 分析图片需求: %s", truncate(userPrompt, 100))
	// 使用 AI 分析需要什么类型的图片
	// 此处为简化实现，返回默认计划
	return &ImageCollectionPlan{}, nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
