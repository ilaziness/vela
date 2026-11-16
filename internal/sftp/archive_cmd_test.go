package sftp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/usererr"
)

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"abc":   "'abc'",
		"":      "''",
		"a b":   "'a b'",
		"a'b":   "'a'\\''b'",
		"$HOME": "'$HOME'",
		"`x`":   "'`x`'",
		"a\nb":  "'a\nb'",
		"/p/q":  "'/p/q'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestDetectArchiveFormat(t *testing.T) {
	cases := []struct {
		name string
		want archiveFormat
	}{
		{"a.zip", formatZip},
		{"A.ZIP", formatZip},
		{"a.tar", formatTar},
		{"a.tar.gz", formatTarGz},
		{"a.tgz", formatTarGz},
		{"a.taz", formatTarGz},
		{"a.tar.bz2", formatTarBz2},
		{"a.tbz2", formatTarBz2},
		{"a.tbz", formatTarBz2},
		{"a.tar.xz", formatTarXz},
		{"a.txz", formatTarXz},
		{"a.gz", formatGz},
		{"a.bz2", formatBz2},
		{"a.xz", formatXz},
		{"a.txt", formatUnknown},
		{"gz", formatUnknown},
		{"/root/dir.tar.gz", formatTarGz},
	}
	for _, c := range cases {
		if got := detectArchiveFormat(c.name); got != c.want {
			t.Errorf("detectArchiveFormat(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCompressCommand(t *testing.T) {
	got, err := compressCommand("/root/out.tar.gz", []string{"/data/src"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "tar czf '/root/out.tar.gz' -C '/data' 'src'"; got != want {
		t.Fatalf("compressCommand = %q, want %q", got, want)
	}

	got, err = compressCommand("/r/a b.tar.gz", []string{"/data/src", "/other/file-a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "tar czf '/r/a b.tar.gz' -C '/data' 'src' -C '/other' 'file-a.txt'"; got != want {
		t.Fatalf("compressCommand multi = %q, want %q", got, want)
	}

	// A leading-dash member must never parse as a tar option.
	got, err = compressCommand("/r/o.tar.gz", []string{"/data/-odd"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "tar czf '/r/o.tar.gz' -C '/data' './-odd'"; got != want {
		t.Fatalf("compressCommand dash = %q, want %q", got, want)
	}

	// File at the filesystem root.
	got, err = compressCommand("/r/o.tar.gz", []string{"/file.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "tar czf '/r/o.tar.gz' -C '/' 'file.txt'"; got != want {
		t.Fatalf("compressCommand root file = %q, want %q", got, want)
	}

	// Trailing slash is trimmed before splitting.
	got, err = compressCommand("/r/o.tar.gz", []string{"/data/src/"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "tar czf '/r/o.tar.gz' -C '/data' 'src'"; got != want {
		t.Fatalf("compressCommand trailing slash = %q, want %q", got, want)
	}

	if _, err := compressCommand("/r/o.tar.gz", nil); usererr.CodeOf(err) != "sftp.invalid_compress" {
		t.Fatalf("empty srcs error = %v", err)
	}
	if _, err := compressCommand("/r/o.tar.gz", []string{"/"}); usererr.CodeOf(err) != "sftp.invalid_compress" {
		t.Fatalf("root src error = %v", err)
	}
	if _, err := compressCommand("/r/o.tar.gz", []string{"/x/.."}); usererr.CodeOf(err) != "sftp.invalid_compress" {
		t.Fatalf("dotdot src error = %v", err)
	}
}

func TestListExtractCommands(t *testing.T) {
	if got, want := listTarCommand(formatTarGz, "/a b.tgz"), "tar tzf '/a b.tgz'"; got != want {
		t.Errorf("listTarCommand gz = %q, want %q", got, want)
	}
	if got, want := listTarCommand(formatTar, "/a.tar"), "tar tf '/a.tar'"; got != want {
		t.Errorf("listTarCommand plain = %q, want %q", got, want)
	}
	if got, want := extractTarCommand(formatTarBz2, "/a.tar.bz2", "/dest dir", 1), "tar xjf '/a.tar.bz2' -C '/dest dir' --strip-components=1"; got != want {
		t.Errorf("extractTarCommand bz2 strip = %q, want %q", got, want)
	}
	if got, want := extractTarCommand(formatTarXz, "/a.tar.xz", "/d", 0), "tar xJf '/a.tar.xz' -C '/d'"; got != want {
		t.Errorf("extractTarCommand xz = %q, want %q", got, want)
	}
	if got, want := extractSingleCommand(formatGz, "/a/f.txt.gz", "/d/f.txt"), "gzip -dc '/a/f.txt.gz' > '/d/f.txt'"; got != want {
		t.Errorf("extractSingleCommand gz = %q, want %q", got, want)
	}
	if got, want := extractSingleCommand(formatBz2, "/a/f.txt.bz2", "/d/f.txt"), "bzip2 -dc '/a/f.txt.bz2' > '/d/f.txt'"; got != want {
		t.Errorf("extractSingleCommand bz2 = %q, want %q", got, want)
	}
	if got, want := extractSingleCommand(formatXz, "/a/f.txt.xz", "/d/f.txt"), "xz -dc '/a/f.txt.xz' > '/d/f.txt'"; got != want {
		t.Errorf("extractSingleCommand xz = %q, want %q", got, want)
	}
}

func TestSingleStreamName(t *testing.T) {
	cases := map[string]string{
		"/a/f.txt.gz":  "f.txt",
		"f.gz":         "f",
		"/a/f.txt.bz2": "f.txt",
		"/a/f.txt.xz":  "f.txt",
		"f.tgz":        "",
		".gz":          "",
		"/a/f.txt":     "",
	}
	for in, want := range cases {
		if got := singleStreamName(in); got != want {
			t.Errorf("singleStreamName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestArchiveEntrySafety(t *testing.T) {
	cases := []struct {
		raw        string
		wantName   string
		wantUnsafe bool
	}{
		{"a/b.txt", "a/b.txt", false},
		{"./a", "a", false},
		{"a//b", "a/b", false},
		{"a/./b", "a/b", false},
		{"src/", "src", false},
		{"/abs", "", true},
		{"../x", "", true},
		{"a/../b", "", true},
		{"C:/x", "", true},
		{"a\\..\\b", "", true},
	}
	for _, c := range cases {
		name, unsafe := archiveEntrySafety(c.raw)
		if name != c.wantName || unsafe != c.wantUnsafe {
			t.Errorf("archiveEntrySafety(%q) = (%q, %v), want (%q, %v)", c.raw, name, unsafe, c.wantName, c.wantUnsafe)
		}
	}
}

func TestClassifyExec(t *testing.T) {
	err := errors.New("boom")
	if got := classifyExec(false, 0, nil); got != execOK {
		t.Errorf("nil error = %v, want execOK", got)
	}
	if got := classifyExec(false, 0, err); got != execUnavailable {
		t.Errorf("no status = %v, want execUnavailable", got)
	}
	if got := classifyExec(true, 126, err); got != execUnavailable {
		t.Errorf("status 126 = %v, want execUnavailable", got)
	}
	if got := classifyExec(true, 127, err); got != execUnavailable {
		t.Errorf("status 127 = %v, want execUnavailable", got)
	}
	if got := classifyExec(true, 2, err); got != execFailed {
		t.Errorf("status 2 = %v, want execFailed", got)
	}
	if got := classifyExec(true, 1, err); got != execFailed {
		t.Errorf("status 1 = %v, want execFailed", got)
	}
}

func TestCommandFailureDetail(t *testing.T) {
	if got := commandFailureDetail("  some output  ", nil); got != "some output" {
		t.Errorf("detail = %q", got)
	}
	if got := commandFailureDetail("", errors.New("err text")); got != "err text" {
		t.Errorf("detail fallback = %q", got)
	}
	long := strings.Repeat("x", 700)
	got := commandFailureDetail(long, nil)
	if r := []rune(got); len(r) != 601 || !strings.HasPrefix(got, "…") {
		t.Errorf("detail truncation: len=%d prefix=%q", len(r), got[:1])
	}
}

func TestFlattenEntries(t *testing.T) {
	tree := []archiveEntry{
		{name: "src", isDir: true},
		{name: "src/a.txt"},
		{name: "src/sub", isDir: true},
		{name: "src/sub/b.txt"},
	}
	flat, stripped := flattenEntries(tree)
	if !stripped {
		t.Fatal("expected stripped for single-root tree")
	}
	var names []string
	for _, e := range flat {
		names = append(names, e.name)
	}
	if want := []string{"a.txt", "sub", "sub/b.txt"}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("flat names = %v, want %v", names, want)
	}

	multi := []archiveEntry{{name: "a.txt"}, {name: "b.log"}}
	flat, stripped = flattenEntries(multi)
	if stripped || len(flat) != 2 {
		t.Fatalf("multi-root must not strip: stripped=%v flat=%v", stripped, flat)
	}

	lone := []archiveEntry{{name: "file.txt"}}
	flat, stripped = flattenEntries(lone)
	if stripped || len(flat) != 1 || flat[0].name != "file.txt" {
		t.Fatalf("lone file must not strip: stripped=%v flat=%v", stripped, flat)
	}

	empty, stripped := flattenEntries(nil)
	if stripped || len(empty) != 0 {
		t.Fatalf("empty entries: stripped=%v flat=%v", stripped, empty)
	}
}

// newRemoteExecRecorder returns a remoteExec stub and the command log.
func newRemoteExecRecorder(t *testing.T, handle func(cmd string) (string, error)) (*Manager, *[]string) {
	t.Helper()
	mgr := NewManager(zap.NewNop(), nil, transfer.NewRegistry(func(transfer.ProgressData) {}), nil)
	cmds := &[]string{}
	mgr.remoteExec = func(ctx context.Context, sessionID, cmd string, timeout time.Duration, maxOut int) (string, error) {
		*cmds = append(*cmds, cmd)
		if handle != nil {
			return handle(cmd)
		}
		return "", nil
	}
	return mgr, cmds
}

func TestRunRemoteCommandUnavailable(t *testing.T) {
	mgr := NewManager(zap.NewNop(), nil, transfer.NewRegistry(func(transfer.ProgressData) {}), nil)
	_, err := mgr.runRemoteCommand(context.Background(), "test", "tar tzf x", 0, 1024)
	if usererr.CodeOf(err) != "sftp.remote_exec_unavailable" {
		t.Fatalf("nil remoteExec error = %v", err)
	}

	// A transport error without a remote exit status is "unavailable" too.
	mgr.remoteExec = func(ctx context.Context, sessionID, cmd string, timeout time.Duration, maxOut int) (string, error) {
		return "", errors.New("connection lost")
	}
	_, err = mgr.runRemoteCommand(context.Background(), "test", "tar tzf x", 0, 1024)
	if usererr.CodeOf(err) != "sftp.remote_exec_unavailable" {
		t.Fatalf("transport error = %v", err)
	}

	// Success passes the output through.
	mgr.remoteExec = func(ctx context.Context, sessionID, cmd string, timeout time.Duration, maxOut int) (string, error) {
		return "a.txt\n", nil
	}
	out, err := mgr.runRemoteCommand(context.Background(), "test", "tar tzf x", 0, 1024)
	if err != nil || out != "a.txt\n" {
		t.Fatalf("success path = %q, %v", out, err)
	}
}

func TestCompressRemoteTarCleansUp(t *testing.T) {
	mgr, cmds := newRemoteExecRecorder(t, func(cmd string) (string, error) {
		if strings.HasPrefix(cmd, "tar ") {
			return "tar: boom", errors.New("exit status 2")
		}
		return "", nil
	})
	err := mgr.compressRemoteTar("test", "tar czf '/x/y.tar.gz' -C '/d' 's'", "/x/y.tar.gz")
	if err == nil {
		t.Fatal("expected compress failure")
	}
	if len(*cmds) != 2 {
		t.Fatalf("commands = %v, want tar then rm", *cmds)
	}
	if want := "rm -f '/x/y.tar.gz'"; (*cmds)[1] != want {
		t.Fatalf("cleanup cmd = %q, want %q", (*cmds)[1], want)
	}
}
