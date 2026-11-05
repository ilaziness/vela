package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAndVerifyUserBcrypt(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(Database{Type: "sqlite", DBPath: filepath.Join(dir, "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	storage, err := NewStorage(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewSyncService(db, storage, 5)

	plainKey := strings.Repeat("ab", 32) // 64 hex-like chars
	user, err := svc.CreateUser("sync-id-1", plainKey)
	if err != nil {
		t.Fatal(err)
	}
	if user.UserKey == plainKey {
		t.Fatal("user key stored in plaintext")
	}
	if !strings.HasPrefix(user.UserKey, "$2a$") && !strings.HasPrefix(user.UserKey, "$2b$") {
		t.Fatalf("expected bcrypt hash, got %q", user.UserKey)
	}

	if _, err := svc.VerifyUser("sync-id-1", plainKey); err != nil {
		t.Fatalf("verify correct key: %v", err)
	}
	if _, err := svc.VerifyUser("sync-id-1", "wrong-key"); err == nil {
		t.Fatal("expected verify failure")
	}
}
