package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Storage 文件存储
type Storage struct {
	baseDir string
}

type storedFile struct {
	Name    string
	Size    int64
	ModTime time.Time
}

func NewStorage(baseDir string) (*Storage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &Storage{baseDir: baseDir}, nil
}

func sanitizeBackupName(name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	name = path.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid backup name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return "", fmt.Errorf("invalid backup name")
	}
	for _, r := range name {
		if r < 32 || r == '/' || unicode.IsControl(r) {
			return "", fmt.Errorf("invalid backup name")
		}
	}
	return name, nil
}

func (s *Storage) getUserDir(userID string) string {
	return filepath.Join(s.baseDir, userID)
}

func (s *Storage) getFilePath(userID, name string) (string, error) {
	name, err := sanitizeBackupName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.getUserDir(userID), name), nil
}

func (s *Storage) Save(userID, name string, reader io.Reader) (int64, error) {
	filePath, err := s.getFilePath(userID, name)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(s.getUserDir(userID), 0755); err != nil {
		return 0, fmt.Errorf("failed to create user directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	size, err := io.Copy(file, reader)
	if err != nil {
		return 0, fmt.Errorf("failed to write file: %w", err)
	}
	return size, nil
}

func (s *Storage) Open(userID, name string) (io.ReadCloser, error) {
	filePath, err := s.getFilePath(userID, name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

func (s *Storage) Stat(userID, name string) (storedFile, error) {
	filePath, err := s.getFilePath(userID, name)
	if err != nil {
		return storedFile{}, err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return storedFile{}, err
	}
	return storedFile{Name: info.Name(), Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (s *Storage) Delete(userID, name string) error {
	filePath, err := s.getFilePath(userID, name)
	if err != nil {
		return err
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (s *Storage) List(userID string) ([]storedFile, error) {
	userDir := s.getUserDir(userID)
	entries, err := os.ReadDir(userDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	var files []storedFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name, err := sanitizeBackupName(e.Name())
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, storedFile{Name: name, Size: info.Size(), ModTime: info.ModTime()})
	}
	return files, nil
}

func (s *Storage) DeleteUserDir(userID string) error {
	userDir := s.getUserDir(userID)
	if err := os.RemoveAll(userDir); err != nil {
		return fmt.Errorf("failed to delete user directory: %w", err)
	}
	return nil
}
