package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMtimeGate(t *testing.T) {
	var g mtimeGate
	base := time.Now()
	mod := base.Add(time.Second)
	stable := 100 * time.Millisecond

	// First sighting only starts the debounce window.
	if g.observe(base, mod, base, stable) {
		t.Fatal("first observe fired early")
	}
	// Still inside the stable window.
	if g.observe(base.Add(50*time.Millisecond), mod, base, stable) {
		t.Fatal("observe fired before stable window")
	}
	// Stable for long enough: fires.
	if !g.observe(base.Add(200*time.Millisecond), mod, base, stable) {
		t.Fatal("observe did not fire after stable window")
	}
	// mtime not newer than baseline resets.
	g.reset()
	if g.observe(base, base, base, stable) {
		t.Fatal("observe fired for unchanged mtime")
	}
	// A newer mtime restarts the debounce.
	mod2 := mod.Add(time.Second)
	if g.observe(base, mod2, base, stable) {
		t.Fatal("observe fired on first sighting of new mtime")
	}
}

func TestSanitizeEditBase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/etc/nginx.conf", "nginx.conf"},
		{"plain.txt", "plain.txt"},
		{"a/b/c.tar.gz", "c.tar.gz"},
		{"..", "file"},
		{"", "file"},
		{"a\x01b\x7f.txt", "a_b_.txt"},
		{"trailing. ", "trailing"},
	}
	for _, tc := range cases {
		if got := sanitizeEditBase(tc.in); got != tc.want {
			t.Errorf("sanitizeEditBase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEditTempFile(t *testing.T) {
	p, err := editTempFile("sess-1", "/home/u/f.txt")
	if err != nil {
		t.Fatalf("editTempFile: %v", err)
	}
	if !strings.Contains(p, "sess-1") || !strings.HasSuffix(p, "f.txt") {
		t.Fatalf("unexpected temp path %q", p)
	}
	if filepath.Base(filepath.Dir(p)) == "" {
		t.Fatal("empty digest dir")
	}
	// Different remote paths hash to different dirs.
	p2, err := editTempFile("sess-1", "/home/u/g.txt")
	if err != nil {
		t.Fatalf("editTempFile: %v", err)
	}
	if p == p2 {
		t.Fatal("two remote paths mapped to the same temp file")
	}
	// Path traversal in the session id is rejected.
	if _, err := editTempFile("../evil", "/x"); err == nil {
		t.Fatal("editTempFile accepted a traversal session id")
	}
}

func TestEditSessionDir(t *testing.T) {
	if _, ok := editSessionDir(""); ok {
		t.Fatal("empty session id accepted")
	}
	if _, ok := editSessionDir("a/b"); ok {
		t.Fatal("slash session id accepted")
	}
	if _, ok := editSessionDir("..\\win"); ok {
		t.Fatal("backslash session id accepted")
	}
	dir, ok := editSessionDir("sess-1")
	if !ok || !strings.Contains(dir, "sess-1") {
		t.Fatalf("editSessionDir = %q,%v", dir, ok)
	}
}
