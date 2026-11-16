package sftp

import (
	"os"
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
		src = "/root/src/a.txt"
		dst = "/root/dst/a.txt"
	)
	staging := editStagingRemotePath(dst) // /root/dst/.a.txt.vela-edit
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
			name:     "copy dir over dir merges",
			srcIsDir: true,
			dstInfo:  dir,
			want:     "m=$(stat -c %a '" + dst + "') && cp -a -- '" + src + "/.' '" + dst + "'/ && chmod \"$m\" '" + dst + "'",
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
			name:     "cut dir over dir merges then removes source",
			srcIsDir: true,
			cut:      true,
			dstInfo:  dir,
			want:     "m=$(stat -c %a '" + dst + "') && cp -a -- '" + src + "/.' '" + dst + "'/ && chmod \"$m\" '" + dst + "' && rm -rf -- '" + src + "'",
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
			cmd, cleanup, err := pasteCommand(src, dst, tt.srcIsDir, tt.cut, tt.dstInfo)
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

func TestPasteCommandRejectsEmptyPaths(t *testing.T) {
	if _, _, err := pasteCommand("", "/root/a", false, false, nil); err == nil {
		t.Fatal("expected error for empty src")
	}
	if _, _, err := pasteCommand("/root/a", "", false, false, nil); err == nil {
		t.Fatal("expected error for empty dst")
	}
}

func TestPasteCommandQuotesHostileNames(t *testing.T) {
	src := `/root/it's a "test" $(rm -rf /).txt`
	dst := "/root/dst/x.txt"
	cmd, _, err := pasteCommand(src, dst, false, true, nil)
	if err != nil {
		t.Fatalf("pasteCommand: %v", err)
	}
	want := `mv -f -- '/root/it'\''s a "test" $(rm -rf /).txt' '/root/dst/x.txt'`
	if cmd != want {
		t.Errorf("command\ngot  %q\nwant %q", cmd, want)
	}
}
