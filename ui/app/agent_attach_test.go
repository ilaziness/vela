package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/ilaziness/vela/internal/ai"
)

func TestInspectComposerAttachment(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(okPath, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	abs, mime, name, reason := inspectComposerAttachment(okPath)
	if reason != "" || mime != "text/plain" || name != "note.txt" || abs != okPath {
		t.Fatalf("got %s %s %s %s", abs, mime, name, reason)
	}

	if _, _, _, reason = inspectComposerAttachment(filepath.Join(dir, "x.bin")); reason != "type" {
		t.Fatalf("bin reason=%s", reason)
	}
	empty := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, reason = inspectComposerAttachment(empty); reason != "type" {
		t.Fatalf("empty reason=%s", reason)
	}

	big := filepath.Join(dir, "big.txt")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(ai.MaxAttachBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, reason = inspectComposerAttachment(big); reason != "size" {
		t.Fatalf("size reason=%s", reason)
	}
}

func TestFitPreviewImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wide.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 2000, 1000))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	w, h := fitPreviewImage(path, 600, 420)
	if w != 600 || h != 300 {
		t.Fatalf("wide fit = %.0fx%.0f, want 600x300", w, h)
	}
	small := filepath.Join(dir, "small.png")
	sf, err := os.Create(small)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(sf, image.NewNRGBA(image.Rect(0, 0, 40, 20))); err != nil {
		t.Fatal(err)
	}
	if err := sf.Close(); err != nil {
		t.Fatal(err)
	}
	w, h = fitPreviewImage(small, 600, 420)
	if w != 40 || h != 20 {
		t.Fatalf("small fit = %.0fx%.0f, want 40x20", w, h)
	}
}
