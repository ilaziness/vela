package sftpui

import (
	"testing"
	"time"

	"github.com/ilaziness/vela/ui/app"
)

func TestRemotePathOf(t *testing.T) {
	s := &app.SFTPState{RemoteDir: "/home/u"}
	if got := remotePathOf(s, "f.txt"); got != "/home/u/f.txt" {
		t.Fatalf("remotePathOf = %q", got)
	}
	s.RemoteDir = "/"
	if got := remotePathOf(s, "f.txt"); got != "/f.txt" {
		t.Fatalf("remotePathOf root = %q", got)
	}
	s.RemoteDir = "/home/u/"
	if got := remotePathOf(s, "f.txt"); got != "/home/u/f.txt" {
		t.Fatalf("remotePathOf trailing slash = %q", got)
	}
}

func TestOnRowActivate(t *testing.T) {
	var last string
	var at time.Time
	fired := 0
	fn := func() { fired++ }

	// First click only records.
	onRowActivate(&last, &at, "a", fn)
	if fired != 0 {
		t.Fatal("single click fired")
	}
	// Second click on the same row within the window fires once.
	onRowActivate(&last, &at, "a", fn)
	if fired != 1 {
		t.Fatalf("double click fired %d times", fired)
	}
	// A different row resets tracking.
	onRowActivate(&last, &at, "b", fn)
	onRowActivate(&last, &at, "b", fn)
	if fired != 2 {
		t.Fatalf("second double click fired %d times", fired)
	}
}

func TestFormatSizeModTime(t *testing.T) {
	if got := formatSize(0); got == "" {
		t.Fatal("formatSize(0) empty")
	}
	if got := formatSize(-1); got != "" {
		t.Fatalf("formatSize(-1) = %q", got)
	}
	if got := formatModTime(""); got != "" {
		t.Fatalf("formatModTime empty = %q", got)
	}
	if got := formatModTime("not-a-time"); got != "not-a-time" {
		t.Fatalf("formatModTime passthrough = %q", got)
	}
	if got := formatModTime("2024-01-02T03:04:05Z"); got == "" {
		t.Fatal("formatModTime valid = empty")
	}
}
