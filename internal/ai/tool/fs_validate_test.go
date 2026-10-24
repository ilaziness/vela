package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateAllowedDir(t *testing.T) {
	root := t.TempDir()
	abs, err := ValidateAllowedDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if abs == "" {
		t.Fatal("empty abs")
	}

	missing := filepath.Join(root, "no-such-dir")
	if _, err := ValidateAllowedDir(missing); err == nil {
		t.Fatal("expected missing dir error")
	}

	file := filepath.Join(root, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAllowedDir(file); err == nil {
		t.Fatal("expected file-not-dir error")
	}

	if _, err := ValidateAllowedDir("  "); err == nil {
		t.Fatal("expected empty path error")
	}
	if _, err := ValidateAllowedDir("relative/path"); err == nil {
		t.Fatal("expected relative path error")
	}
}

func TestSanitizeAllowedDirsDropsInvalid(t *testing.T) {
	root := t.TempDir()
	got := SanitizeAllowedDirs([]string{root, filepath.Join(root, "missing"), "relative"})
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}
