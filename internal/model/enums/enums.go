// Package enums — 枚举定义
//
// CodeGenTypeEnum：代码生成策略（html/multi_file/vue_project）
// UserRoleEnum：用户角色（user/admin）
// ChatHistoryMessageTypeEnum：对话消息类型（user/ai）
// StreamMessageTypeEnum：流式消息类型（ai_response/tool_request/tool_executed）
package enums

// CodeGenTypeEnum 代码生成类型
type CodeGenTypeEnum struct {
	Text  string
	Value string
}

var (
	CodeGenTypeHTML       = CodeGenTypeEnum{Text: "原生 HTML 模式", Value: "html"}
	CodeGenTypeMultiFile  = CodeGenTypeEnum{Text: "原生多文件模式", Value: "multi_file"}
	CodeGenTypeVueProject = CodeGenTypeEnum{Text: "Vue 工程模式", Value: "vue_project"}
)

// allCodeGenTypes 所有代码生成类型
var allCodeGenTypes = []CodeGenTypeEnum{
	CodeGenTypeHTML,
	CodeGenTypeMultiFile,
	CodeGenTypeVueProject,
}

// GetCodeGenTypeByValue 根据 value 获取枚举
func GetCodeGenTypeByValue(value string) *CodeGenTypeEnum {
	for _, t := range allCodeGenTypes {
		if t.Value == value {
			return &t
		}
	}
	return nil
}

// IsValidCodeGenType 验证
func IsValidCodeGenType(value string) bool {
	return GetCodeGenTypeByValue(value) != nil
}

// ========================

// UserRoleEnum 用户角色
type UserRoleEnum struct {
	Text  string
	Value string
}

var (
	UserRoleUser  = UserRoleEnum{Text: "普通用户", Value: "user"}
	UserRoleAdmin = UserRoleEnum{Text: "管理员", Value: "admin"}
)

// ========================

// ChatHistoryMessageTypeEnum 对话消息类型
type ChatHistoryMessageTypeEnum struct {
	Text  string
	Value string
}

var (
	ChatHistoryTypeUser = ChatHistoryMessageTypeEnum{Text: "用户", Value: "user"}
	ChatHistoryTypeAI   = ChatHistoryMessageTypeEnum{Text: "AI", Value: "ai"}
)

// ========================

// StreamMessageTypeEnum 流式消息类型
type StreamMessageTypeEnum struct {
	Text  string
	Value string
}

var (
	StreamMsgTypeAIResponse    = StreamMessageTypeEnum{Text: "AI 响应", Value: "ai_response"}
	StreamMsgTypeToolRequest   = StreamMessageTypeEnum{Text: "工具请求", Value: "tool_request"}
	StreamMsgTypeToolExecuted  = StreamMessageTypeEnum{Text: "工具执行结果", Value: "tool_executed"}
)
