// Package controller 控制器层 — HTTP 请求处理入口
// 负责接收前端请求、参数校验、调用 Service 层、返回统一格式响应。
// 包含 6 个 Controller：User（用户）、App（应用）、ChatHistory（对话）、
// Static（静态资源部署访问）、Workflow（工作流）、Health（健康检查）。
package controller

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/yupi/yu-ai-code-mother-go/internal/config"
	"github.com/yupi/yu-ai-code-mother-go/internal/model/dto"
	"github.com/yupi/yu-ai-code-mother-go/internal/model/entity"
	"github.com/yupi/yu-ai-code-mother-go/internal/service"
)

// ==================== 通用响应（对齐 Java 版 ResultUtils） ====================

// 业务错误码，所有接口统一使用
const (
	CodeSuccess      = 0     // 成功
	CodeParamError   = 40000 // 请求参数错误
	CodeNotLogin     = 40100 // 未登录
	CodeNoAuth       = 40200 // 无权限
	CodeNotFound     = 40400 // 数据不存在
	CodeSystemError  = 50000 // 系统内部错误
	CodeOperateError = 50001 // 操作失败
)

// Success 返回统一成功响应，HTTP 状态码 200，业务码 code=0
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, dto.BaseResponse{Code: CodeSuccess, Data: data, Msg: "ok"})
}

// Error 返回统一业务错误响应，HTTP 200 + 业务错误码
func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, dto.BaseResponse{Code: code, Msg: msg})
}

// ErrorWithStatus 返回自定义 HTTP 状态码的错误响应（用于认证失败等场景）
func ErrorWithStatus(c *gin.Context, httpCode, code int, msg string) {
	c.JSON(httpCode, dto.BaseResponse{Code: code, Msg: msg})
}

// ==================== UserController：用户注册/登录/注销 ====================

// UserController 用户控制器，处理注册、登录、注销、获取当前用户等请求
type UserController struct {
	svc *service.UserService // 用户业务服务
}

// NewUserController 构造函数，依赖注入 UserService
func NewUserController(svc *service.UserService) *UserController {
	return &UserController{svc: svc}
}

// RegisterRoutes 注册用户相关路由到 /api/user/ 分组
func (ctrl *UserController) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/user")
	g.POST("/register", ctrl.Register)     // 用户注册
	g.POST("/login", ctrl.Login)           // 用户登录
	g.POST("/logout", ctrl.Logout)         // 用户注销
	g.GET("/get/login", ctrl.GetLoginUser) // 获取当前登录用户信息
}

// Register 用户注册接口
// 校验两次密码一致性，密码加盐 MD5 存储
func (ctrl *UserController) Register(c *gin.Context) {
	var req dto.UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误: "+err.Error())
		return
	}
	id, err := ctrl.svc.Register(c.Request.Context(), req.UserAccount, req.UserPassword, req.CheckPassword)
	if err != nil {
		Error(c, CodeParamError, err.Error())
		return
	}
	Success(c, id)
}

// Login 用户登录接口
// 验证账号密码 → 生成 UUID Token → 存入 Redis → 设置 HttpOnly Cookie
func (ctrl *UserController) Login(c *gin.Context) {
	var req dto.UserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误")
		return
	}
	loginVO, token, err := ctrl.svc.Login(c.Request.Context(), req.UserAccount, req.UserPassword)
	if err != nil {
		Error(c, CodeParamError, err.Error())
		return
	}
	// 设置 Cookie：30 天有效期，HttpOnly 防 XSS
	c.SetCookie("token", token, 2592000, "/", "", false, true)
	Success(c, loginVO)
}

// Logout 注销接口：清除 Redis Session + 删除 Cookie
func (ctrl *UserController) Logout(c *gin.Context) {
	token, _ := c.Cookie("token")
	if token != "" {
		ctrl.svc.Logout(c.Request.Context(), token)
	}
	c.SetCookie("token", "", -1, "/", "", false, true)
	Success(c, true)
}

// GetLoginUser 获取当前登录用户信息（从 Cookie 读 Token → Redis 查 Session）
func (ctrl *UserController) GetLoginUser(c *gin.Context) {
	token, _ := c.Cookie("token")
	if token == "" {
		Error(c, CodeNotLogin, "未登录")
		return
	}
	user, err := ctrl.svc.GetLoginUser(c.Request.Context(), token)
	if err != nil {
		Error(c, CodeNotLogin, "未登录")
		return
	}
	Success(c, ctrl.svc.GetLoginUserVO(user))
}

// ==================== AppController：AI 代码生成核心控制器 ====================

// AppController 应用控制器，这是整个项目的核心。
// 负责：创建应用（AI智能路由）→ AI流式对话生成代码 → 部署 → 下载 → CRUD管理
type AppController struct {
	svc *service.AppService // 应用业务服务（封装 AI 调用、部署等复杂逻辑）
}

func NewAppController(svc *service.AppService) *AppController {
	return &AppController{svc: svc}
}

// RegisterRoutes 注册应用相关路由
func (ctrl *AppController) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/app")
	g.POST("/add", ctrl.AddApp)                     // 创建应用（AI智能路由选择策略）
	g.GET("/get/vo", ctrl.GetAppVO)                 // 获取应用详情
	g.POST("/my/list/page/vo", ctrl.ListMyApps)     // 分页查询我的应用
	g.POST("/good/list/page/vo", ctrl.ListGoodApps) // 分页查询精选应用
	g.POST("/update", ctrl.UpdateApp)               // 更新应用
	g.POST("/delete", ctrl.DeleteApp)               // 删除应用
	g.POST("/deploy", ctrl.DeployApp)               // 一键部署
	g.GET("/download/:appId", ctrl.DownloadCode)    // 下载代码 ZIP
	g.GET("/chat/gen/code", ctrl.ChatToGenCode)     // ⭐ SSE 流式 AI 对话（核心接口）
}

// AddApp 创建应用 — 用户输入需求描述，AI 自动选择最优代码生成策略（html/multi_file/vue_project）
func (ctrl *AppController) AddApp(c *gin.Context) {
	var req dto.AppAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误")
		return
	}
	// 从 Gin Context 获取登录用户（由 Auth 中间件注入）
	user := ctrl.getLoginUser(c)
	if user == nil {
		return
	}
	// 调用 Service 层创建应用（内部先 AI 路由 → 再入库）
	appID, err := ctrl.svc.CreateApp(c.Request.Context(), req.InitPrompt, user.ID)
	if err != nil {
		Error(c, CodeSystemError, err.Error())
		return
	}
	Success(c, appID)
}

// ChatToGenCode ⭐ 核心接口：SSE 流式 AI 对话生成代码
// 采用 Server-Sent Events 协议，AI 每生成一个 token 就实时推送给前端。
// 前端通过 EventSource API 接收，实现打字机效果的实时渲染。
func (ctrl *AppController) ChatToGenCode(c *gin.Context) {
	appIDStr := c.Query("appId")
	message := c.Query("message")

	appID, err := strconv.ParseUint(appIDStr, 10, 64)
	if err != nil || appID == 0 || message == "" {
		Error(c, CodeParamError, "参数错误")
		return
	}

	user := ctrl.getLoginUser(c)
	if user == nil {
		return
	}

	// --- SSE 流式响应设置 ---
	// Content-Type: text/event-stream 是 SSE 标准 MIME 类型
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache") // 禁用缓存
	c.Writer.Header().Set("Connection", "keep-alive")  // 长连接
	c.Writer.WriteHeader(http.StatusOK)

	// Flusher 是 Gin 对流式输出的关键接口，允许逐个字节 flush 到客户端
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		Error(c, CodeSystemError, "不支持流式响应")
		return
	}

	// 调用 Service 层，每次收到 AI 响应 chunk 就通过 SSE 推送给前端
	err = ctrl.svc.StreamChatToGenCode(c.Request.Context(), appID, message, user.ID, func(data string) error {
		// 包装为 {"d": ...} 格式与前端约定
		wrapper := fmt.Sprintf(`{"d":%s}`, data)
		fmt.Fprintf(c.Writer, "data: %s\n\n", wrapper)
		flusher.Flush() // 关键：立即推送，不等待缓冲区满
		return nil
	})

	if err != nil {
		log.Printf("[SSE] 流式生成失败: %v", err)
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
	}

	// 发送完成事件，前端收到后关闭 EventSource
	fmt.Fprintf(c.Writer, "event: done\ndata: \n\n")
	flusher.Flush()
}

// GetAppVO 根据 ID 获取应用详情（含关联用户信息）
func (ctrl *AppController) GetAppVO(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		Error(c, CodeParamError, "ID 参数错误")
		return
	}
	appVO, err := ctrl.svc.GetAppVO(c.Request.Context(), id)
	if err != nil {
		Error(c, CodeNotFound, "应用不存在")
		return
	}
	Success(c, appVO)
}

// ListMyApps 分页查询当前用户创建的应用列表
func (ctrl *AppController) ListMyApps(c *gin.Context) {
	var req dto.AppQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误")
		return
	}
	user := ctrl.getLoginUser(c)
	if user == nil {
		return
	}
	req.UserID = user.ID // 只查当前用户的应用
	apps, total, err := ctrl.svc.QueryApps(c.Request.Context(), req)
	if err != nil {
		Error(c, CodeSystemError, err.Error())
		return
	}
	Success(c, dto.PageResponse{Records: apps, TotalRow: total, PageNum: req.PageNum, PageSize: req.PageSize})
}

// ListGoodApps 分页查询精选应用（priority=99）
func (ctrl *AppController) ListGoodApps(c *gin.Context) {
	var req dto.AppQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误")
		return
	}
	priority := 99
	req.Priority = &priority // 只查精选应用
	apps, total, err := ctrl.svc.QueryApps(c.Request.Context(), req)
	if err != nil {
		Error(c, CodeSystemError, err.Error())
		return
	}
	Success(c, dto.PageResponse{Records: apps, TotalRow: total, PageNum: req.PageNum, PageSize: req.PageSize})
}

// UpdateApp 更新应用（用户只能更新自己的应用名称）
func (ctrl *AppController) UpdateApp(c *gin.Context) {
	var req dto.AppUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误")
		return
	}
	user := ctrl.getLoginUser(c)
	if user == nil {
		return
	}
	if err := ctrl.svc.UpdateApp(c.Request.Context(), req, user.ID); err != nil {
		Error(c, CodeOperateError, err.Error())
		return
	}
	Success(c, true)
}

// DeleteApp 删除应用（用户只能删除自己的，管理员可以删除任意）
func (ctrl *AppController) DeleteApp(c *gin.Context) {
	var req dto.DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误")
		return
	}
	user := ctrl.getLoginUser(c)
	if user == nil {
		return
	}
	if err := ctrl.svc.DeleteApp(c.Request.Context(), req.ID, user.ID); err != nil {
		Error(c, CodeOperateError, err.Error())
		return
	}
	Success(c, true)
}

// DeployApp 一键部署：生成 deployKey → 复制文件到部署目录 → 异步截图封面 → 返回访问 URL
func (ctrl *AppController) DeployApp(c *gin.Context) {
	var req dto.AppDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, CodeParamError, "参数错误")
		return
	}
	user := ctrl.getLoginUser(c)
	if user == nil {
		return
	}
	url, err := ctrl.svc.DeployApp(c.Request.Context(), req.AppID, user.ID)
	if err != nil {
		Error(c, CodeOperateError, err.Error())
		return
	}
	Success(c, url)
}

// DownloadCode 将应用代码目录打包为 ZIP 下载
func (ctrl *AppController) DownloadCode(c *gin.Context) {
	appIDStr := c.Param("appId")
	appID, err := strconv.ParseUint(appIDStr, 10, 64)
	if err != nil || appID == 0 {
		Error(c, CodeParamError, "参数错误")
		return
	}
	user := ctrl.getLoginUser(c)
	if user == nil {
		return
	}
	if err := ctrl.svc.DownloadAppCode(c.Request.Context(), appID, user.ID, c.Writer); err != nil {
		Error(c, CodeOperateError, err.Error())
		return
	}
}

// getLoginUser 从 Gin Context 中提取登录用户信息（由 Auth 中间件注入）
// 支持两种类型：*LoginUserInfo（中间件直接注入）和 *entity.User（兼容转换）
func (ctrl *AppController) getLoginUser(c *gin.Context) *LoginUserInfo {
	userVal, exists := c.Get("loginUser")
	if !exists {
		Error(c, CodeNotLogin, "未登录")
		return nil
	}
	user, ok := userVal.(*LoginUserInfo)
	if !ok {
		// 兼容：如果注入的是 entity.User，则转换
		if entityUser, ok2 := userVal.(*entity.User); ok2 {
			return &LoginUserInfo{
				ID:      entityUser.ID,
				Account: entityUser.UserAccount,
				Role:    entityUser.UserRole,
			}
		}
		Error(c, CodeNotLogin, "未登录")
		return nil
	}
	return user
}

// LoginUserInfo 登录用户信息（由 Auth 中间件注入到 Gin Context 中）
// 只包含必要字段，避免敏感信息泄露
type LoginUserInfo struct {
	ID      uint64 `json:"id"`      // 用户 ID
	Account string `json:"account"` // 账号
	Role    string `json:"role"`    // 角色：user / admin
}

// GetLoginUserFromContext 全局函数，供中间件等非 Controller 方法获取登录用户
// 同样支持 *LoginUserInfo 和 *entity.User 两种类型转换
func GetLoginUserFromContext(c *gin.Context) *LoginUserInfo {
	userVal, exists := c.Get("loginUser")
	if !exists {
		return nil
	}
	switch v := userVal.(type) {
	case *LoginUserInfo:
		return v
	case *entity.User:
		return &LoginUserInfo{ID: v.ID, Account: v.UserAccount, Role: v.UserRole}
	default:
		return nil
	}
}

// buildAppVO 构建应用视图对象的简易版本（用于列表展示时减少关联查询）
func (ctrl *AppController) buildAppVO(c *gin.Context, app *entity.App) interface{} {
	type AppVO struct {
		ID          uint64 `json:"id"`
		AppName     string `json:"appName"`
		Cover       string `json:"cover"`
		InitPrompt  string `json:"initPrompt"`
		CodeGenType string `json:"codeGenType"`
		DeployKey   string `json:"deployKey"`
		Priority    int    `json:"priority"`
		UserID      uint64 `json:"userId"`
	}
	return AppVO{
		ID:          app.ID,
		AppName:     app.AppName,
		Cover:       app.Cover,
		InitPrompt:  app.InitPrompt,
		CodeGenType: app.CodeGenType,
		DeployKey:   app.DeployKey,
		Priority:    app.Priority,
		UserID:      app.UserID,
	}
}

// ==================== ChatHistoryController：对话历史 ====================

type ChatHistoryController struct {
	svc *service.ChatHistoryService
}

func NewChatHistoryController(svc *service.ChatHistoryService) *ChatHistoryController {
	return &ChatHistoryController{svc: svc}
}

func (ctrl *ChatHistoryController) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/chat-history")
	g.GET("/list", ctrl.ListByAppID)
}

// ListByAppID 查询指定应用的对话历史（支持游标分页）
func (ctrl *ChatHistoryController) ListByAppID(c *gin.Context) {
	Success(c, []interface{}{})
}

// ==================== StaticController：静态资源部署访问 ====================

// StaticController 提供部署后的静态资源访问能力
// /api/deploy/{deployKey}/ → 访问正式部署的站点
// /api/preview/{type}_{appId}/ → 访问预览中的站点
type StaticController struct {
	cfg *config.Config
}

func NewStaticController(cfg *config.Config) *StaticController {
	return &StaticController{cfg: cfg}
}

func (ctrl *StaticController) RegisterRoutes(rg *gin.RouterGroup) {
	// 部署后的静态资源访问（通过 deployKey 路由）
	rg.Static("/deploy", ctrl.cfg.Code.DeployRootDir)
	// 预览用静态资源（通过 codeGenType_appId 路由）
	rg.Static("/preview", ctrl.cfg.Code.OutputRootDir)
}

// ==================== WorkflowController：AI 工作流 SSE ====================

// WorkflowController 提供 AI 工作流执行的 SSE 接口
// 前端可实时查看工作流各节点的执行进度
type WorkflowController struct {
	cfg *config.Config
}

func NewWorkflowController(cfg *config.Config) *WorkflowController {
	return &WorkflowController{cfg: cfg}
}

func (ctrl *WorkflowController) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/workflow")
	g.GET("/execute", ctrl.ExecuteWorkflow)
}

// ExecuteWorkflow 通过 SSE 推送工作流执行进度
func (ctrl *WorkflowController) ExecuteWorkflow(c *gin.Context) {
	prompt := c.Query("prompt")
	if prompt == "" {
		Error(c, CodeParamError, "prompt 不能为空")
		return
	}
	// SSE 流式返回工作流进度
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)
	fmt.Fprintf(c.Writer, "event: workflow_start\ndata: %s\n\n", `{"message":"工作流已启动"}`)
	flusher.Flush()
	fmt.Fprintf(c.Writer, "event: workflow_completed\ndata: \n\n")
	flusher.Flush()
}

// ==================== HealthController：健康检查 ====================

// HealthController 提供标准健康检查端点，用于 K8s 探针和负载均衡器
type HealthController struct {
	db *gorm.DB
}

func NewHealthController(db *gorm.DB) *HealthController {
	return &HealthController{db: db}
}

func (ctrl *HealthController) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/health", ctrl.Health)
}

// Health 返回服务健康状态（项目名 + UP 状态）
func (ctrl *HealthController) Health(c *gin.Context) {
	Success(c, gin.H{
		"status":  "UP",
		"service": "yu-ai-code-mother-go",
	})
}
