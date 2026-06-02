// Package repository — 数据访问层（GORM）
//
// 使用 GORM 封装 MySQL 操作：UserRepository、AppRepository、ChatHistoryRepository。
// ChatHistoryRepository 额外提供游标分页查询（WHERE id < cursorID），避免深分页性能问题。
// Package repository — 数据访问层（GORM）
//
// 使用 GORM 封装所有 MySQL 操作。
// UserRepository：用户 CRUD，含账号唯一性查询
// AppRepository：应用 CRUD，支持多条件组合查询
// ChatHistoryRepository：对话历史，额外提供游标分页（WHERE id < cursorID），避免深分页性能问题
package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/yupi/yu-ai-code-mother-go/internal/model/entity"
)

// ==================== UserRepository：用户数据访问 ====================

// UserRepository 用户数据访问层
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create 创建用户
func (r *UserRepository) Create(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) GetByID(ctx context.Context, id uint64) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("id = ? AND isDelete = 0", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByAccount 根据账号查询用户（用于登录/注册时的唯一性校验）
func (r *UserRepository) GetByAccount(ctx context.Context, account string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("userAccount = ? AND isDelete = 0", account).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListByIDs(ctx context.Context, ids []uint64) ([]entity.User, error) {
	var users []entity.User
	err := r.db.WithContext(ctx).Where("id IN ? AND isDelete = 0", ids).Find(&users).Error
	return users, err
}

func (r *UserRepository) Update(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).Where("id = ?", user.ID).Updates(user).Error
}

func (r *UserRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).Where("id = ?", id).Update("isDelete", 1).Error
}

func (r *UserRepository) Query(ctx context.Context, conditions map[string]interface{}, page, pageSize int, sortField, sortOrder string) ([]entity.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.User{}).Where("isDelete = 0")
	for k, v := range conditions {
		query = query.Where(k, v)
	}

	var total int64
	query.Count(&total)

	if sortField != "" {
		if sortOrder == "desc" {
			sortField += " DESC"
		}
		query = query.Order(sortField)
	} else {
		query = query.Order("createTime DESC")
	}

	offset := (page - 1) * pageSize
	var users []entity.User
	err := query.Offset(offset).Limit(pageSize).Find(&users).Error
	return users, total, err
}

// ==================== AppRepository ====================

// AppRepository 应用数据访问层
type AppRepository struct {
	db *gorm.DB
}

func NewAppRepository(db *gorm.DB) *AppRepository {
	return &AppRepository{db: db}
}

func (r *AppRepository) Create(ctx context.Context, app *entity.App) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *AppRepository) GetByID(ctx context.Context, id uint64) (*entity.App, error) {
	var app entity.App
	err := r.db.WithContext(ctx).Where("id = ? AND isDelete = 0", id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *AppRepository) Update(ctx context.Context, app *entity.App) error {
	return r.db.WithContext(ctx).Model(&entity.App{}).Where("id = ?", app.ID).Updates(app).Error
}

func (r *AppRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&entity.App{}).Where("id = ?", id).Update("isDelete", 1).Error
}

func (r *AppRepository) Query(ctx context.Context, conditions map[string]interface{}, page, pageSize int, sortField, sortOrder string) ([]entity.App, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.App{}).Where("isDelete = 0")
	for k, v := range conditions {
		query = query.Where(k, v)
	}

	var total int64
	query.Count(&total)

	if sortField != "" {
		if sortOrder == "desc" {
			sortField += " DESC"
		}
		query = query.Order(sortField)
	} else {
		query = query.Order("createTime DESC")
	}

	offset := (page - 1) * pageSize
	var apps []entity.App
	err := query.Offset(offset).Limit(pageSize).Find(&apps).Error
	return apps, total, err
}

// ==================== ChatHistoryRepository ====================

// ==================== ChatHistoryRepository：对话历史数据访问 ====================

// ChatHistoryRepository 对话历史数据访问层
// 额外提供游标分页查询，避免传统 OFFSET 在大数据量下的性能退化
type ChatHistoryRepository struct {
	db *gorm.DB
}

func NewChatHistoryRepository(db *gorm.DB) *ChatHistoryRepository {
	return &ChatHistoryRepository{db: db}
}

func (r *ChatHistoryRepository) Create(ctx context.Context, ch *entity.ChatHistory) error {
	return r.db.WithContext(ctx).Create(ch).Error
}

func (r *ChatHistoryRepository) GetByAppID(ctx context.Context, appID uint64, page, pageSize int) ([]entity.ChatHistory, int64, error) {
	var list []entity.ChatHistory
	var total int64
	query := r.db.WithContext(ctx).Model(&entity.ChatHistory{}).Where("appId = ? AND isDelete = 0", appID)
	query.Count(&total)
	err := query.Order("createTime DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// GetByAppIDWithCursor 游标分页查询（比 offset 更高效）
// GetByAppIDWithCursor ⭐ 游标分页查询 — 比 OFFSET 更高效
// 原理：WHERE id < cursorID ORDER BY id DESC LIMIT pageSize
// 优势：无论翻到第几页，查询复杂度都是 O(1)，不受数据总量影响
// 适用场景：对话历史的"加载更多"无限滚动
func (r *ChatHistoryRepository) GetByAppIDWithCursor(ctx context.Context, appID uint64, cursorID uint64, pageSize int) ([]entity.ChatHistory, error) {
	var list []entity.ChatHistory
	query := r.db.WithContext(ctx).Model(&entity.ChatHistory{}).
		Where("appId = ? AND isDelete = 0", appID)
	if cursorID > 0 {
		query = query.Where("id < ?", cursorID)
	}
	err := query.Order("id DESC").Limit(pageSize).Find(&list).Error
	return list, err
}

// GetRecentByAppID 获取最近 N 条对话记录
// GetRecentByAppID 获取指定应用最近 N 条对话记录（用于加载 AI 上下文）
func (r *ChatHistoryRepository) GetRecentByAppID(ctx context.Context, appID uint64, limit int) ([]entity.ChatHistory, error) {
	var list []entity.ChatHistory
	err := r.db.WithContext(ctx).Model(&entity.ChatHistory{}).
		Where("appId = ? AND isDelete = 0", appID).
		Order("createTime DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}
