package database

import (
	"fmt"
	"os"
	"path/filepath"

	"nstock/internal/config"
	"nstock/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(cfg.DBPath) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.WatchItem{},
		&model.QuoteSnap{},
		&model.DailyBar{},
		&model.Prediction{},
		&model.Account{},
		&model.Position{},
		&model.Order{},
		&model.Strategy{},
		&model.Backtest{},
		&model.AlertRule{},
		&model.AlertEvent{},
		&model.User{},
		&model.Role{},
		&model.Permission{},
		&model.UserRole{},
		&model.RolePermission{},
		&model.Session{},
	)
}

// Schema 表示表结构已经迁移完成，供种子数据在灌库前等待。
type Schema struct{}

func Prepare(db *gorm.DB) (Schema, error) {
	if err := Migrate(db); err != nil {
		return Schema{}, err
	}
	return Schema{}, nil
}
