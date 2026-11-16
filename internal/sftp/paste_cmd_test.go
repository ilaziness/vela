package sftp

import (
	"errors"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

// pasteFakeInfo is a minimal os.FileInfo for pasteCommand's dst view.
type pasteFakeInfo struct {
	mode os.FileMode
}

func (f pasteFakeInfo) Name() string       { return "x" }
func (f pasteFakeInfo) Size() int64        { return 0 }
func (f pasteFakeInfo) Mode() os.FileMode  { return f.mode }
func (f pasteFakeInfo) ModTime() time.Time { return time.Time{} }
func (f pasteFakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f pasteFakeInfo) Sys() any           { return nil }

func TestPasteCommand(t *testing.T) {
	const (
		src     = "/root/src/a.txt"
		dst     = "/root/dst/a.txt"
		staging = "/root/dst/.a.txt.vela-edit-1a2b3c4d"
	)
	dir := pasteFakeInfo{mode: os.ModeDir | 0o755}
	file := pasteFakeInfo{mode: 0o644}
	link := pasteFakeInfo{mode: os.ModeSymlink | 0o777}

	tests := []struct {
		name      string
		srcIsDir  bool
		cut       bool
		dstInfo   os.FileInfo
		want      string
		wantClean string
	}{
		{
			name:      "copy file, dst missing",
			want:      "rm -rf -- '" + staging + "' && cp -a -- '" + src + "' '" + staging + "' && mv -f -- '" + staging + "' '" + dst + "'",
			wantClean: "rm -rf -- '" + staging + "'",
		},
		{
			name:      "copy file over file",
			dstInfo:   file,
			want:      "rm -rf -- '" + staging + "' && cp -a -- '" + src + "' '" + staging + "' && mv -f -- '" + staging + "' '" + dst + "'",
			wantClean: "rm -rf -- '" + staging + "'",
		},
		{
			name:      "copy file over symlink removes link first",
			dstInfo:   link,
			want:      "rm -rf -- '" + staging + "' && cp -a -- '" + src + "' '" + staging + "' && rm -rf -- '" + dst + "' && mv -f -- '" + staging + "' '" + dst + "'",
			wantClean: "rm -rf -- '" + staging + "'",
		},
		{
			name:      "copy file over dir removes tree after staging",
			dstInfo:   dir,
			want:      "rm -rf -- '" + staging + "' && cp -a -- '" + src + "' '" + staging + "' && rm -rf -- '" + dst + "' && mv -f -- '" + staging + "' '" + dst + "'",
			wantClean: "rm -rf -- '" + staging + "'",
		},
		{
			name:      "copy dir, dst missing",
			srcIsDir:  true,
			want:      "rm -rf -- '" + staging + "' && cp -a -- '" + src + "' '" + staging + "' && mv -f -- '" + staging + "' '" + dst + "'",
			wantClean: "rm -rf -- '" + staging + "'",
		},
		{
			name:      "copy dir over file removes file after staging",
			srcIsDir:  true,
			dstInfo:   file,
			want:      "rm -rf -- '" + staging + "' && cp -a -- '" + src + "' '" + staging + "' && rm -rf -- '" + dst + "' && mv -f -- '" + staging + "' '" + dst + "'",
			wantClean: "rm -rf -- '" + staging + "'",
		},
		{
			name: "cut file, dst missing",
			cut:  true,
			want: "mv -f -- '" + src + "' '" + dst + "'",
		},
		{
			name:    "cut file over file",
			cut:     true,
			dstInfo: file,
			want:    "mv -f -- '" + src + "' '" + dst + "'",
		},
		{
			name:    "cut file over symlink removes link first",
			cut:     true,
			dstInfo: link,
			want:    "rm -rf -- '" + dst + "' && mv -f -- '" + src + "' '" + dst + "'",
		},
		{
			name:     "cut dir, dst missing",
			srcIsDir: true,
			cut:      true,
			want:     "mv -f -- '" + src + "' '" + dst + "'",
		},
		{
			name:     "cut dir over file removes file first",
			srcIsDir: true,
			cut:      true,
			dstInfo:  file,
			want:     "rm -rf -- '" + dst + "' && mv -f -- '" + src + "' '" + dst + "'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cleanup, err := pasteCommand(src, dst, tt.srcIsDir, tt.cut, tt.dstInfo, staging)
			if err != nil {
				t.Fatalf("pasteCommand: %v", err)
			}
			if cmd != tt.want {
				t.Errorf("command\ngot  %q\nwant %q", cmd, tt.want)
			}
			if cleanup != tt.wantClean {
				t.Errorf("cleanup\ngot  %q\nwant %q", cleanup, tt.wantClean)
			}
		})
	}
}

// TestPasteCommandMergeUsesStagedFallback: a real directory merged onto a real
// directory never runs as a remote cp -a — that would broaden existing
// destination subdirectory modes and write replacement bytes directly over
// existing destination files. The merge runs on the staged SFTP path instead.
func TestPasteCommandMergeUsesStagedFallback(t *testing.T) {
	dir := pasteFakeInfo{mode: os.ModeDir | 0o755}
	for _, cut := range []bool{false, true} {
		_, _, err := pasteCommand("/root/src/a", "/root/dst/a", true, cut, dir, "/root/dst/.a.vela-edit-1a2b3c4d")
		if !errors.Is(err, errPasteCommandFallback) {
			t.Fatalf("cut=%v: merge error = %v, want errPasteCommandFallback", cut, err)
		}
	}
}

func TestPasteCommandRejectsEmptyPaths(t *testing.T) {
	const staging = "/root/.a.vela-edit-1a2b3c4d"
	if _, _, err := pasteCommand("", "/root/a", false, false, nil, staging); err == nil {
		t.Fatal("expected error for empty src")
	}
	if _, _, err := pasteCommand("/root/a", "", false, false, nil, staging); err == nil {
		t.Fatal("expected error for empty dst")
	}
	if _, _, err := pasteCommand("/root/a", "/root/b", false, false, nil, "/root/b"); err == nil {
		t.Fatal("expected error for staging equal to dst")
	}
}

func TestPasteCommandQuotesHostileNames(t *testing.T) {
	src := `/root/it's a "test" $(rm -rf /).txt`
	dst := "/root/dst/x.txt"
	staging := "/root/dst/.x.txt.vela-edit-1a2b3c4d"
	cmd, _, err := pasteCommand(src, dst, false, true, nil, staging)
	if err != nil {
		t.Fatalf("pasteCommand: %v", err)
	}
	want := `mv -f -- '/root/it'\''s a "test" $(rm -rf /).txt' '/root/dst/x.txt'`
	if cmd != want {
		t.Errorf("command\ngot  %q\nwant %q", cmd, want)
	}
}

// TestStagingRemotePathUnique: every operation gets its own staging name, so
// cleanup can never remove another operation's or a user's entry and a
// pre-planted staging symlink cannot be predicted.
func TestStagingRemotePathUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 16; i++ {
		got := stagingRemotePath("/root/dst/a.txt")
		if seen[got] {
			t.Fatalf("stagingRemotePath returned a duplicate: %q", got)
		}
		seen[got] = true
		if want := "/root/dst/.a.txt.vela-edit-"; !strings.HasPrefix(got, want) {
			t.Fatalf("staging %q does not start with %q", got, want)
		}
	}
	if got := stagingRemotePath("/a"); !strings.HasPrefix(path.Base(got), ".a.vela-edit-") {
		t.Fatalf("staging for root-relative target = %q", got)
	}
	// Long basenames stay within typical filesystem name limits.
	if n := len(path.Base(stagingRemotePath("/d/" + strings.Repeat("b", 300)))); n > 240 {
		t.Fatalf("staging base name too long: %d", n)
	}
}
