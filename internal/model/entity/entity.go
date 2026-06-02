// Package entity — 数据库实体定义（GORM）
//
// User：用户表（账号/密码/角色/头像）
// App：应用表（名称/封面/生成类型/部署密钥/优先级）
// ChatHistory：对话历史表（消息/类型/关联应用/游标分页支持）
// Package entity — 数据库实体定义（GORM 映射）
//
// User：用户表
// App：应用表
// ChatHistory：对话历史表
// 所有实体均使用 GORM tag 指定列名、索引、唯一约束
package entity

import (
	"time"
)

// User 用户实体
// User 用户实体 — 映射 user 表
// 密码字段使用 json:"-" 标记，序列化时自动忽略（防泄露）
type User struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserAccount  string    `gorm:"column:userAccount;size:256;not null;uniqueIndex:uk_userAccount" json:"userAccount"` // 账号（唯一）
	UserPassword string    `gorm:"column:userPassword;size:512;not null" json:"-"`                                     // 密码 MD5+Salt，json:"-" 永不返回
	UserName     string    `gorm:"column:userName;size:256" json:"userName"`                                            // 昵称
	UserAvatar   string    `gorm:"column:userAvatar;size:1024" json:"userAvatar"`                                      // 头像 URL
	UserProfile  string    `gorm:"column:userProfile;size:512" json:"userProfile"`                                      // 个人简介
	UserRole     string    `gorm:"column:userRole;size:256;default:user" json:"userRole"`                               // 角色：user / admin
	EditTime     time.Time `gorm:"column:editTime;autoCreateTime" json:"editTime"`
	CreateTime   time.Time `gorm:"column:createTime;autoCreateTime" json:"createTime"`
	UpdateTime   time.Time `gorm:"column:updateTime;autoUpdateTime" json:"updateTime"`
	IsDelete     int8      `gorm:"column:isDelete;default:0" json:"-"`                                                  // 软删除标记
}

func (User) TableName() string {
	return "user"
}

// App 应用实体
// App 应用实体 — 映射 app 表
// deployKey 字段唯一，用于部署后的 URL 路由
type App struct {
	ID           uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppName      string     `gorm:"column:appName;size:256" json:"appName"`                                            // 应用名称
	Cover        string     `gorm:"column:cover;size:512" json:"cover"`                                                // 封面图 URL（部署后截图）
	InitPrompt   string     `gorm:"column:initPrompt;type:text" json:"initPrompt"`                                     // 初始提示词（用户创建时的需求描述）
	CodeGenType  string     `gorm:"column:codeGenType;size:64" json:"codeGenType"`                                     // 代码生成类型：html / multi_file / vue_project
	DeployKey    string     `gorm:"column:deployKey;size:64;uniqueIndex:uk_deployKey" json:"deployKey"`                // 部署密钥（6位随机字符串，用于 URL 路由）
	DeployedTime *time.Time `gorm:"column:deployedTime" json:"deployedTime"`                                           // 部署时间
	Priority     int        `gorm:"column:priority;default:0" json:"priority"`                                         // 优先级（99=精选展示在首页）
	UserID       uint64     `gorm:"column:userId;not null;index:idx_userId" json:"userId"`                             // 创建者 ID
	EditTime     time.Time  `gorm:"column:editTime;autoCreateTime" json:"editTime"`
	CreateTime   time.Time  `gorm:"column:createTime;autoCreateTime" json:"createTime"`
	UpdateTime   time.Time  `gorm:"column:updateTime;autoUpdateTime" json:"updateTime"`
	IsDelete     int8       `gorm:"column:isDelete;default:0" json:"-"`
}

func (App) TableName() string {
	return "app"
}

// ChatHistory 对话历史实体
// ChatHistory 对话历史实体 — 映射 chat_history 表
// 联合索引 idx_appId_createTime 用于游标分页和按时间查询
type ChatHistory struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Message     string    `gorm:"column:message;type:text;not null" json:"message"`                                     // 消息内容
	MessageType string    `gorm:"column:messageType;size:32;not null" json:"messageType"`                               // 类型：user / ai
	AppID       uint64    `gorm:"column:appId;not null;index:idx_appId;index:idx_appId_createTime" json:"appId"`        // 所属应用 ID
	UserID      uint64    `gorm:"column:userId;not null" json:"userId"`                                                 // 创建者 ID
	CreateTime  time.Time `gorm:"column:createTime;autoCreateTime;index:idx_createTime;index:idx_appId_createTime" json:"createTime"`
	UpdateTime  time.Time `gorm:"column:updateTime;autoUpdateTime" json:"updateTime"`
	IsDelete    int8      `gorm:"column:isDelete;default:0" json:"-"`
}

func (ChatHistory) TableName() string {
	return "chat_history"
}
