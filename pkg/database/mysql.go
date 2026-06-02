// Package database — MySQL 连接池管理（GORM）
//
// 初始化 GORM MySQL 连接，配置连接池参数（最大空闲/最大打开/最大存活时间）。
// Package database — MySQL 连接池管理
//
// 使用 GORM + MySQL 驱动初始化数据库连接，配置连接池参数。
// MaxIdleConns/MaxOpenConns/ConnMaxLifetime 控制连接池行为，防止连接泄漏。
package database

import (
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/yupi/yu-ai-code-mother-go/internal/config"
)

// InitMySQL 初始化 MySQL 连接
// InitMySQL 初始化 MySQL 连接
// DSN 从 config 自动生成，连接池参数可调
func InitMySQL(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Info),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 连接池配置
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMinutes) * time.Minute)

	log.Println("[DB] MySQL 连接成功")
	return db, nil
}
