package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 数据库连接
type DB struct {
	*gorm.DB
}

// NewDB 创建数据库连接
func NewDB(config Database) (*DB, error) {
	var db *gorm.DB
	var err error

	switch config.Type {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.User, config.Password, config.Host, config.Port, config.Name)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	case "sqlite":
		fallthrough
	default:
		db, err = gorm.Open(sqlite.Open(config.DBPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	return &DB{db}, nil
}

// Migrate 执行数据库迁移
func (db *DB) Migrate() error {
	return db.AutoMigrate(&User{})
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
