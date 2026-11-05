package main

import (
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const userKeyBcryptCost = 12

// SyncService 同步服务
type SyncService struct {
	db      *DB
	storage *Storage
	maxVer  int
}

// NewSyncService 创建同步服务
func NewSyncService(db *DB, storage *Storage, maxVersions int) *SyncService {
	return &SyncService{
		db:      db,
		storage: storage,
		maxVer:  maxVersions,
	}
}

// CreateUser 创建用户（userKey 明文仅用于生成 bcrypt 哈希后丢弃）
func (s *SyncService) CreateUser(id, userKey string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(userKey), userKeyBcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash user key: %w", err)
	}
	user := &User{
		ID:        id,
		UserKey:   string(hash),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

// GetUser 获取用户
func (s *SyncService) GetUser(id string) (*User, error) {
	var user User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// VerifyUser 验证用户
func (s *SyncService) VerifyUser(id, userKey string) (*User, error) {
	user, err := s.GetUser(id)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.UserKey), []byte(userKey)); err != nil {
		return nil, fmt.Errorf("invalid user key")
	}
	return user, nil
}

// UpdateLastSyncAt 更新上次同步时间
func (s *SyncService) UpdateLastSyncAt(userID string) error {
	return s.db.Model(&User{}).Where("id = ?", userID).Update("last_sync_at", time.Now()).Error
}

// GetLastSyncAt 获取上次同步时间
func (s *SyncService) GetLastSyncAt(userID string) (time.Time, error) {
	var user User
	if err := s.db.Select("last_sync_at").First(&user, "id = ?", userID).Error; err != nil {
		return time.Time{}, err
	}
	return user.LastSyncAt, nil
}

// SaveVersion 保存版本
func (s *SyncService) SaveVersion(userID string, reader io.Reader) (*SyncVersion, error) {
	// 获取当前最大版本号
	var maxVersion int
	s.db.Model(&SyncVersion{}).Where("user_id = ?", userID).Select("COALESCE(MAX(version_number), 0)").Scan(&maxVersion)

	newVersion := maxVersion + 1

	// 保存文件
	filePath, fileSize, err := s.storage.Save(userID, newVersion, reader)
	if err != nil {
		return nil, err
	}

	// 创建版本记录
	version := &SyncVersion{
		UserID:        userID,
		VersionNumber: newVersion,
		FileSize:      fileSize,
		FilePath:      filePath,
		CreatedAt:     time.Now(),
	}
	if err := s.db.Create(version).Error; err != nil {
		// 清理文件
		_ = s.storage.Delete(userID, newVersion)
		return nil, fmt.Errorf("failed to create version record: %w", err)
	}

	// 清理旧版本
	if err := s.cleanupOldVersions(userID); err != nil {
		// 记录错误但不影响主流程
		fmt.Printf("failed to cleanup old versions: %v\n", err)
	}

	return version, nil
}

// GetVersion 获取版本
func (s *SyncService) GetVersion(userID string, versionNumber int) (*SyncVersion, error) {
	var version SyncVersion
	if err := s.db.First(&version, "user_id = ? AND version_number = ?", userID, versionNumber).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

// GetLatestVersion 获取最新版本
func (s *SyncService) GetLatestVersion(userID string) (*SyncVersion, error) {
	var version SyncVersion
	if err := s.db.Where("user_id = ?", userID).Order("version_number DESC").First(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

// ListVersions 列出所有版本（支持分页，limit=0表示不分页）
func (s *SyncService) ListVersions(userID string, limit, offset int) ([]VersionInfo, error) {
	var versions []SyncVersion
	query := s.db.Where("user_id = ?", userID).Order("version_number DESC")

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Find(&versions).Error; err != nil {
		return nil, err
	}

	result := make([]VersionInfo, len(versions))
	for i, v := range versions {
		result[i] = VersionInfo{
			VersionNumber: v.VersionNumber,
			FileSize:      v.FileSize,
			CreatedAt:     v.CreatedAt,
		}
	}
	return result, nil
}

// GetVersionReader 获取版本文件读取器
func (s *SyncService) GetVersionReader(userID string, versionNumber int) (io.ReadCloser, error) {
	return s.storage.Open(userID, versionNumber)
}

// DeleteVersion 删除指定版本（永久删除，无备份）
func (s *SyncService) DeleteVersion(userID string, versionNumber int) error {
	// 查询版本记录
	var version SyncVersion
	if err := s.db.First(&version, "user_id = ? AND version_number = ?", userID, versionNumber).Error; err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	// 删除物理文件
	if err := s.storage.Delete(userID, versionNumber); err != nil {
		return fmt.Errorf("failed to delete version file: %w", err)
	}

	// 删除数据库记录
	if err := s.db.Delete(&version).Error; err != nil {
		return fmt.Errorf("failed to delete version record: %w", err)
	}

	return nil
}

// cleanupOldVersions 清理旧版本
func (s *SyncService) cleanupOldVersions(userID string) error {
	var versions []SyncVersion
	if err := s.db.Where("user_id = ?", userID).Order("version_number DESC").Find(&versions).Error; err != nil {
		return err
	}

	if len(versions) <= s.maxVer {
		return nil
	}

	// 删除旧版本
	for i := s.maxVer; i < len(versions); i++ {
		v := versions[i]
		if err := s.storage.Delete(userID, v.VersionNumber); err != nil {
			return err
		}
		if err := s.db.Delete(&v).Error; err != nil {
			return err
		}
	}

	return nil
}

// Close 关闭服务
func (s *SyncService) Close() {
	if s.db != nil {
		s.db.Close()
	}
}
