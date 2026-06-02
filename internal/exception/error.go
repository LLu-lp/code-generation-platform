// Package exception — 业务异常与错误码
//
// 定义业务错误码常量（对齐 Java 版 ErrorCode）和 BizError 自定义异常类型。
// Package exception — 业务异常与错误码
//
// 定义标准业务错误码（对齐 HTTP 语义但独立于 HTTP 状态码）和 BizError 自定义异常类型。
// 所有接口统一返回 HTTP 200 + 业务 code（0=成功，40000+=客户端错误，50000+=服务端错误）
package exception

// ErrorCode 业务错误码（对齐 Java 版）
const (
	Success          = 0
	ParamsError      = 40000
	NotLoginError    = 40100
	NoAuthError      = 40200
	ForbiddenError   = 40300
	NotFoundError    = 40400
	SystemError      = 50000
	OperationError   = 50001
	RateLimitError   = 42900
)

// ErrorMsg 错误码对应消息
var ErrorMsg = map[int]string{
	Success:          "ok",
	ParamsError:      "请求参数错误",
	NotLoginError:    "未登录",
	NoAuthError:      "无权限",
	ForbiddenError:   "禁止访问",
	NotFoundError:    "请求数据不存在",
	SystemError:      "系统内部错误",
	OperationError:   "操作失败",
	RateLimitError:   "请求过于频繁，请稍后再试",
}

// GetMsg 获取错误消息
func GetMsg(code int) string {
	if msg, ok := ErrorMsg[code]; ok {
		return msg
	}
	return "未知错误"
}

// BizError 业务异常
// BizError 业务异常 — 携带错误码和错误消息
// 实现了 error 接口，可以像普通 error 一样使用
type BizError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *BizError) Error() string {
	return e.Message
}

// NewBizError 创建业务异常
func NewBizError(code int, message string) *BizError {
	return &BizError{Code: code, Message: message}
}

// NewBizErrorWithCode 创建业务异常（使用默认消息）
func NewBizErrorWithCode(code int) *BizError {
	return &BizError{Code: code, Message: GetMsg(code)}
}
