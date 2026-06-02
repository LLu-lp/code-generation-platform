// Package vo — 视图对象（返回给前端的数据）
//
// LoginUserVO：登录用户脱敏视图（不含密码）
// UserVO：用户信息脱敏视图
// AppVO：应用详情视图（含关联用户信息）
package vo

import (
	"time"
)

// LoginUserVO 登录用户视图（脱敏）
type LoginUserVO struct {
	ID          uint64    `json:"id"`
	UserAccount string    `json:"userAccount"`
	UserName    string    `json:"userName"`
	UserAvatar  string    `json:"userAvatar"`
	UserProfile string    `json:"userProfile"`
	UserRole    string    `json:"userRole"`
	CreateTime  time.Time `json:"createTime"`
}

// UserVO 用户视图（脱敏）
type UserVO struct {
	ID          uint64    `json:"id"`
	UserAccount string    `json:"userAccount"`
	UserName    string    `json:"userName"`
	UserAvatar  string    `json:"userAvatar"`
	UserProfile string    `json:"userProfile"`
	UserRole    string    `json:"userRole"`
	CreateTime  time.Time `json:"createTime"`
}

// AppVO 应用视图
type AppVO struct {
	ID           uint64     `json:"id"`
	AppName      string     `json:"appName"`
	Cover        string     `json:"cover"`
	InitPrompt   string     `json:"initPrompt"`
	CodeGenType  string     `json:"codeGenType"`
	DeployKey    string     `json:"deployKey"`
	DeployedTime *time.Time `json:"deployedTime"`
	Priority     int        `json:"priority"`
	UserID       uint64     `json:"userId"`
	User         *UserVO    `json:"user"`
	EditTime     time.Time  `json:"editTime"`
	CreateTime   time.Time  `json:"createTime"`
}
