package sftp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/home/user/file.txt": "'/home/user/file.txt'",
		"/a b/c'd.txt":        `'/a b/c'\''d.txt'`,
		"中文/文件":               `'中文/文件'`,
		"":                    `''`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShCmd(t *testing.T) {
	got := shCmd("cd '/tmp' && ls")
	if !strings.HasPrefix(got, "sh -c '") || !strings.HasSuffix(got, "'") {
		t.Fatalf("shCmd not wrapped: %q", got)
	}
}

func TestDetectArchiveFormat(t *testing.T) {
	cases := map[string]string{
		"a.zip":        ArchZip,
		"a.tar":        ArchTar,
		"a.tar.gz":     ArchTarGz,
		"a.tgz":        ArchTarGz,
		"a.tar.bz2":    ArchTarBz2,
		"a.tbz2":       ArchTarBz2,
		"a.tar.xz":     ArchTarXz,
		"a.txz":        ArchTarXz,
		"a.tar.zst":    ArchTarZst,
		"a.tzst":       ArchTarZst,
		"a.ZIP":        ArchZip,
		"a.tar.zip":    ArchZip, // .zip wins over nothing; suffix order fine
		"a.txt":        "",
		"a.tar.gz.zip": ArchZip,
		"noext":        "",
	}
	for name, want := range cases {
		got, ok := detectArchiveFormat(name)
		if want == "" {
			if ok {
				t.Errorf("detectArchiveFormat(%q) = %q, want none", name, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("detectArchiveFormat(%q) = %q,%v want %q", name, got, ok, want)
		}
	}
}

func TestBuildCompressCmd(t *testing.T) {
	cmd, err := buildCompressCmd("/home/u", "archive.tar.gz", []string{"foo", "-rf", "a'b"}, ArchTarGz)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"sh -c '", "cd ", "/home/u", "tar -czf", "'./archive.tar.gz'", "./foo", "./-rf", "./a",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("cmd %q missing %q", cmd, want)
		}
	}
	if _, err := buildCompressCmd("/d", "o", []string{"a"}, "rar"); err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestBuildExtractCmd(t *testing.T) {
	cmd, err := buildExtractCmd("/d/a.tar.xz", "/d", ArchTarXz)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "tar -xJf") || !strings.Contains(cmd, "/d/a.tar.xz") || !strings.Contains(cmd, "-C") {
		t.Errorf("unexpected cmd: %q", cmd)
	}
	cmd, err = buildExtractCmd("/d/a.zip", "/d", ArchZip)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "unzip -o") || !strings.Contains(cmd, "/d/a.zip") || !strings.Contains(cmd, "-d") {
		t.Errorf("unexpected cmd: %q", cmd)
	}
	if _, err := buildExtractCmd("/d/a.rar", "/d", "rar"); err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestBuildCopyMoveCmd(t *testing.T) {
	cp := buildCopyCmd([]string{"/a b/x", "/y"}, "/dest")
	if !strings.Contains(cp, "cp -rf") || !strings.Contains(cp, "/a b/x") || !strings.Contains(cp, "/dest/") {
		t.Errorf("bad copy cmd: %q", cp)
	}
	mv := buildMoveCmd([]string{"/a"}, "/")
	if !strings.Contains(mv, "mv -f") || !strings.Contains(mv, "'/a'") {
		t.Errorf("bad move cmd: %q", mv)
	}
}

func TestCheckPasteGuard(t *testing.T) {
	if err := checkPasteGuard([]string{"/a/b"}, "/a/b/c"); err == nil {
		t.Error("paste into own subtree should fail")
	}
	if err := checkPasteGuard([]string{"/a/b"}, "/a/b"); err == nil {
		t.Error("paste onto itself should fail")
	}
	if err := checkPasteGuard([]string{"/a/b"}, "/x"); err != nil {
		t.Errorf("normal paste should pass: %v", err)
	}
	if err := checkPasteGuard([]string{"/a/bc"}, "/a/b"); err != nil {
		t.Errorf("sibling prefix should pass: %v", err)
	}
}

func TestExecRemoteCancel(t *testing.T) {
	m := &Manager{}
	started := make(chan struct{})
	m.execFn = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}
	done := make(chan error, 1)
	go func() { _, err := m.execRemote("s1", "sleep 999"); done <- err }()
	<-started
	m.CancelRemoteOp("s1")
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if _, loaded := m.execCancels.Load("s1"); loaded {
		t.Error("execCancels entry not cleaned up")
	}
}

func TestExecRemoteErrorMerge(t *testing.T) {
	m := &Manager{}
	m.execFn = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		return "zip: command not found", errors.New("exit status 127")
	}
	_, err := m.execRemote("s1", "x")
	if err == nil || !strings.Contains(err.Error(), "command not found") || !strings.Contains(err.Error(), "exit status 127") {
		t.Fatalf("expected merged stderr+exit error, got %v", err)
	}
}

func TestExecRemoteSecondOpRejected(t *testing.T) {
	m := &Manager{}
	block := make(chan struct{})
	m.execFn = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		<-block
		return "", nil
	}
	done := make(chan error, 1)
	go func() { _, err := m.execRemote("s1", "x"); done <- err }()
	// wait until registered
	for i := 0; i < 100; i++ {
		if _, ok := m.execCancels.Load("s1"); ok {
			break
		}
		time.Sleep(time.Millisecond)
	}
	_, err := m.execRemote("s1", "y")
	if err == nil {
		t.Fatal("expected second op to be rejected")
	}
	close(block)
	<-done
}
