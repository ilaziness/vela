package config

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestWindowGeometryEffectiveSize(t *testing.T) {
	st := WindowGeometry{Width: 1200, Height: 800}
	w, h := st.EffectiveSize()
	if w != 1200 || h != 800 {
		t.Fatalf("got %dx%d", w, h)
	}
	st = WindowGeometry{}
	w, h = st.EffectiveSize()
	if w != DefaultWindowWidth || h != DefaultWindowHeight {
		t.Fatalf("default got %dx%d", w, h)
	}
	st = WindowGeometry{Width: 100, Height: 100}
	w, h = st.EffectiveSize()
	if w != DefaultWindowWidth || h != DefaultWindowHeight {
		t.Fatalf("too small got %dx%d", w, h)
	}
}

func TestClampToWorkArea(t *testing.T) {
	w, h := ClampToWorkArea(3000, 2000, 1920, 1080)
	if w != 1920 || h != 1080 {
		t.Fatalf("got %dx%d", w, h)
	}
	w, h = ClampToWorkArea(800, 600, 0, 0)
	if w != 800 || h != 600 {
		t.Fatalf("no work area got %dx%d", w, h)
	}
}

func TestSaveLoadLocalState(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	st := WindowGeometry{Width: 1400, Height: 900, Maximised: true}
	if err := store.SaveWindow(st); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, LocalStateFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file missing: %v", err)
	}

	sub := filepath.Join(dir, "browse")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLastDialogDir(sub); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSftpLocalDir("host-a", sub); err != nil {
		t.Fatal(err)
	}

	store2 := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	store2.LoadLocalState()
	got := store2.Window()
	if got.Width != 1400 || got.Height != 900 || !got.Maximised {
		t.Fatalf("window got %+v", got)
	}
	if store2.LastDialogDir() != sub {
		t.Fatalf("dialog dir got %q", store2.LastDialogDir())
	}
	if store2.SftpLocalDir("host-a") != sub {
		t.Fatalf("sftp dir got %q", store2.SftpLocalDir("host-a"))
	}
}

func TestSaveWindowPreservesDirs(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	sub := filepath.Join(dir, "keep")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLastDialogDir(sub); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSftpLocalDir("bk1", sub); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWindow(WindowGeometry{Width: 1000, Height: 800}); err != nil {
		t.Fatal(err)
	}
	if store.LastDialogDir() != sub {
		t.Fatalf("dialog lost: %q", store.LastDialogDir())
	}
	if store.SftpLocalDir("bk1") != sub {
		t.Fatalf("sftp lost: %q", store.SftpLocalDir("bk1"))
	}
}

func TestSetDirsSkipInvalid(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	if err := store.SetLastDialogDir(filepath.Join(dir, "missing")); err != nil {
		t.Fatal(err)
	}
	if store.LastDialogDir() != "" {
		t.Fatalf("expected empty, got %q", store.LastDialogDir())
	}
	if err := store.SetSftpLocalDir("", dir); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSftpLocalDir("x", filepath.Join(dir, "nope")); err != nil {
		t.Fatal(err)
	}
	if store.SftpLocalDir("x") != "" {
		t.Fatalf("expected empty sftp dir")
	}
}

func TestSaveWindowRejectsZero(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	if err := store.SaveWindow(WindowGeometry{Width: 0, Height: 0}); err == nil {
		t.Fatal("expected error")
	}
}
