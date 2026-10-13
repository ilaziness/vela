package database

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestResolveDBPathMigratesLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, legacyDBFileName)
	want := filepath.Join(dir, dbFileName)
	if err := os.WriteFile(legacy, []byte("sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveDBPath(dir, zap.NewNop())
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("vela.db missing after migrate: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy db should be gone, err=%v", err)
	}
}

func TestResolveDBPathFallsBackWhenDestinationBlocked(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, legacyDBFileName)
	target := filepath.Join(dir, dbFileName)
	if err := os.WriteFile(legacy, []byte("sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Occupy the destination as a directory so rename cannot create vela.db.
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	got := resolveDBPath(dir, zap.NewNop())
	if got != legacy {
		t.Fatalf("got %q, want legacy %q", got, legacy)
	}
}

func TestResolveDBPathPrefersLegacyOverEmptyVela(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, legacyDBFileName)
	target := filepath.Join(dir, dbFileName)
	if err := os.WriteFile(legacy, []byte("has-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveDBPath(dir, zap.NewNop())
	if got != target {
		t.Fatalf("got %q, want migrated %q", got, target)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "has-data" {
		t.Fatalf("vela.db content = %q, want legacy data", data)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy db should be gone, err=%v", err)
	}
}
