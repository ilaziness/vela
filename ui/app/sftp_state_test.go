package app

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/ui/protocol"
	sshui "github.com/ilaziness/vela/ui/protocol/ssh"
)

func TestRemoteJoin(t *testing.T) {
	cases := []struct{ dir, name, want string }{
		{"/", "a.txt", "/a.txt"},
		{"", "a.txt", "/a.txt"},
		{"/home", "a.txt", "/home/a.txt"},
		{"/home/", "a.txt", "/home/a.txt"},
		{"/home/user", "dir", "/home/user/dir"},
	}
	for _, tc := range cases {
		if got := remoteJoin(tc.dir, tc.name); got != tc.want {
			t.Errorf("remoteJoin(%q,%q) = %q, want %q", tc.dir, tc.name, got, tc.want)
		}
	}
}

func TestRemoteParent(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/", "/"},
		{"/home", "/"},
		{"/home/user", "/home"},
		{"/home/user/", "/home"},
		{"a/b", "/a"},
		{"", "/"},
	}
	for _, tc := range cases {
		if got := remoteParent(tc.in); got != tc.want {
			t.Errorf("remoteParent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidRemoteName(t *testing.T) {
	for _, ok := range []string{"file.txt", "my dir", "a-b_c"} {
		if !validRemoteName(ok) {
			t.Errorf("validRemoteName(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "/abs"} {
		if validRemoteName(bad) {
			t.Errorf("validRemoteName(%q) = true, want false", bad)
		}
	}
}

func TestNaturalLess(t *testing.T) {
	in := []string{"file10.txt", "file2.txt", "File1.txt", "abc", "abd"}
	want := []string{"abc", "abd", "File1.txt", "file2.txt", "file10.txt"}
	out := append([]string(nil), in...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if naturalLess(out[j], out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", out, want)
		}
	}
}

func TestParseModeInput(t *testing.T) {
	cases := []struct {
		in   string
		want uint32
		ok   bool
	}{
		{"755", 0o755, true},
		{"0755", 0o755, true},
		{"0o755", 0o755, true},
		{"rwxr-xr-x", 0o755, true},
		{"", 0, false},
		{"888", 0, false},
		{"../etc", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseModeInput(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseModeInput(%q) = %o,%v want %o,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFormatModeBits(t *testing.T) {
	if got := formatModeBits(0o755); got != "755" {
		t.Fatalf("formatModeBits(755) = %q", got)
	}
	if got := formatModeBits(0o4755); got != "4755" {
		t.Fatalf("formatModeBits(4755) = %q", got)
	}
}

func TestTransferUpsertRemove(t *testing.T) {
	list := transferUpsert(nil, transfer.ProgressData{ID: "a", Transferred: 1})
	list = transferUpsert(list, transfer.ProgressData{ID: "b"})
	list = transferUpsert(list, transfer.ProgressData{ID: "a", Transferred: 5})
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Transferred != 5 {
		t.Fatalf("upsert did not replace: %+v", list[0])
	}
	list = transferRemove(list, "a")
	if len(list) != 1 || list[0].ID != "b" {
		t.Fatalf("remove failed: %+v", list)
	}
}

func TestSFTPStateSelection(t *testing.T) {
	s := newSFTPState("k", "/tmp")
	s.RemoteDir = "/remote"
	s.RemoteEntries = []sftp.FileInfo{{Name: "a"}, {Name: "b", IsDir: true}}
	s.LocalEntries = []localfs.Entry{{Path: "/tmp/x"}}

	s.ToggleRemote("a")
	s.ToggleRemote("a")
	if len(s.RemoteSelected) != 0 {
		t.Fatal("toggle did not clear")
	}
	s.ToggleRemote("a")
	if len(s.RemoteSelected) != 1 {
		t.Fatalf("select = %d", len(s.RemoteSelected))
	}
	// Single-select: picking another row replaces the selection.
	s.ToggleRemote("b")
	if len(s.RemoteSelected) != 1 || !s.RemoteSelected["b"] {
		t.Fatalf("single select = %v", s.RemoteSelected)
	}
	paths := s.RemotePaths()
	if len(paths) != 1 || paths[0] != "/remote/b" {
		t.Fatalf("RemotePaths = %v", paths)
	}
	s.PruneRemoteSelection()
	s.RemoteEntries = s.RemoteEntries[:1]
	s.PruneRemoteSelection()
	if len(s.RemoteSelected) != 0 {
		t.Fatalf("prune = %d", len(s.RemoteSelected))
	}
	s.ClearRemoteSelection()
	if len(s.RemoteSelected) != 0 {
		t.Fatal("clear failed")
	}
}

func TestSFTPOwnerKey(t *testing.T) {
	c := &Controller{}
	if got := c.sftpOwnerKey(reconnectReq{bookmarkID: "bm-1"}); got != "bm-1" {
		t.Fatalf("bookmark owner = %q", got)
	}
	req := reconnectReq{draft: protocol.Draft{Payload: sshui.Payload{User: "u", Host: "h"}}}
	if got := c.sftpOwnerKey(req); got != "u@h:22" {
		t.Fatalf("default port owner = %q", got)
	}
	req.draft.Payload = sshui.Payload{User: "u", Host: "h", Port: 2222}
	if got := c.sftpOwnerKey(req); got != "u@h:2222" {
		t.Fatalf("port owner = %q", got)
	}
	if got := c.sftpOwnerKey(reconnectReq{}); got != "" {
		t.Fatalf("empty owner = %q", got)
	}
}

func TestHandleWindowEventSkipsWhenSFTPOpen(t *testing.T) {
	st := &State{
		ActiveTab: "t1",
		Tabs: []Tab{{
			ID: "t1", Kind: TabSession, SessID: "s1",
			Status: TabStatusConnected, SFTPOpen: true,
		}},
	}
	c := &Controller{}
	// Must not panic or touch a terminal while the panel is open.
	c.HandleWindowEvent(&gui.Event{}, st)
}

func TestOnTerminalCwdGating(t *testing.T) {
	st := &State{
		ActiveTab: "t1",
		Tabs:      []Tab{{ID: "t1", Kind: TabSession, SessID: "s1", SFTPOpen: true}},
		SFTP: map[string]*SFTPState{
			"s1": {Status: SFTPReady, RemoteReady: true, RemoteDir: "/old"},
		},
	}
	w := newTestWindow(t, st)
	c := &Controller{logger: zap.NewNop()}
	c.SetWindow(w)
	// Panel closed for the tab: cwd follow is suppressed.
	st.Tabs[0].SFTPOpen = false
	c.OnTerminalCwd("s1", "/new")
	if s := st.SFTP["s1"]; s.RemoteDir != "/old" {
		t.Fatalf("cwd followed while panel closed: %q", s.RemoteDir)
	}
}
