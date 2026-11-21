package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ilaziness/vela/internal/termio"
)

func TestRecordingDirPrefersRecordDir(t *testing.T) {
	dir := t.TempDir()
	c := &Controller{}
	c.termCfg.RecordDir = dir
	got, err := c.recordingDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("recordingDir = %q, want %q", got, dir)
	}
}

func TestListRecordingsFiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "2026-01-01T00-00-00-old.gtr")
	newer := filepath.Join(dir, "2026-01-02T00-00-00-new.GTR")
	for _, p := range []string{older, newer, filepath.Join(dir, "notes.txt")} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "dir.gtr"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	c := &Controller{}
	items, err := c.listRecordings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2: %+v", len(items), items)
	}
	if items[0].path != newer || items[1].path != older {
		t.Fatalf("order = %q, %q; want newest first", items[0].path, items[1].path)
	}
}

func TestListRecordingsMissingDir(t *testing.T) {
	c := &Controller{}
	items, err := c.listRecordings(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v, want empty", items)
	}
}

func TestIsReplayProtocol(t *testing.T) {
	if !IsReplayProtocol(termio.ProtocolReplay) {
		t.Fatal("ProtocolReplay should be replay")
	}
	if IsReplayProtocol(termio.ProtocolSSH) || IsReplayProtocol(termio.ProtocolSerial) {
		t.Fatal("dialable protocols must not be replay")
	}
	if _, ok := termio.ParseProtocol(string(termio.ProtocolReplay)); ok {
		t.Fatal("replay must not parse as a dialable protocol")
	}
}
