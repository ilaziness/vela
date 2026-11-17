//go:build integration

package sftp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

// tarSpec describes one archive member for the round-5 regressions: regular
// files, directories with an explicit mode, and symlinks with arbitrary
// (including trailing-newline) target text.
type tarSpec struct {
	name string
	mode int64
	kind byte
	body string // regular file content or symlink target
}

func buildTarGzSpecs(t *testing.T, specs []tarSpec) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, s := range specs {
		hdr := &tar.Header{Name: s.name, Mode: s.mode, Typeflag: s.kind}
		switch s.kind {
		case tar.TypeDir:
			// Directory names conventionally end with a slash.
			if !strings.HasSuffix(hdr.Name, "/") {
				hdr.Name += "/"
			}
		case tar.TypeSymlink:
			hdr.Linkname = s.body
		case tar.TypeFifo:
			hdr.Size = 0
		default:
			hdr.Size = int64(len(s.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header %s: %v", s.name, err)
		}
		if s.kind == tar.TypeReg {
			if _, err := tw.Write([]byte(s.body)); err != nil {
				t.Fatalf("tar write %s: %v", s.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func waitRemoteLinkText(t *testing.T, client *sftp.Client, p, want, desc string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if got, err := client.ReadLink(p); err == nil && got == want {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for link %s -> %q (%s)", p, want, desc)
}

// TestIntegrationExtractReadOnlyDirNonRoot (#0001): an archived read-only
// directory (0555) must not be chmod'ed before its children are installed —
// otherwise a non-root user cannot link into it and cleanup cannot remove the
// read-only staged tree. The final directory mode is restored bottom-up after
// all descendants exist.
func TestIntegrationExtractReadOnlyDirNonRoot(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	work := "/tmp/vela-extract-rodir"
	_ = client.RemoveAll(work)
	t.Cleanup(func() {
		_ = client.RemoveAll(work)
	})
	remoteRun(t, mgr, "mkdir -p "+work+"/dest")
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/locked/", 0o555, tar.TypeDir, ""},
		{"root/locked/a.txt", 0o644, tar.TypeReg, "RO-OK\n"},
		{"root/top.txt", 0o644, tar.TypeReg, "TOP\n"},
	}))
	remoteRun(t, mgr, "chown -R nobody "+work+"/dest && chmod 0644 "+arc+" && chmod 0555 "+work)

	cmd := extractTarNoOverwriteCommand(formatTarGz, arc, path.Join(work, "dest"), 1)
	remoteRun(t, mgr, "su -s /bin/sh -c "+shellQuote(cmd)+" nobody")

	if got := string(readRemote(t, client, path.Join(work, "dest", "locked", "a.txt"))); got != "RO-OK\n" {
		t.Fatalf("content of read-only dir = %q, want %q", got, "RO-OK\n")
	}
	if got := string(readRemote(t, client, path.Join(work, "dest", "top.txt"))); got != "TOP\n" {
		t.Fatalf("top-level content = %q", got)
	}
	mode := strings.TrimSpace(remoteRun(t, mgr, "stat -c %a "+path.Join(work, "dest", "locked")))
	if mode != "555" {
		t.Fatalf("restored dir mode = %q, want 555", mode)
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "ls -A "+path.Join(work, "dest"))); got != "locked\ntop.txt" {
		t.Fatalf("dest entries = %q, want only locked and top.txt (staging cleaned)", got)
	}
}

// TestIntegrationExtractRelativeDestDir (#0002): the no-overwrite command must
// keep a RELATIVE destination spelling meaningful end-to-end. All paths are
// resolved against the initial cwd before find starts from a globally
// searchable directory (needed for non-root users inheriting /root), so a
// relative "out" never silently becomes "out/out" and staging is cleaned up.
func TestIntegrationExtractRelativeDestDir(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	work := "/tmp/vela-extract-rel"
	_ = client.RemoveAll(work)
	t.Cleanup(func() {
		_ = client.Chmod(work, 0o755)
		_ = client.RemoveAll(work)
	})
	remoteRun(t, mgr, "mkdir -p "+work+"/out")
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/a.txt", 0o644, tar.TypeReg, "REL\n"},
	}))

	// (a) root, relative destination.
	cmd := extractTarNoOverwriteCommand(formatTarGz, arc, "out", 1)
	remoteRun(t, mgr, "cd "+work+" && "+cmd)
	if got := string(readRemote(t, client, path.Join(work, "out", "a.txt"))); got != "REL\n" {
		t.Fatalf("root relative extraction = %q, want %q", got, "REL\n")
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "ls -A "+path.Join(work, "out"))); got != "a.txt" {
		t.Fatalf("root relative dest entries = %q", got)
	}
	if _, err := client.Lstat(path.Join(work, "out", "out")); err == nil {
		t.Fatalf("relative destination resolved twice (out/out exists)")
	}

	// (b) non-root, relative destination: find must not inherit /root as cwd.
	remoteRun(t, mgr, "rm -f "+arc+" && chown -R nobody "+work+"/out && chmod 0555 "+work)
	uploadBytes(t, client, path.Join(work, "b.tar.gz"), buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/b.txt", 0o644, tar.TypeReg, "REL-NP\n"},
	}))
	cmd = extractTarNoOverwriteCommand(formatTarGz, path.Join(work, "b.tar.gz"), "out", 1)
	remoteRun(t, mgr, "cd "+work+" && su -s /bin/sh -c "+shellQuote(cmd)+" nobody")
	if got := string(readRemote(t, client, path.Join(work, "out", "b.txt"))); got != "REL-NP\n" {
		t.Fatalf("non-root relative extraction = %q, want %q", got, "REL-NP\n")
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "ls -A "+path.Join(work, "out"))); got != "a.txt\nb.txt" {
		t.Fatalf("non-root relative dest entries = %q", got)
	}
}

// TestIntegrationExtractStagingSymlinkDotDot (#0003): a destination spelling
// with a symlink followed by ".." must keep that spelling for the staging
// directory. path.Join would clean it into a different (possibly absent) tree.
func TestIntegrationExtractStagingSymlinkDotDot(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "extract-linkdotdot")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/srv "+work+"/data/sub "+work+"/data/out")
	remoteRun(t, mgr, "ln -s -- ../data/sub "+work+"/srv/link")
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/a.txt", 0o644, tar.TypeReg, "DOTDOT\n"},
	}))

	// Literal spelling: path.Join would produce work + "/srv/out".
	dest := work + "/srv/link/../out"
	if err := mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, dest, true, false); err != nil {
		t.Fatalf("extract into symlink/.. destination: %v", err)
	}
	if got := string(readRemote(t, client, path.Join(work, "data", "out", "a.txt"))); got != "DOTDOT\n" {
		t.Fatalf("content at server-resolved destination = %q, want %q", got, "DOTDOT\n")
	}
	if _, err := client.Lstat(path.Join(work, "srv", "out")); err == nil {
		t.Fatalf("staging/extraction landed under the lexically cleaned /srv/out")
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "ls -A "+path.Join(work, "data", "out"))); got != "a.txt" {
		t.Fatalf("resolved dest entries = %q, want only a.txt (staging cleaned)", got)
	}
}

// TestIntegrationExtractCrossDeviceMount (#0004): a nested destination mount
// lives on a different filesystem, so link(2) from the staging tree fails with
// EXDEV. Promotion must copy into an operation-owned file on the target
// filesystem and claim the final name atomically (no overwrite), leaving both
// the mounted and unmounted entries installed.
func TestIntegrationExtractCrossDeviceMount(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	work := "/tmp/vela-extract-exdev"
	_ = client.RemoveAll(work)
	mp := path.Join(work, "dest", "data")
	t.Cleanup(func() {
		_, _ = mgr.runRemoteCommand(context.Background(), "test", "umount "+mp+" 2>/dev/null || true", 30*time.Second, 1<<20)
		_ = client.RemoveAll(work)
	})
	remoteRun(t, mgr, "mkdir -p "+mp)
	// tmpfs is a different device than the destination's parent filesystem.
	devBefore := strings.TrimSpace(remoteRun(t, mgr, "stat -c %d "+path.Join(work, "dest")))
	remoteRun(t, mgr, "mount -t tmpfs tmpfs "+mp)
	devMount := strings.TrimSpace(remoteRun(t, mgr, "stat -c %d "+mp))
	if devBefore == devMount {
		t.Fatalf("nested mount did not create a distinct device (%s == %s)", devBefore, devMount)
	}
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/data/", 0o755, tar.TypeDir, ""},
		{"root/data/a.txt", 0o644, tar.TypeReg, "CROSS-DEV\n"},
		{"root/config.txt", 0o644, tar.TypeReg, "SAME-DEV\n"},
	}))

	if err := mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, path.Join(work, "dest"), true, false); err != nil {
		t.Fatalf("extract across a nested mount: %v", err)
	}
	if got := string(readRemote(t, client, path.Join(mp, "a.txt"))); got != "CROSS-DEV\n" {
		t.Fatalf("mounted filesystem content = %q, want %q", got, "CROSS-DEV\n")
	}
	if got := string(readRemote(t, client, path.Join(work, "dest", "config.txt"))); got != "SAME-DEV\n" {
		t.Fatalf("root filesystem content = %q, want %q", got, "SAME-DEV\n")
	}
	// No operation-owned copy staging may survive on either filesystem.
	if got := strings.TrimSpace(remoteRun(t, mgr, "find "+path.Join(work, "dest")+" -name '.vela-copy-*' -print")); got != "" {
		t.Fatalf("copy-staging leftovers: %q", got)
	}
}

// TestIntegrationExtractSymlinkTargetTrailingNewline (#0005): command
// substitution strips trailing newlines from readlink output. The sentinel
// capture must reproduce the link target byte-for-byte.
func TestIntegrationExtractSymlinkTargetTrailingNewline(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "extract-link-nl")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	dest := path.Join(work, "dest")
	remoteRun(t, mgr, "mkdir -p "+dest)
	arc := path.Join(work, "a.tar.gz")
	const want = "report\n"
	uploadBytes(t, client, arc, buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/link", 0o777, tar.TypeSymlink, want},
	}))

	if err := mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, dest, true, false); err != nil {
		t.Fatalf("extract symlink with trailing-newline target: %v", err)
	}
	if got, err := client.ReadLink(path.Join(dest, "link")); err != nil || got != want {
		t.Fatalf("extracted link target = %q, %v; want %q", got, err, want)
	}
	waitRemoteLinkText(t, client, path.Join(dest, "link"), want, "trailing newline preserved")
}

// TestIntegrationExtractFifoCrossDevice (#0001 round 6): the EXDEV fallback
// must byte-copy REGULAR files only. cp -p on a FIFO would block forever
// waiting for a writer (the extract command previously ran without a timeout),
// so an archived FIFO on a nested mount is rejected explicitly and cleaned up
// instead of hanging.
func TestIntegrationExtractFifoCrossDevice(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	work := "/tmp/vela-extract-fifo"
	mp := path.Join(work, "dest", "data")
	_ = client.RemoveAll(work)
	t.Cleanup(func() {
		_, _ = mgr.runRemoteCommand(context.Background(), "test", "umount "+mp+" 2>/dev/null || true", 30*time.Second, 1<<20)
		_ = client.RemoveAll(work)
	})
	remoteRun(t, mgr, "mkdir -p "+mp)
	remoteRun(t, mgr, "mount -t tmpfs tmpfs "+mp)
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/data/", 0o755, tar.TypeDir, ""},
		{"root/data/events.pipe", 0o644, tar.TypeFifo, ""},
		{"root/ok.txt", 0o644, tar.TypeReg, "OK\n"},
	}))

	// The command must return promptly (well under the exec test timeout) with
	// an error, not hang on the FIFO.
	done := make(chan error, 1)
	go func() {
		done <- mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, path.Join(work, "dest"), true, false)
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("FIFO on a cross-device mount unexpectedly succeeded")
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("extraction hung on a cross-device FIFO (no exec timeout / cp -p blocked)")
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "ls -A "+mp)); got != "" {
		t.Fatalf("mounted dest entries = %q, want empty (no partial FIFO/staging left)", got)
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "find "+work+" -name '.vela-*' -print")); got != "" {
		t.Fatalf("operation-owned leftovers after FIFO rejection: %q", got)
	}
}

// TestIntegrationExtractNewlineDirNoChmodInjection (#0002 round 6, security):
// a directory name containing a newline used to be written into a line-based
// mode list and parsed back with read/awk, injecting extra chmod targets
// outside the destination. Modes now travel in staging-tree markers and names
// never enter a line-oriented parser.
func TestIntegrationExtractNewlineDirNoChmodInjection(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "extract-newline-dir")
	_ = client.RemoveAll(work)
	t.Cleanup(func() {
		_, _ = mgr.runRemoteCommand(context.Background(), "test", "chmod 0700 /tmp/vela-inject-target 2>/dev/null || true", 30*time.Second, 1<<20)
		_ = client.RemoveAll(work)
	})
	dest := path.Join(work, "dest")
	remoteRun(t, mgr, "mkdir -p "+dest+" /tmp/vela-inject-target")
	remoteRun(t, mgr, "chmod 0700 /tmp/vela-inject-target")
	// A newline in the directory name would split the old "mode path" list;
	// the forged second line asked chmod to widen this external directory.
	evil := "evil\n0777 /tmp/vela-inject-target"
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzSpecs(t, []tarSpec{
		{"root/", 0o755, tar.TypeDir, ""},
		{"root/" + evil + "/", 0o750, tar.TypeDir, ""},
		{"root/" + evil + "/a.txt", 0o644, tar.TypeReg, "SAFE\n"},
	}))

	if err := mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, dest, true, false); err != nil {
		t.Fatalf("extract newline-bearing directory name: %v", err)
	}
	// The external directory's mode must be untouched (0700), not widened.
	if got := strings.TrimSpace(remoteRun(t, mgr, "stat -c %a /tmp/vela-inject-target")); got != "700" {
		t.Fatalf("external directory mode = %q, want 700 (no injected chmod)", got)
	}
	// The legitimate (if unusual) directory is still created with its archived
	// mode, and no staging marker leaks into the destination.
	if got := string(readRemote(t, client, path.Join(dest, evil, "a.txt"))); got != "SAFE\n" {
		t.Fatalf("content under newline directory = %q", got)
	}
	dirMode := strings.TrimSpace(remoteRun(t, mgr, "stat -c %a "+shellQuote(path.Join(dest, evil))))
	if dirMode != "750" {
		t.Fatalf("newline directory mode = %q, want 750", dirMode)
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "find "+dest+" -name '.vela-*' -print")); got != "" {
		t.Fatalf("staging markers leaked into destination: %q", got)
	}
}
