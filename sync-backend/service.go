package main

import (
	"fmt"
	"io"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const userKeyBcryptCost = 12

type SyncService struct {
	db       *DB
	storage  *Storage
	maxFiles int
}

func NewSyncService(db *DB, storage *Storage, maxFiles int) *SyncService {
	return &SyncService{
		db:       db,
		storage:  storage,
		maxFiles: maxFiles,
	}
}

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

func (s *SyncService) GetUser(id string) (*User, error) {
	var user User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

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

func (s *SyncService) UpdateLastSyncAt(userID string) error {
	return s.db.Model(&User{}).Where("id = ?", userID).Update("last_sync_at", time.Now()).Error
}

func (s *SyncService) GetLastSyncAt(userID string) (time.Time, error) {
	var user User
	if err := s.db.Select("last_sync_at").First(&user, "id = ?", userID).Error; err != nil {
		return time.Time{}, err
	}
	return user.LastSyncAt, nil
}

func (s *SyncService) SaveBackup(userID, name string, reader io.Reader) (*BackupInfo, error) {
	name, err := sanitizeBackupName(name)
	if err != nil {
		return nil, err
	}
	size, err := s.storage.Save(userID, name, reader)
	if err != nil {
		return nil, err
	}
	if err := s.cleanupOldFiles(userID); err != nil {
		fmt.Printf("failed to cleanup old backups: %v\n", err)
	}
	if info, err := s.storage.Stat(userID, name); err == nil {
		return &BackupInfo{Name: info.Name, FileSize: info.Size, CreatedAt: info.ModTime.UTC()}, nil
	}
	return &BackupInfo{Name: name, FileSize: size, CreatedAt: time.Now().UTC()}, nil
}

func (s *SyncService) GetBackupReader(userID, name string) (io.ReadCloser, error) {
	return s.storage.Open(userID, name)
}

func (s *SyncService) ListBackups(userID string, limit, offset int) ([]BackupInfo, error) {
	files, err := s.storage.List(userID)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].ModTime.After(files[j].ModTime)
		}
		return files[i].Name > files[j].Name
	})
	if offset < 0 {
		offset = 0
	}
	if offset >= len(files) {
		return []BackupInfo{}, nil
	}
	files = files[offset:]
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	result := make([]BackupInfo, len(files))
	for i, f := range files {
		result[i] = BackupInfo{Name: f.Name, FileSize: f.Size, CreatedAt: f.ModTime.UTC()}
	}
	return result, nil
}

func (s *SyncService) DeleteBackup(userID, name string) error {
	return s.storage.Delete(userID, name)
}

func (s *SyncService) cleanupOldFiles(userID string) error {
	if s.maxFiles <= 0 {
		return nil
	}
	files, err := s.storage.List(userID)
	if err != nil {
		return err
	}
	if len(files) <= s.maxFiles {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})
	for i := s.maxFiles; i < len(files); i++ {
		if err := s.storage.Delete(userID, files[i].Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *SyncService) Close() {
	if s.db != nil {
		s.db.Close()
	}
}
