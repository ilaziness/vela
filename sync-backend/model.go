package main

import (
	"time"
)

// User 用户模型
type User struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	UserKey    string    `gorm:"size:128;not null" json:"-"`
	LastSyncAt time.Time `json:"last_sync_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

// BackupInfo is returned to the client for one stored zip.
type BackupInfo struct {
	Name      string    `json:"name"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}
