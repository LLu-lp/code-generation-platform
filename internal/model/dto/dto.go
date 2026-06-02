// Package dto — 数据传输对象（请求/响应）
//
// 定义所有 API 接口的入参和出参结构体。
// BaseResponse：统一响应格式 {code, data, message}
// PageRequest/PageResponse：分页请求/响应
package dto

// ==================== 通用 ====================

// BaseResponse 通用响应
type BaseResponse struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"message"`
}

// PageRequest 分页请求
type PageRequest struct {
	PageNum   int    `json:"pageNum" form:"pageNum"`
	PageSize  int    `json:"pageSize" form:"pageSize"`
	SortField string `json:"sortField" form:"sortField"`
	SortOrder string `json:"sortOrder" form:"sortOrder"` // asc / desc
}

// PageResponse 分页响应
type PageResponse struct {
	Records  interface{} `json:"records"`
	TotalRow int64       `json:"totalRow"`
	PageNum  int         `json:"pageNum"`
	PageSize int         `json:"pageSize"`
}

// DeleteRequest 删除请求
type DeleteRequest struct {
	ID uint64 `json:"id"`
}

// ==================== 用户 ====================

// UserRegisterRequest 注册请求
type UserRegisterRequest struct {
	UserAccount   string `json:"userAccount" binding:"required,min=4,max=32"`
	UserPassword  string `json:"userPassword" binding:"required,min=8,max=32"`
	CheckPassword string `json:"checkPassword" binding:"required,eqfield=UserPassword"`
}

// UserLoginRequest 登录请求
type UserLoginRequest struct {
	UserAccount  string `json:"userAccount" binding:"required"`
	UserPassword string `json:"userPassword" binding:"required"`
}

// UserAddRequest 添加用户（管理员）
type UserAddRequest struct {
	UserAccount string `json:"userAccount"`
	UserName    string `json:"userName"`
	UserRole    string `json:"userRole"`
}

// UserQueryRequest 用户查询请求
type UserQueryRequest struct {
	PageRequest
	ID          uint64 `json:"id" form:"id"`
	UserName    string `json:"userName" form:"userName"`
	UserAccount string `json:"userAccount" form:"userAccount"`
	UserRole    string `json:"userRole" form:"userRole"`
}

// ==================== 应用 ====================

// AppAddRequest 创建应用请求
type AppAddRequest struct {
	InitPrompt string `json:"initPrompt" binding:"required"`
}

// AppUpdateRequest 更新应用（用户）
type AppUpdateRequest struct {
	ID      uint64 `json:"id" binding:"required"`
	AppName string `json:"appName"`
}

// AppAdminUpdateRequest 管理员更新应用
type AppAdminUpdateRequest struct {
	ID         uint64 `json:"id" binding:"required"`
	AppName    string `json:"appName"`
	Cover      string `json:"cover"`
	InitPrompt string `json:"initPrompt"`
	CodeGenType string `json:"codeGenType"`
	Priority   int    `json:"priority"`
}

// AppQueryRequest 应用查询请求
type AppQueryRequest struct {
	PageRequest
	ID          uint64 `json:"id" form:"id"`
	AppName     string `json:"appName" form:"appName"`
	Cover       string `json:"cover" form:"cover"`
	InitPrompt  string `json:"initPrompt" form:"initPrompt"`
	CodeGenType string `json:"codeGenType" form:"codeGenType"`
	DeployKey   string `json:"deployKey" form:"deployKey"`
	Priority    *int   `json:"priority" form:"priority"`
	UserID      uint64 `json:"userId" form:"userId"`
}

// AppDeployRequest 部署请求
type AppDeployRequest struct {
	AppID uint64 `json:"appId" binding:"required"`
}

// AppChatRequest AI 对话请求
type AppChatRequest struct {
	AppID   uint64 `json:"appId" form:"appId" binding:"required"`
	Message string `json:"message" form:"message" binding:"required"`
}

// ==================== 对话历史 ====================

// ChatHistoryQueryRequest 对话历史查询
type ChatHistoryQueryRequest struct {
	PageRequest
	AppID    uint64 `json:"appId" form:"appId"`
	UserID   uint64 `json:"userId" form:"userId"`
	CursorID uint64 `json:"cursorId" form:"cursorId"` // 游标分页
}
