package main

import (
	"time"
)

// User 用户模型
type User struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"` // sync_id (UUID)
	UserKey    string    `gorm:"size:128;not null" json:"-"`   // bcrypt hash of user_key
	LastSyncAt time.Time `json:"last_sync_at"`                 // 上次同步时间
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// SyncVersion 同步版本模型
type SyncVersion struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        string    `gorm:"size:36;not null;index" json:"user_id"`
	VersionNumber int       `json:"version_number"`
	FileSize      int64     `json:"file_size"`
	FilePath      string    `gorm:"size:255" json:"-"` // 数据文件路径（不在 JSON 中暴露）
	CreatedAt     time.Time `json:"created_at"`
}

// TableName 指定表名
func (SyncVersion) TableName() string {
	return "sync_versions"
}

// VersionInfo 版本信息（返回给客户端的结构）
type VersionInfo struct {
	VersionNumber int       `json:"version_number"`
	FileSize      int64     `json:"file_size"`
	CreatedAt     time.Time `json:"created_at"`
}
