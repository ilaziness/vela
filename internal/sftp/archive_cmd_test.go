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
	if got, want := listTarCommand(formatTarGz, "/a b.tgz"), "TAR_OPTIONS=--quoting-style=escape tar tzf '/a b.tgz'"; got != want {
		t.Errorf("listTarCommand gz = %q, want %q", got, want)
	}
	if got, want := listTarCommand(formatTar, "/a.tar"), "TAR_OPTIONS=--quoting-style=escape tar tf '/a.tar'"; got != want {
		t.Errorf("listTarCommand plain = %q, want %q", got, want)
	}
	if got, want := extractTarCommand(formatTarBz2, "/a.tar.bz2", "/dest dir", 1), "tar xjf '/a.tar.bz2' -C '/dest dir' --strip-components=1"; got != want {
		t.Errorf("extractTarCommand bz2 strip = %q, want %q", got, want)
	}
	if got, want := extractTarCommand(formatTarXz, "/a.tar.xz", "/d", 0), "tar xJf '/a.tar.xz' -C '/d'"; got != want {
		t.Errorf("extractTarCommand xz plain = %q, want %q", got, want)
	}
	// The no-overwrite path stages the archive in an operation-owned staging
	// directory and promotes the flattened names under per-entry guards.
	noOverwrite := extractTarNoOverwriteCommand(formatTarXz, "/a.tar.xz", "/d", 0)
	// The staging directory lives inside the destination (no write access to
	// the destination's parent is required) and the promotion walks the
	// staging tree itself: no inline name list can grow past the shell's
	// per-argument size cap.
	if !strings.HasPrefix(noOverwrite, "st='/d/.d.vela-extract-") {
		t.Errorf("extractTarNoOverwriteCommand staging not inside destDir: %q", noOverwrite)
	}
	for _, banned := range []string{"for n in", "mv -f"} {
		if strings.Contains(noOverwrite, banned) {
			t.Errorf("extractTarNoOverwriteCommand contains banned %q in %q", banned, noOverwrite)
		}
	}
	for _, want := range []string{
		`tar xJf '/a.tar.xz' -C "$st"`,
		`rc=0; cd "$d" 2>/dev/null || touch -- "$fl"; find "$st" -mindepth 1 -exec sh -c '`,
		`n=${s#"$st"/}; t="$d/$n"`,
		`[ -e "$s" ] || [ -L "$s" ] || continue`,
		`if [ -e "$t" ] || [ -L "$t" ]; then touch -- "$fl"; continue; fi`,
		`if [ -L "$q" ]; then touch -- "$fl"; continue 2; fi`,
		`mkdir -- "$q" 2>/dev/null`,
		`m=$(stat -c %a -- "$s") && chmod -- "$m" "$t" || touch -- "$fl"`,
		`g=$(readlink -- "$s") && ln -s -- "$g" "$t" || touch -- "$fl"`,
		`ln -- "$s" "$t" 2>/dev/null && rm -rf -- "$s" || touch -- "$fl"`,
		`sh "$st" "$fl" "$d" {} +`,
		`rc=0; [ -e "$fl" ] && rc=1; rm -rf -- "$st" "$fl"; exit "$rc"; }`,
		`|| { rm -rf -- "$st" "$fl"; false; }`,
	} {
		if !strings.Contains(noOverwrite, want) {
			t.Errorf("extractTarNoOverwriteCommand missing %q in %q", want, noOverwrite)
		}
	}
	const staging = "/d/.f.txt.vela-edit-1a2b3c4d"
	gzOverwrite := `gzip -dc '/a/f.txt.gz' > '` + staging + `' || { rm -f -- '` + staging + `'; false; } && if [ -f '/d/f.txt' ] && [ ! -L '/d/f.txt' ]; then m=$(stat -c %a -- '/d/f.txt') && chmod -- "$m" '` + staging + `' || { rm -f -- '` + staging + `'; false; }; fi && rm -rf -- '/d/f.txt' && mv -f -- '` + staging + `' '/d/f.txt'`
	if got, want := extractSingleCommand(formatGz, "/a/f.txt.gz", staging, "/d/f.txt", true), gzOverwrite; got != want {
		t.Errorf("extractSingleCommand gz overwrite = %q, want %q", got, want)
	}
	bz2Keep := `bzip2 -dc '/a/f.txt.bz2' > '` + staging + `' || { rm -f -- '` + staging + `'; false; } && if [ -e '/d/f.txt' ] || [ -L '/d/f.txt' ]; then rm -f -- '` + staging + `'; false; else mv -f -- '` + staging + `' '/d/f.txt'; fi`
	if got, want := extractSingleCommand(formatBz2, "/a/f.txt.bz2", staging, "/d/f.txt", false), bz2Keep; got != want {
		t.Errorf("extractSingleCommand bz2 keep = %q, want %q", got, want)
	}
	xzOverwrite := `xz -dc '/a/f.txt.xz' > '` + staging + `' || { rm -f -- '` + staging + `'; false; } && if [ -f '/d/f.txt' ] && [ ! -L '/d/f.txt' ]; then m=$(stat -c %a -- '/d/f.txt') && chmod -- "$m" '` + staging + `' || { rm -f -- '` + staging + `'; false; }; fi && rm -rf -- '/d/f.txt' && mv -f -- '` + staging + `' '/d/f.txt'`
	if got, want := extractSingleCommand(formatXz, "/a/f.txt.xz", staging, "/d/f.txt", true), xzOverwrite; got != want {
		t.Errorf("extractSingleCommand xz overwrite = %q, want %q", got, want)
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

func TestUnescapeTarName(t *testing.T) {
	cases := map[string]string{
		`plain/name.txt`: `plain/name.txt`,
		`a\nb`:           "a\nb",
		`c\\d`:           `c\d`,
		`tab\there`:      "tab\there",
		`oct\040space`:   "oct space",
		`unknown\q`:      `unknown\q`,
		`trailing\`:      `trailing\`,
	}
	for in, want := range cases {
		if got := unescapeTarName(in); got != want {
			t.Errorf("unescapeTarName(%q) = %q, want %q", in, got, want)
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
		// "." components are kept: tar counts a leading "./" as the first
		// path component, so flattening must agree on the written paths.
		{"./a", "./a", false},
		{"a//b", "a/b", false},
		{"a/./b", "a/./b", false},
		{"src/", "src", false},
		{"/abs", "", true},
		{"../x", "", true},
		{"a/../b", "", true},
		{"C:/x", "", true},
		// A backslash is an ordinary POSIX filename character, not a
		// separator: "a\..\b" is one safe component.
		{`a\..\b`, `a\..\b`, false},
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

	// A single nested entry does have a root to strip — the old guard kept it
	// whole, creating the root folder again on the remote tar path.
	nested := []archiveEntry{{name: "src/a.txt"}}
	flat, stripped = flattenEntries(nested)
	if !stripped || len(flat) != 1 || flat[0].name != "a.txt" {
		t.Fatalf("single nested entry must strip: stripped=%v flat=%v", stripped, flat)
	}

	// Repeated top-level members share a plain file name (an archive updated
	// with tar -r): nothing to strip, every version stays listed.
	repeatTop := []archiveEntry{{name: "a.txt"}, {name: "a.txt"}}
	flat, stripped = flattenEntries(repeatTop)
	if stripped || len(flat) != 2 {
		t.Fatalf("repeated top-level file must not strip: stripped=%v flat=%v", stripped, flat)
	}

	// Repeated members under one common root strip and keep every version.
	repeat := []archiveEntry{{name: "root/a.txt"}, {name: "root/a.txt"}}
	flat, stripped = flattenEntries(repeat)
	if !stripped || len(flat) != 2 || flat[0].name != "a.txt" || flat[1].name != "a.txt" {
		t.Fatalf("repeated nested entries must strip with duplicates: stripped=%v flat=%v", stripped, flat)
	}

	// A dot-rooted tree flattens like a plain one ("./src/a.txt" -> "src/a.txt"
	// after tar strips one component).
	dot := []archiveEntry{{name: "./src", isDir: true}, {name: "./src/a.txt"}}
	flat, stripped = flattenEntries(dot)
	if !stripped || len(flat) != 2 || flat[0].name != "src" || flat[1].name != "src/a.txt" {
		t.Fatalf("dot-rooted tree must strip: stripped=%v flat=%v", stripped, flat)
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
	err := mgr.compressRemoteTar(context.Background(), "test", "tar czf '/x/y.tar.gz' -C '/d' 's'", "/x/y.tar.gz")
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

// TestCompressRemoteTarCleansUpAfterCancel: a cancelled run still removes its
// partial archive, and the cleanup command always runs on a live context.
func TestCompressRemoteTarCleansUpAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mgr := NewManager(zap.NewNop(), nil, transfer.NewRegistry(func(transfer.ProgressData) {}), nil)
	var rmCtxErr error
	mgr.remoteExec = func(c context.Context, sessionID, cmd string, timeout time.Duration, maxOut int) (string, error) {
		if strings.HasPrefix(cmd, "tar ") {
			return "", ctx.Err()
		}
		rmCtxErr = c.Err()
		return "", nil
	}
	if err := mgr.compressRemoteTar(ctx, "test", "tar czf '/x/y.tar.gz' -C '/d' 's'", "/x/y.tar.gz"); err == nil {
		t.Fatal("expected cancellation failure")
	}
	if rmCtxErr != nil {
		t.Fatalf("cleanup ran on a dead context: %v", rmCtxErr)
	}
}
