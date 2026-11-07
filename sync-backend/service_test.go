package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupOldFilesKeepsNewest(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	svc := NewSyncService(nil, storage, 2)
	userID := "test-user"
	names := []string{"a.zip", "b.zip", "c.zip"}
	for i, name := range names {
		if _, err := storage.Save(userID, name, bytes.NewReader([]byte(name))); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		ts := time.Date(2026, 11, 1, 0, 0, i, 0, time.UTC)
		if err := os.Chtimes(filepath.Join(tmpDir, userID, name), ts, ts); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}
	if err := svc.cleanupOldFiles(userID); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	files, err := storage.List(userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string]bool{}
	for _, f := range files {
		got[f.Name] = true
	}
	if len(files) != 2 || !got["b.zip"] || !got["c.zip"] {
		t.Fatalf("kept %v, want b.zip and c.zip", got)
	}
}
