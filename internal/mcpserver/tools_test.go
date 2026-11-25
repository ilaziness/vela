package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/termio"
)

func testBookmarks() []*bookmark.Group {
	return []*bookmark.Group{
		{
			Name: "prod",
			Bookmarks: []bookmark.Bookmark{
				{ID: "b1", Title: "Web Server", GroupName: "prod", Protocol: "ssh", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"web", "prod"}},
				{ID: "b2", Title: "DB Host", GroupName: "prod", Protocol: "ssh", Host: "10.0.0.2", Port: 22, User: "admin", Tags: []string{"db"}},
			},
		},
		{
			Name: "lab",
			Bookmarks: []bookmark.Bookmark{
				{ID: "b3", Title: "Router Console", GroupName: "lab", Protocol: "serial", Host: "", Port: 0, User: "", SerialDevice: "/dev/ttyUSB0"},
			},
		},
	}
}

func TestFilterBookmarks(t *testing.T) {
	all := flattenBookmarks(testBookmarks())

	got := filterBookmarks(all, searchBookmarksInput{Query: "web"})
	if len(got) != 1 || got[0].ID != "b1" {
		t.Fatalf("query filter: %+v", got)
	}

	got = filterBookmarks(all, searchBookmarksInput{Group: "LAB"})
	if len(got) != 1 || got[0].ID != "b3" {
		t.Fatalf("group filter should be case-insensitive: %+v", got)
	}

	got = filterBookmarks(all, searchBookmarksInput{Protocol: "ssh"})
	if len(got) != 2 {
		t.Fatalf("protocol filter: %+v", got)
	}

	got = filterBookmarks(all, searchBookmarksInput{Tags: []string{"prod", "web"}})
	if len(got) != 1 || got[0].ID != "b1" {
		t.Fatalf("tags AND filter: %+v", got)
	}

	got = filterBookmarks(all, searchBookmarksInput{})
	if len(got) != 3 {
		t.Fatalf("empty filter should return all: %+v", got)
	}
}

func TestExecParams(t *testing.T) {
	timeout, maxOut := execParams(0, 0)
	if timeout != defaultExecTimeout || maxOut != defaultMaxOutputByte {
		t.Fatalf("defaults: %v %d", timeout, maxOut)
	}
	timeout, maxOut = execParams(9999, 1<<30)
	if timeout != maxExecTimeout || maxOut != maxMaxOutputByte {
		t.Fatalf("clamps: %v %d", timeout, maxOut)
	}
	timeout, _ = execParams(10, 0)
	if timeout != 10*time.Second {
		t.Fatalf("timeout: %v", timeout)
	}
}

func TestListSessionsEmptyHub(t *testing.T) {
	out, err := listSessions(context.Background(), &Deps{Hub: termio.NewHub()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty hub should return []: %q", out)
	}
}

func TestRunCommandOnSessionValidation(t *testing.T) {
	d := &Deps{SSH: nil}
	if _, err := runCommandOnSession(context.Background(), d, sessionCommandInput{SessionID: "s1", Command: "ls"}); err == nil {
		t.Fatal("expected error when SSH manager is nil")
	}
}
