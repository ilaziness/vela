package config

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestWindowStateEffectiveSize(t *testing.T) {
	st := WindowState{Width: 1200, Height: 800}
	w, h := st.EffectiveSize()
	if w != 1200 || h != 800 {
		t.Fatalf("got %dx%d", w, h)
	}
	st = WindowState{}
	w, h = st.EffectiveSize()
	if w != DefaultWindowWidth || h != DefaultWindowHeight {
		t.Fatalf("default got %dx%d", w, h)
	}
	st = WindowState{Width: 100, Height: 100}
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

func TestSaveLoadWindow(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	st := WindowState{Width: 1400, Height: 900, Maximised: true}
	if err := store.SaveWindow(st); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, WindowStateFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file missing: %v", err)
	}
	store2 := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	store2.LoadWindow()
	got := store2.Window()
	if got.Width != 1400 || got.Height != 900 || !got.Maximised {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveWindowRejectsZero(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Config: &Config{General: GeneralConfig{UserDataDir: dir}},
		logger: zap.NewNop(),
	}
	if err := store.SaveWindow(WindowState{Width: 0, Height: 0}); err == nil {
		t.Fatal("expected error")
	}
}
