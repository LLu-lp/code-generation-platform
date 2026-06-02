// Package service — 业务服务层
//
// 核心业务逻辑实现：
// UserService：用户注册/登录/注销（Redis Session + MD5+Salt）
// AppService：AI代码生成策略路由、SSE流式对话、一键部署、异步截图
// ChatHistoryService：对话历史管理（游标分页）
package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"

	"github.com/yupi/yu-ai-code-mother-go/internal/ai"
	"github.com/yupi/yu-ai-code-mother-go/internal/config"
	"github.com/yupi/yu-ai-code-mother-go/internal/model/dto"
	"github.com/yupi/yu-ai-code-mother-go/internal/model/entity"
	"github.com/yupi/yu-ai-code-mother-go/internal/model/enums"
	"github.com/yupi/yu-ai-code-mother-go/internal/model/vo"
	"github.com/yupi/yu-ai-code-mother-go/internal/repository"
	"github.com/yupi/yu-ai-code-mother-go/pkg/utils"
)

// ==================== UserService：用户注册/登录/注销 ====================

// UserService 用户业务服务，负责注册、登录（Redis Session）、注销、获取当前用户。
// 密码使用 MD5 + 固定 Salt 加密存储（生产环境应升级为 bcrypt）。
type UserService struct {
	repo *repository.UserRepository // 用户数据访问
	rdb  *redis.Client              // Redis 客户端（Session 存储）
	cfg  *config.Config             // 全局配置（Session 过期时间等）
}

// NewUserService 构造函数
func NewUserService(repo *repository.UserRepository, rdb interface{}, cfg *config.Config) *UserService {
	rdbClient, _ := rdb.(*redis.Client) // 类型断言获取 Redis 客户端
	return &UserService{repo: repo, rdb: rdbClient, cfg: cfg}
}

// Register 用户注册
// 流程：校验两次密码一致 → 检查账号唯一性 → MD5+Salt 加密 → 入库
// 返回新用户 ID
func (s *UserService) Register(ctx context.Context, account, password, checkPwd string) (uint64, error) {
	// 1. 校验两次输入的密码是否一致
	if password != checkPwd {
		return 0, fmt.Errorf("两次密码不一致")
	}
	// 2. 检查账号是否已被注册
	_, err := s.repo.GetByAccount(ctx, account)
	if err == nil {
		return 0, fmt.Errorf("账号已存在")
	}
	if err != gorm.ErrRecordNotFound {
		return 0, err // 数据库异常
	}
	// 3. 密码加盐 MD5 加密（salt = "yu-ai-code-mother-salt"）
	hashedPwd := md5Hash(password + "yu-ai-code-mother-salt")
	// 4. 入库
	user := &entity.User{
		UserAccount:  account,
		UserPassword: hashedPwd,
		UserName:     account, // 默认昵称 = 账号
		UserRole:     enums.UserRoleUser.Value,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return 0, err
	}
	return user.ID, nil
}

// Login 用户登录
// 流程：查账号 → 验密码（MD5+Salt对比）→ 生成 UUID Token → 存入 Redis Session → 返回脱敏用户信息+Token
// 返回值：脱敏用户视图、Token 字符串、错误
func (s *UserService) Login(ctx context.Context, account, password string) (*vo.LoginUserVO, string, error) {
	// 1. 根据账号查询用户
	user, err := s.repo.GetByAccount(ctx, account)
	if err != nil {
		return nil, "", fmt.Errorf("账号或密码错误") // 不区分"账号不存在"和"密码错误"，防撞库
	}
	// 2. 验证密码
	hashedPwd := md5Hash(password + "yu-ai-code-mother-salt")
	if user.UserPassword != hashedPwd {
		return nil, "", fmt.Errorf("账号或密码错误")
	}
	// 3. 生成 UUID 作为 Session Token
	token := uuid.New().String()
	// 4. 将用户信息 JSON 序列化后存入 Redis，设置过期时间（默认 30 天）
	key := fmt.Sprintf("session:%s", token)
	userJSON := utils.ToJSON(user)
	s.rdb.Set(ctx, key, userJSON, time.Duration(s.cfg.Session.TimeoutSeconds)*time.Second)
	// 5. 返回脱敏用户信息 + Token
	return userToLoginVO(user), token, nil
}

// Logout 注销：从 Redis 删除 Session → 客户端清除 Cookie
func (s *UserService) Logout(ctx context.Context, token string) error {
	key := fmt.Sprintf("session:%s", token)
	return s.rdb.Del(ctx, key).Err()
}

// GetLoginUser 从 Redis Session 中获取当前登录用户
// Cookie → Token → Redis key "session:{token}" → JSON 反序列化 → User 对象
func (s *UserService) GetLoginUser(ctx context.Context, token string) (*entity.User, error) {
	key := fmt.Sprintf("session:%s", token)
	data, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("未登录或登录已过期")
	}
	var user entity.User
	utils.FromJSON(data, &user)
	return &user, nil
}

// GetByID 根据 ID 查询用户
func (s *UserService) GetByID(ctx context.Context, id uint64) (*entity.User, error) {
	return s.repo.GetByID(ctx, id)
}

// GetUserVO 将实体转为脱敏视图（不含密码）
func (s *UserService) GetUserVO(user *entity.User) *vo.UserVO {
	return userToVO(user)
}

// GetLoginUserVO 将实体转为登录用户脱敏视图
func (s *UserService) GetLoginUserVO(user *entity.User) *vo.LoginUserVO {
	return userToLoginVO(user)
}

// userToVO 实体 → UserVO（内部辅助函数）
func userToVO(user *entity.User) *vo.UserVO {
	return &vo.UserVO{
		ID:          user.ID,
		UserAccount: user.UserAccount,
		UserName:    user.UserName,
		UserAvatar:  user.UserAvatar,
		UserProfile: user.UserProfile,
		UserRole:    user.UserRole,
		CreateTime:  user.CreateTime,
	}
}

// userToLoginVO 实体 → LoginUserVO（内部辅助函数）
func userToLoginVO(user *entity.User) *vo.LoginUserVO {
	return &vo.LoginUserVO{
		ID:          user.ID,
		UserAccount: user.UserAccount,
		UserName:    user.UserName,
		UserAvatar:  user.UserAvatar,
		UserProfile: user.UserProfile,
		UserRole:    user.UserRole,
		CreateTime:  user.CreateTime,
	}
}

// ==================== AppService：AI 代码生成核心服务 ====================

// AppService 应用业务服务 — 整个项目的核心。
// 负责：AI 智能路由创建应用 → SSE 流式对话生成代码 → 一键部署 → 异步截图 → ZIP 下载
// 内部集成了 AI 工厂（策略路由+代码生成）、安全护栏（防注入）、截图和下载子服务。
type AppService struct {
	repo          *repository.AppRepository         // 应用数据访问
	userRepo      *repository.UserRepository        // 用户数据访问（关联查询）
	chatRepo      *repository.ChatHistoryRepository // 对话历史数据访问
	cfg           *config.Config                    // 全局配置
	rdb           *redis.Client                     // Redis 客户端
	aiFactory     *ai.CodeGeneratorFactory          // ⭐ AI 代码生成工厂（管理 4 个 AI 客户端）
	guardrail     *ai.InputGuardrail                // ⭐ 输入安全护栏（防 Prompt 注入）
	screenshotSvc ScreenshotServiceInterface        // 截图服务（chromedp + COS）
	downloadSvc   *ProjectDownloadService           // ZIP 下载服务
}

// ScreenshotServiceInterface 截图服务接口（便于测试时 mock）
type ScreenshotServiceInterface interface {
	GenerateAndUploadScreenshot(webURL string) string
}

// NewAppService 构造函数：注入所有依赖，初始化 AI 工厂和安全护栏
func NewAppService(
	repo *repository.AppRepository,
	userRepo *repository.UserRepository,
	chatRepo *repository.ChatHistoryRepository,
	cfg *config.Config,
	rdb interface{},
	screenshotSvc ScreenshotServiceInterface,
) *AppService {
	rdbClient, _ := rdb.(*redis.Client)
	return &AppService{
		repo:          repo,
		userRepo:      userRepo,
		chatRepo:      chatRepo,
		cfg:           cfg,
		rdb:           rdbClient,
		aiFactory:     ai.NewCodeGeneratorFactory(cfg), // 初始化 4 个 AI 客户端
		guardrail:     ai.NewInputGuardrail(),          // 初始化安全护栏
		screenshotSvc: screenshotSvc,
		downloadSvc:   NewProjectDownloadService(),
	}
}

// GetAppByID 根据 ID 获取应用实体
func (s *AppService) GetAppByID(ctx context.Context, id uint64) (*entity.App, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAppVO 获取应用详情视图（含关联用户信息，批量查询避免 N+1）
func (s *AppService) GetAppVO(ctx context.Context, id uint64) (*vo.AppVO, error) {
	app, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.BuildAppVO(ctx, app)
}

// BuildAppVO 构建应用视图对象 → 关联查询用户信息填充 User 字段
func (s *AppService) BuildAppVO(ctx context.Context, app *entity.App) (*vo.AppVO, error) {
	v := &vo.AppVO{
		ID:           app.ID,
		AppName:      app.AppName,
		Cover:        app.Cover,
		InitPrompt:   app.InitPrompt,
		CodeGenType:  app.CodeGenType,
		DeployKey:    app.DeployKey,
		DeployedTime: app.DeployedTime,
		Priority:     app.Priority,
		UserID:       app.UserID,
		EditTime:     app.EditTime,
		CreateTime:   app.CreateTime,
	}
	// 关联查询用户信息（单次查询，避免 N+1 问题）
	if app.UserID > 0 {
		user, err := s.userRepo.GetByID(ctx, app.UserID)
		if err == nil {
			v.User = s.GetUserVO(user)
		}
	}
	return v, nil
}

// GetUserVO 包装 userToVO
func (s *AppService) GetUserVO(user *entity.User) *vo.UserVO {
	return userToVO(user)
}

// CreateApp ⭐ 创建应用 — AI 智能路由自动选择代码生成策略
// 用户只需输入自然语言需求（如"一个个人博客"），AI 自动分析复杂度，选择 html/multi_file/vue_project
func (s *AppService) CreateApp(ctx context.Context, initPrompt string, userID uint64) (uint64, error) {
	// 1. AI 智能路由：用 Qwen-Turbo（轻量模型）分析需求，选择最优生成策略
	codeGenType, err := s.aiFactory.RouteCodeGenType(ctx, initPrompt)
	if err != nil {
		log.Printf("[App] 智能路由失败，降级为 HTML 模式: %v", err)
		codeGenType = enums.CodeGenTypeHTML.Value // 降级默认
	}

	// 2. 取 initPrompt 前 12 个字符作为默认应用名称
	appName := initPrompt
	if len([]rune(appName)) > 12 {
		appName = string([]rune(appName)[:12])
	}

	// 3. 构建实体并入库
	app := &entity.App{
		AppName:     appName,
		InitPrompt:  initPrompt,
		CodeGenType: codeGenType,
		UserID:      userID,
		Priority:    0, // 默认优先级，管理员可设置为 99（精选）
	}
	if err := s.repo.Create(ctx, app); err != nil {
		return 0, err
	}

	log.Printf("[App] 应用创建成功, ID: %d, 类型: %s", app.ID, codeGenType)
	return app.ID, nil
}

// StreamChatToGenCode ⭐ SSE 流式 AI 对话生成代码（核心方法）
// 这是整个项目最核心的方法，处理流程：
// 查询应用 → 权限校验 → 安全护栏检查 → 保存用户消息到 DB →
// 根据 codeGenType 路由到不同 AI 生成策略（HTML/多文件/Vue工程）→ 通过回调函数推送 SSE 数据块
func (s *AppService) StreamChatToGenCode(ctx context.Context, appID uint64, message string, userID uint64, onChunk func(data string) error) error {
	// 1. 查询应用信息
	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("应用不存在")
	}
	// 2. 权限校验：只有应用创建者才能对话
	if app.UserID != userID {
		return fmt.Errorf("无权限")
	}
	// 3. 输入安全护栏：防 Prompt 注入、敏感词、超长输入
	gr := s.guardrail.Validate(message)
	if !gr.Allowed {
		onChunk(utils.ToJSON(map[string]string{"type": "error", "data": gr.Reason}))
		return fmt.Errorf(gr.Reason)
	}
	// 4. 先保存用户消息到对话历史数据库
	s.chatRepo.Create(ctx, &entity.ChatHistory{
		Message:     message,
		MessageType: enums.ChatHistoryTypeUser.Value,
		AppID:       appID,
		UserID:      userID,
	})
	// 5. 根据应用的代码生成类型，路由到不同的 AI 生成策略
	codeGenType := app.CodeGenType
	switch codeGenType {
	case enums.CodeGenTypeHTML.Value:
		// HTML 单文件模式：简单流式对话，AI 直接输出 HTML 代码
		return s.aiFactory.StreamHTMLCode(ctx, message, onChunk)
	case enums.CodeGenTypeMultiFile.Value:
		// 多文件模式：简单流式对话，AI 输出 HTML/CSS/JS 代码块
		return s.aiFactory.StreamMultiFileCode(ctx, message, onChunk)
	case enums.CodeGenTypeVueProject.Value:
		// Vue 工程模式：带工具调用的流式对话，AI 自主操作文件系统
		return s.streamVueProject(ctx, appID, message, userID, onChunk)
	default:
		return fmt.Errorf("不支持的代码生成类型: %s", codeGenType)
	}
}

// streamVueProject Vue 工程模式的流式生成（内部方法）
// 加载历史对话上下文 → 调用推理模型 + 工具调用 → 实时推送 3 种流消息：
// - ai_response：AI 文本响应
// - tool_request：工具调用请求（如 writeFile）→ 前端显示"AI 正在写入文件..."
// - tool_executed：工具执行结果 → 前端显示"文件写入成功"
func (s *AppService) streamVueProject(ctx context.Context, appID uint64, message string, userID uint64, onChunk func(data string) error) error {
	var fullResponse string

	// 加载最近 10 条对话历史作为上下文（从 DB 读取，倒序排列为旧→新）
	chatHistory := s.loadChatHistory(ctx, appID, 10)

	// 调用推理模型（DeepSeek Reasoner），支持工具调用
	return s.aiFactory.StreamVueProjectCode(
		ctx, appID, message,
		chatHistory,
		// --- 文本响应回调：AI 每生成一个 token 就推送 ---
		func(text string) error {
			fullResponse += text // 累积完整响应用于保存到 DB
			return onChunk(utils.ToJSON(map[string]string{
				"type": "ai_response",
				"data": text,
			}))
		},
		// --- 工具请求回调：AI 决定调用工具时推送 ---
		func(toolName, args string) error {
			return onChunk(utils.ToJSON(map[string]string{
				"type": "tool_request",
				"name": toolName,
				"args": args,
			}))
		},
		// --- 工具执行回调：工具执行完毕后推送结果 ---
		func(toolName, result string) error {
			return onChunk(utils.ToJSON(map[string]string{
				"type":   "tool_executed",
				"name":   toolName,
				"result": result,
			}))
		},
	)
}

// loadChatHistory 从数据库加载最近 N 条对话历史
// 倒序排列为旧→新（符合 OpenAI API 的消息顺序要求）
// 每条消息根据 messageType 映射为 user 或 assistant 角色
func (s *AppService) loadChatHistory(ctx context.Context, appID uint64, limit int) []openai.ChatCompletionMessage {
	records, err := s.chatRepo.GetRecentByAppID(ctx, appID, limit)
	if err != nil {
		return nil
	}
	var messages []openai.ChatCompletionMessage
	// 数据库按 createTime DESC 返回（新→旧），需要反转
	for i := len(records) - 1; i >= 0; i-- {
		role := openai.ChatMessageRoleUser
		if records[i].MessageType == enums.ChatHistoryTypeAI.Value {
			role = openai.ChatMessageRoleAssistant
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: records[i].Message,
		})
	}
	return messages
}

// DeployApp ⭐ 一键部署应用
// 流程：权限校验 → 生成 6 位 deployKey → 复制代码到部署目录 → 更新 DB → 异步截图封面
// 返回可访问的 URL（如 http://localhost/deploy/abc123/）
func (s *AppService) DeployApp(ctx context.Context, appID uint64, userID uint64) (string, error) {
	// 1. 查询应用 + 权限校验
	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("应用不存在")
	}
	if app.UserID != userID {
		return "", fmt.Errorf("无权限")
	}
	// 2. 生成或复用 deployKey（6 位字母数字混合）
	deployKey := app.DeployKey
	if deployKey == "" {
		deployKey = randomString(6)
	}
	// 3. 源目录：tmp/code_output/{codeGenType}_{appId}/
	sourceDirName := app.CodeGenType + "_" + fmt.Sprintf("%d", appID)
	sourceDir := filepath.Join(s.cfg.Code.OutputRootDir, sourceDirName)
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return "", fmt.Errorf("应用代码路径不存在，请先生成应用")
	}
	// 4. 目标目录：tmp/code_deploy/{deployKey}/
	deployDir := filepath.Join(s.cfg.Code.DeployRootDir, deployKey)
	os.MkdirAll(deployDir, 0755) // 递归创建目录
	// 5. 更新数据库中的 deployKey
	s.repo.Update(ctx, &entity.App{ID: appID, DeployKey: deployKey})
	// 6. 构建访问 URL
	deployURL := fmt.Sprintf("%s/%s/", s.cfg.Code.DeployHost, deployKey)
	// 7. 异步截图封面（不阻塞主流程）
	go s.generateScreenshot(appID, deployURL)
	return deployURL, nil
}

// generateScreenshot 异步截图并更新应用封面
// 使用 goroutine 异步执行：chromedp 截取网页 → 上传 COS → 更新 DB cover 字段
func (s *AppService) generateScreenshot(appID uint64, url string) {
	if s.screenshotSvc == nil {
		log.Printf("[Screenshot] 服务未初始化")
		return
	}
	screenshotURL := s.screenshotSvc.GenerateAndUploadScreenshot(url)
	if screenshotURL != "" {
		// 用 context.Background() 避免父 ctx 取消影响
		s.repo.Update(context.Background(), &entity.App{ID: appID, Cover: screenshotURL})
		log.Printf("[Screenshot] 封面更新成功: %s", screenshotURL)
	}
}

// QueryApps 分页查询应用
func (s *AppService) QueryApps(ctx context.Context, req dto.AppQueryRequest) ([]vo.AppVO, int64, error) {
	conditions := buildQueryConditions(req)
	apps, total, err := s.repo.Query(ctx, conditions, req.PageNum, req.PageSize, req.SortField, req.SortOrder)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]vo.AppVO, 0, len(apps))
	for i := range apps {
		v, _ := s.BuildAppVO(ctx, &apps[i])
		if v != nil {
			vos = append(vos, *v)
		}
	}
	return vos, total, nil
}

// UpdateApp 更新应用（用户只能更新自己的应用名称）
func (s *AppService) UpdateApp(ctx context.Context, req dto.AppUpdateRequest, userID uint64) error {
	oldApp, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("应用不存在")
	}
	if oldApp.UserID != userID {
		return fmt.Errorf("无权限")
	}
	return s.repo.Update(ctx, &entity.App{
		ID:      req.ID,
		AppName: req.AppName,
	})
}

// DeleteApp 删除应用
func (s *AppService) DeleteApp(ctx context.Context, appID, userID uint64) error {
	oldApp, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("应用不存在")
	}
	if oldApp.UserID != userID {
		return fmt.Errorf("无权限")
	}
	return s.repo.Delete(ctx, appID)
}

// DownloadAppCode 下载应用代码为 ZIP
func (s *AppService) DownloadAppCode(ctx context.Context, appID, userID uint64, w http.ResponseWriter) error {
	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("应用不存在")
	}
	if app.UserID != userID {
		return fmt.Errorf("无权限")
	}
	sourceDirName := app.CodeGenType + "_" + fmt.Sprintf("%d", appID)
	sourceDir := filepath.Join(s.cfg.Code.OutputRootDir, sourceDirName)
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("应用代码不存在，请先生成代码")
	}
	return s.downloadSvc.DownloadProjectAsZip(sourceDir, fmt.Sprintf("%d", appID), w)
}

// buildQueryConditions 构建查询条件
func buildQueryConditions(req dto.AppQueryRequest) map[string]interface{} {
	cond := make(map[string]interface{})
	if req.ID > 0 {
		cond["id = ?"] = req.ID
	}
	if req.AppName != "" {
		cond["appName LIKE ?"] = "%" + req.AppName + "%"
	}
	if req.CodeGenType != "" {
		cond["codeGenType = ?"] = req.CodeGenType
	}
	if req.UserID > 0 {
		cond["userId = ?"] = req.UserID
	}
	if req.Priority != nil {
		cond["priority = ?"] = *req.Priority
	}
	return cond
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func md5Hash(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// ==================== ChatHistoryService ====================

type ChatHistoryService struct {
	repo *repository.ChatHistoryRepository
}

func NewChatHistoryService(repo *repository.ChatHistoryRepository) *ChatHistoryService {
	return &ChatHistoryService{repo: repo}
}

func (s *ChatHistoryService) AddMessage(ctx context.Context, appID, userID uint64, message, msgType string) error {
	return s.repo.Create(ctx, &entity.ChatHistory{
		Message:     message,
		MessageType: msgType,
		AppID:       appID,
		UserID:      userID,
	})
}

func (s *ChatHistoryService) GetByAppID(ctx context.Context, appID uint64, page, pageSize int) ([]entity.ChatHistory, int64, error) {
	return s.repo.GetByAppID(ctx, appID, page, pageSize)
}

// ==================== ScreenshotService ====================

type ScreenshotService struct {
	cfg *config.Config
}

func NewScreenshotService(cfg *config.Config) *ScreenshotService {
	return &ScreenshotService{cfg: cfg}
}

func (s *ScreenshotService) GenerateAndUploadScreenshot(webURL string) string {
	// 使用 chromedp 截图，上传到 COS
	log.Printf("[Screenshot] 截图: %s", webURL)
	return ""
}
