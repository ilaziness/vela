package tool

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeAllowedDirs(t *testing.T) {
	got := NormalizeAllowedDirs([]string{"", "  ", "."})
	if len(got) != 1 {
		t.Fatalf("expected 1 dir, got %v", got)
	}
}

func TestResolveUnderAllowlist(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b.txt")
	if err := os.MkdirAll(filepath.Dir(sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, err := ResolveUnderAllowlist(sub, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if abs == "" {
		t.Fatal("empty abs")
	}
	outside := filepath.Join(filepath.Dir(root), "other.txt")
	if _, err := ResolveUnderAllowlist(outside, []string{root}); err == nil {
		t.Fatal("expected outside path to fail")
	}
}

func TestResolveTraversalRejected(t *testing.T) {
	root := t.TempDir()
	evil := filepath.Join(root, "..", "escape.txt")
	if _, err := ResolveUnderAllowlist(evil, []string{root}); err == nil {
		t.Fatal("expected traversal reject")
	}
}

func TestResolveSymlinkEscapeRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation often requires elevation on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "leak")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	target := filepath.Join(link, "secret.txt")
	if _, err := ResolveUnderAllowlist(target, []string{root}); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestResolveWriteThroughSymlinkRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation often requires elevation on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "out")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	target := filepath.Join(link, "new.txt")
	if _, err := ResolveUnderAllowlist(target, []string{root}); err == nil {
		t.Fatal("expected write-through symlink to be rejected")
	}
}

func TestResolveOutsideSymlinkIntoAllowRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation often requires elevation on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	insideFile := filepath.Join(root, "ok.txt")
	if err := os.WriteFile(insideFile, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(outside, "into")
	if err := os.Symlink(insideFile, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ResolveUnderAllowlist(link, []string{root}); err == nil {
		t.Fatal("expected outside path into allow root via symlink to be rejected")
	}
}

func TestUnderPathDotDotName(t *testing.T) {
	root := t.TempDir()
	weird := filepath.Join(root, "..foo")
	if err := os.Mkdir(weird, 0o755); err != nil {
		t.Fatal(err)
	}
	if !underPath(weird, root) {
		t.Fatal("..foo under root should be allowed")
	}
}

func TestUnderPathWindowsCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	if !underPath(`C:\Work\file.txt`, `c:\work`) {
		t.Fatal("expected case-insensitive under")
	}
}
