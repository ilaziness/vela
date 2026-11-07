package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStorage(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	if storage.baseDir != tmpDir {
		t.Errorf("expected baseDir %s, got %s", tmpDir, storage.baseDir)
	}
}

func TestStorageSaveAndOpen(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	userID := "test-user"
	name := "vela-backup-20261107T010203Z-abcd.zip"
	data := []byte("test data content")

	size, err := storage.Save(userID, name, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to save file: %v", err)
	}
	if size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), size)
	}

	expectedPath := filepath.Join(tmpDir, userID, name)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("file missing: %v", err)
	}

	reader, err := storage.Open(userID, name)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !bytes.Equal(content, data) {
		t.Errorf("content mismatch")
	}
}

func TestStorageRejectsBadName(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	if _, err := storage.Save("u", "payload.bin", bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("expected reject")
	}
}

func TestStorageDeleteAndList(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	userID := "test-user"
	name := "vela-backup-1.zip"
	if _, err := storage.Save(userID, name, bytes.NewReader([]byte("test data"))); err != nil {
		t.Fatalf("failed to save file: %v", err)
	}

	files, err := storage.List(userID)
	if err != nil || len(files) != 1 || files[0].Name != name {
		t.Fatalf("list=%v err=%v", files, err)
	}

	if err := storage.Delete(userID, name); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}
	if _, err := storage.Open(userID, name); err == nil {
		t.Error("should not be able to open deleted file")
	}
}

func TestStorageDeleteUserDir(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	userID := "test-user"
	if _, err := storage.Save(userID, "a.zip", bytes.NewReader([]byte("a"))); err != nil {
		t.Fatalf("failed to save file: %v", err)
	}
	if err := storage.DeleteUserDir(userID); err != nil {
		t.Fatalf("failed to delete user dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, userID)); !os.IsNotExist(err) {
		t.Error("user directory should be deleted")
	}
}
