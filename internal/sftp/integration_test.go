//go:build integration

package sftp

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/usererr"
)

// Integration harness against a real SSH/SFTP server.
// Set VELA_SFTP_TEST_ADDR (host:port), VELA_SFTP_TEST_USER, VELA_SFTP_TEST_PASS
// and build with: go test -tags integration ./internal/sftp/ -run TestIntegration -v
// dialTestSSH dials the test server and returns the raw SSH client so tests
// can open their own exec sessions (remote archive commands, fixtures).
func dialTestSSH(t *testing.T) *gossh.Client {
	t.Helper()
	addr := os.Getenv("VELA_SFTP_TEST_ADDR")
	user := os.Getenv("VELA_SFTP_TEST_USER")
	pass := os.Getenv("VELA_SFTP_TEST_PASS")
	if addr == "" || user == "" || pass == "" {
		t.Skip("integration env vars not set")
	}
	cfg := &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.Password(pass)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	conn, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("ssh dial %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func dialTestClient(t *testing.T) *sftp.Client {
	conn := dialTestSSH(t)
	client, err := sftp.NewClient(conn)
	if err != nil {
		t.Fatalf("sftp client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

type progressCollector struct {
	mu     sync.Mutex
	events []transfer.ProgressData
}

func (p *progressCollector) record(d transfer.ProgressData) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, d)
}

func (p *progressCollector) errors() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for _, e := range p.events {
		if e.Error != "" {
			out = append(out, e.LocalFile+" -> "+e.RemoteFile+": "+e.Error)
		}
	}
	return out
}

func newTestManager(t *testing.T) (*Manager, *sftp.Client, *progressCollector) {
	client := dialTestClient(t)
	col := &progressCollector{}
	mgr := NewManager(zap.NewNop(), nil, transfer.NewRegistry(col.record), nil)
	// Wire remote exec over a real SSH connection (mirrors ssh.Manager.Exec:
	// one fresh session per command, combined output).
	conn := dialTestSSH(t)
	mgr.remoteExec = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		sess, err := conn.NewSession()
		if err != nil {
			return "", err
		}
		defer sess.Close()
		out, err := sess.CombinedOutput(command)
		s := string(out)
		if maxOut > 0 && len(s) > maxOut {
			s = s[:maxOut]
		}
		return s, err
	}
	mgr.clients.Store("test", client)
	mgr.owners.Store("test", "owner-test")
	return mgr, client, col
}

// remoteRun runs one shell command on the test server (fixture building).
func remoteRun(t *testing.T, mgr *Manager, cmd string) string {
	t.Helper()
	out, err := mgr.runRemoteCommand(context.Background(), "test", cmd, 30*time.Second, 1<<20)
	if err != nil {
		t.Fatalf("remote cmd %q: %v (out=%q)", cmd, err, out)
	}
	return out
}

func waitRemote(t *testing.T, client *sftp.Client, p string, want bool, desc string) {
	waitRemoteCond(t, client, p, func(st os.FileInfo, err error) bool {
		if want {
			return err == nil
		}
		return os.IsNotExist(err)
	}, desc)
}

func waitRemoteCond(t *testing.T, client *sftp.Client, p string, cond func(os.FileInfo, error) bool, desc string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		st, err := client.Stat(p)
		if cond(st, err) {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s (%s)", p, desc)
}

func readRemote(t *testing.T, client *sftp.Client, p string) []byte {
	t.Helper()
	f, err := client.Open(p)
	if err != nil {
		t.Fatalf("open %s: %v", p, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return data
}

func TestIntegrationPasteAndCompress(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	src := path.Join(root, "src")
	dstFile := path.Join(root, "paste-file.txt")
	dstDir := path.Join(root, "paste-dir")
	archive := path.Join(root, "out.tar.gz")

	// clean leftovers
	_ = client.RemoveAll(dstFile)
	_ = client.RemoveAll(dstDir)
	_ = client.Remove(archive)
	_ = client.Remove(path.Join(root, "file-a.txt"))
	_ = client.Remove(path.Join(root, "conflict-check.txt"))
	_ = client.Remove(path.Join(root, "moved.txt"))
	t.Cleanup(func() {
		_ = client.RemoveAll(dstFile)
		_ = client.RemoveAll(dstDir)
		_ = client.Remove(archive)
		_ = client.Remove(path.Join(root, "file-a.txt"))
		_ = client.Remove(path.Join(root, "conflict-check.txt"))
		_ = client.Remove(path.Join(root, "moved.txt"))
	})

	// 1. conflicts: none before paste, one after (without overwrite).
	conflicts, err := mgr.PasteConflicts("test", root, []string{path.Join(src, "file-a.txt")})
	if err != nil {
		t.Fatalf("PasteConflicts: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %v", conflicts)
	}

	// 2. copy single file under a new name.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(src, "file-a.txt")}, []string{"paste-file.txt"}, root, false, false); err != nil {
		t.Fatalf("paste file: %v", err)
	}
	waitRemote(t, client, dstFile, true, "copied file")
	if got := readRemote(t, client, dstFile); string(got) != "hello from vela\n" {
		t.Fatalf("file content mismatch: %q", got)
	}
	conflicts, err = mgr.PasteConflicts("test", root, []string{dstFile})
	if err != nil || len(conflicts) != 1 || conflicts[0] != "paste-file.txt" {
		t.Fatalf("conflicts after copy: %v err %v", conflicts, err)
	}

	// 3. overwrite the file.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(src, "file-a.txt")}, []string{"paste-file.txt"}, root, false, true); err != nil {
		t.Fatalf("paste overwrite: %v", err)
	}
	waitRemote(t, client, dstFile, true, "overwritten file")

	// 4. same-name conflict without overwrite must fail fast on second run:
	// copy to a fresh dest, then try again with overwrite=false expecting error.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(src, "file-a.txt")}, []string{"conflict-check.txt"}, root, false, false); err != nil {
		t.Fatalf("paste fresh name: %v", err)
	}
	waitRemote(t, client, path.Join(root, "conflict-check.txt"), true, "first conflict-check copy")
	err = mgr.PasteRemotePaths("test", []string{path.Join(src, "file-a.txt")}, []string{"conflict-check.txt"}, root, false, false)
	if err == nil {
		t.Fatalf("expected conflict error without overwrite")
	}
	_ = client.Remove(path.Join(root, "conflict-check.txt"))

	// 5. copy directory tree (renamed so the destination differs from the source).
	if err := mgr.PasteRemotePaths("test", []string{src}, []string{"paste-dir"}, root, false, false); err != nil {
		t.Fatalf("paste dir: %v", err)
	}
	waitRemote(t, client, path.Join(dstDir, "sub", "inner.txt"), true, "nested file")
	if got := readRemote(t, client, path.Join(dstDir, "sub", "inner.txt")); string(got) != "nested\n" {
		t.Fatalf("nested content mismatch: %q", got)
	}
	waitRemoteCond(t, client, path.Join(dstDir, "big.bin"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Size() == 1000000
	}, "big file fully copied")
	info, err := client.Stat(path.Join(dstDir, "big.bin"))
	if err != nil || info.Size() != 1000000 {
		t.Fatalf("big.bin size: %d err %v", info.Size(), err)
	}

	// 6. cut: copy + delete source.
	moved := path.Join(root, "moved.txt")
	_ = client.Remove(moved)
	if err := mgr.PasteRemotePaths("test", []string{path.Join(src, "file-b.log")}, []string{"moved.txt"}, root, true, false); err != nil {
		t.Fatalf("paste cut: %v", err)
	}
	waitRemote(t, client, moved, true, "moved file")
	waitRemote(t, client, path.Join(src, "file-b.log"), false, "cut source removed")
	if got := readRemote(t, client, moved); string(got) != "line two\n" {
		t.Fatalf("moved content mismatch: %q", got)
	}
	// restore for reruns
	if err := mgr.PasteRemotePaths("test", []string{moved}, []string{"file-b.log"}, src, true, false); err != nil {
		t.Fatalf("restore moved: %v", err)
	}
	waitRemote(t, client, path.Join(src, "file-b.log"), true, "restored source")

	// 7. compress: build the tar.gz of the src tree on the server, verify
	// entries and contents.
	if err := mgr.CompressRemotePaths("test", []string{src}, archive); err != nil {
		t.Fatalf("compress: %v", err)
	}
	entries := waitTarGzReady(t, client, archive)
	for name, content := range map[string]string{
		"src/file-a.txt":    "hello from vela\n",
		"src/file-b.log":    "line two\n",
		"src/sub/inner.txt": "nested\n",
	} {
		if got := entries[name]; string(got) != content {
			t.Fatalf("tar entry %s: got %q want %q", name, got, content)
		}
	}
	if got := len(entries["src/big.bin"]); got != 1000000 {
		t.Fatalf("tar entry src/big.bin size: %d", got)
	}
	// 8. compress onto existing archive must fail.
	if err := mgr.CompressRemotePaths("test", []string{src}, archive); err == nil {
		t.Fatalf("expected exists error for second compress")
	}

	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// tarGzFiles reads a complete gzip+tar blob into name→content (nil for
// directory entries).
func tarGzFiles(data []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			out[hdr.Name] = content
		} else {
			out[hdr.Name] = nil
		}
	}
}

// waitTarGzReady polls until the remote file parses as a complete gzip+tar
// stream (the compress job is asynchronous).
func waitTarGzReady(t *testing.T, client *sftp.Client, p string) map[string][]byte {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		data := []byte(nil)
		if f, err := client.Open(p); err == nil {
			data, _ = io.ReadAll(f)
			f.Close()
		} else if !os.IsNotExist(err) {
			t.Fatalf("open %s: %v", p, err)
		}
		if len(data) > 0 {
			files, err := tarGzFiles(data)
			if err == nil {
				return files
			}
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("tar.gz never completed: %v", lastErr)
	return nil
}

func TestIntegrationExtract(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	src := path.Join(root, "src")
	archive := path.Join(root, "extract-me.tar.gz")
	extractDir := path.Join(root, "ex")
	_ = client.Remove(archive)
	_ = client.RemoveAll(extractDir)
	t.Cleanup(func() {
		_ = client.Remove(archive)
		_ = client.RemoveAll(extractDir)
	})

	// Build archive from the src directory (single root "src/").
	if err := mgr.CompressRemotePaths("test", []string{src}, archive); err != nil {
		t.Fatalf("compress: %v", err)
	}
	waitTarGzReady(t, client, archive)

	// Fresh target: no conflicts.
	conflicts, err := mgr.ListArchiveConflicts("test", archive, extractDir)
	if err != nil {
		t.Fatalf("ListArchiveConflicts: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts on fresh dir, got %v", conflicts)
	}

	// Extract must flatten: no "src/" level is recreated.
	if err := mgr.ExtractRemoteArchive("test", archive, extractDir, false); err != nil {
		t.Fatalf("extract: %v", err)
	}
	waitRemote(t, client, path.Join(extractDir, "file-a.txt"), true, "flattened file")
	if got := readRemote(t, client, path.Join(extractDir, "file-a.txt")); string(got) != "hello from vela\n" {
		t.Fatalf("extracted content mismatch: %q", got)
	}
	waitRemote(t, client, path.Join(extractDir, "sub", "inner.txt"), true, "flattened nested file")
	if got := readRemote(t, client, path.Join(extractDir, "sub", "inner.txt")); string(got) != "nested\n" {
		t.Fatalf("extracted nested content mismatch: %q", got)
	}
	waitRemoteCond(t, client, path.Join(extractDir, "big.bin"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Size() == 1000000
	}, "extracted big file")
	if _, statErr := client.Stat(path.Join(extractDir, "src")); statErr == nil {
		t.Fatalf("extract recreated the archive root folder; expected flatten")
	}

	// Now everything conflicts; overwrite extract must succeed.
	conflicts, err = mgr.ListArchiveConflicts("test", archive, extractDir)
	if err != nil || len(conflicts) == 0 {
		t.Fatalf("expected conflicts after extract: %v err %v", conflicts, err)
	}
	if err := mgr.ExtractRemoteArchive("test", archive, extractDir, true); err != nil {
		t.Fatalf("overwrite extract: %v", err)
	}
	waitRemoteCond(t, client, path.Join(extractDir, "big.bin"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Size() == 1000000
	}, "overwritten big file")

	// Multi-root archive (two files at archive root): nothing to strip.
	multi := path.Join(root, "multi.tar.gz")
	ex2 := path.Join(root, "ex2")
	_ = client.Remove(multi)
	_ = client.RemoveAll(ex2)
	t.Cleanup(func() {
		_ = client.Remove(multi)
		_ = client.RemoveAll(ex2)
	})
	if err := mgr.CompressRemotePaths("test", []string{path.Join(src, "file-a.txt"), path.Join(src, "file-b.log")}, multi); err != nil {
		t.Fatalf("compress multi: %v", err)
	}
	waitTarGzReady(t, client, multi)
	if err := mgr.ExtractRemoteArchive("test", multi, ex2, false); err != nil {
		t.Fatalf("extract multi: %v", err)
	}
	waitRemote(t, client, path.Join(ex2, "file-a.txt"), true, "multi-root file a")
	waitRemote(t, client, path.Join(ex2, "file-b.log"), true, "multi-root file b")

	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

func writeRemote(t *testing.T, client *sftp.Client, p, content string) {
	t.Helper()
	f, err := client.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", p, err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatalf("write %s: %v", p, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", p, err)
	}
}

// waitRemoteContent polls until the remote file's content equals want; for
// async overwrite jobs, existence alone says nothing about completion.
func waitRemoteContent(t *testing.T, client *sftp.Client, p, want, desc string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if f, err := client.Open(p); err == nil {
			data, _ := io.ReadAll(f)
			f.Close()
			if string(data) == want {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s content (%s)", p, desc)
}

// waitRemoteRegularFile polls until the path is a regular file (not a symlink).
func waitRemoteRegularFile(t *testing.T, client *sftp.Client, p, desc string) {
	t.Helper()
	waitRemoteCond(t, client, p, func(st os.FileInfo, err error) bool {
		return err == nil && st.Mode().IsRegular()
	}, desc)
}

func TestIntegrationPasteMergeAndSelfGuard(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	src := path.Join(root, "src")
	mergeDst := path.Join(root, "merge-dst")
	modeDst := path.Join(root, "mode-dst")

	_ = client.RemoveAll(mergeDst)
	_ = client.RemoveAll(modeDst)
	t.Cleanup(func() {
		_ = client.RemoveAll(mergeDst)
		_ = client.RemoveAll(modeDst)
	})

	// #4: pasting a directory onto itself or into its own subtree is refused.
	if err := mgr.PasteRemotePaths("test", []string{src}, []string{"src"}, root, false, false); err == nil {
		t.Fatal("expected paste onto itself to fail")
	}
	if err := mgr.PasteRemotePaths("test", []string{src}, nil, path.Join(src, "sub"), false, false); err == nil {
		t.Fatal("expected paste into own subdir to fail")
	}

	// Prepare a merge destination: one destination-only file, one same-named
	// file, and its own mode.
	if err := client.MkdirAll(mergeDst); err != nil {
		t.Fatal(err)
	}
	if err := client.Chmod(mergeDst, 0o711); err != nil {
		t.Fatal(err)
	}
	writeRemote(t, client, path.Join(mergeDst, "dest-only.txt"), "keep me\n")
	writeRemote(t, client, path.Join(mergeDst, "file-a.txt"), "stale\n")

	// #1: overwrite-pasting a directory onto an existing same-named directory
	// merges — destination-only entries are kept, same-named files replaced.
	// (dst = root/merge-dst, conflicting with the pasted dir's basename.)
	if err := mgr.PasteRemotePaths("test", []string{src}, []string{"merge-dst"}, root, false, true); err != nil {
		t.Fatalf("paste merge: %v", err)
	}
	waitRemoteContent(t, client, path.Join(mergeDst, "file-a.txt"), "hello from vela\n", "same-named file replaced")
	waitRemoteContent(t, client, path.Join(mergeDst, "sub", "inner.txt"), "nested\n", "merged nested file")
	waitRemoteCond(t, client, path.Join(mergeDst, "big.bin"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Size() == 1000000
	}, "merged big file")
	// The whole walk is done; destination-only entries must have survived.
	waitRemoteContent(t, client, path.Join(mergeDst, "dest-only.txt"), "keep me\n", "dest-only file kept")
	// Merge keeps the destination directory's own mode.
	st, err := client.Stat(mergeDst)
	if err != nil || st.Mode().Perm() != 0o711 {
		t.Fatalf("merge-dst mode = %v err %v", st.Mode().Perm(), err)
	}

	// #8: a fresh copy carries the source root directory's mode.
	if err := client.Chmod(src, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Chmod(src, 0o755) })
	if err := mgr.PasteRemotePaths("test", []string{src}, []string{"mode-dst"}, root, false, false); err != nil {
		t.Fatalf("paste mode: %v", err)
	}
	waitRemote(t, client, path.Join(modeDst, "file-a.txt"), true, "mode copy content")
	st, err = client.Stat(modeDst)
	if err != nil || st.Mode().Perm() != 0o750 {
		t.Fatalf("top dir mode not preserved: %v err %v", st.Mode().Perm(), err)
	}

	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

func TestIntegrationExtractSymlinkSafety(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	ex3 := path.Join(root, "ex3")
	outside := path.Join(root, "outside-dir")
	zipPath := path.Join(root, "evil.zip")

	_ = client.RemoveAll(ex3)
	_ = client.RemoveAll(outside)
	_ = client.Remove(zipPath)
	t.Cleanup(func() {
		_ = client.RemoveAll(ex3)
		_ = client.RemoveAll(outside)
		_ = client.Remove(zipPath)
	})
	if err := client.MkdirAll(outside); err != nil {
		t.Fatal(err)
	}
	if err := client.MkdirAll(ex3); err != nil {
		t.Fatal(err)
	}
	writeRemote(t, client, path.Join(outside, "file2.txt"), "untouched\n")

	// A symlinked parent directory must not be written through (#7), and a
	// symlinked target is replaced as a link, never followed.
	if err := client.Symlink(outside, path.Join(ex3, "evil")); err != nil {
		t.Fatalf("symlink parent: %v", err)
	}
	if err := client.Symlink(path.Join(outside, "file2.txt"), path.Join(ex3, "link.txt")); err != nil {
		t.Fatalf("symlink target: %v", err)
	}

	// One zip, two entries: evil/file.txt (parent is the symlink) and
	// link.txt (target is a symlink). No directory entries at all.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"evil/file.txt": "pwned\n",
		"link.txt":      "replaced\n",
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := client.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := mgr.ExtractRemoteArchive("test", zipPath, ex3, true); err != nil {
		t.Fatalf("extract: %v", err)
	}
	// The run aborts at the symlinked parent (tracker error) and must not
	// have written through it.
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) && len(col.errors()) == 0 {
		time.Sleep(300 * time.Millisecond)
	}
	if errs := col.errors(); len(errs) == 0 {
		t.Fatal("expected symlink-parent extract to fail")
	}
	if _, statErr := client.Stat(path.Join(outside, "file.txt")); statErr == nil {
		t.Fatal("extract wrote through the symlinked parent into outside dir")
	}
	if got := readRemote(t, client, path.Join(outside, "file2.txt")); string(got) != "untouched\n" {
		t.Fatalf("symlink target was followed and modified: %q", got)
	}

	// The same archive without the symlink-parent entry: link.txt alone is
	// extracted, replacing the symlink itself.
	var buf2 bytes.Buffer
	zw2 := zip.NewWriter(&buf2)
	w2, err := zw2.Create("link.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w2.Write([]byte("replaced\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw2.Close(); err != nil {
		t.Fatal(err)
	}
	zip2 := path.Join(root, "link.zip")
	_ = client.Remove(zip2)
	t.Cleanup(func() { _ = client.Remove(zip2) })
	f2, err := client.Create(zip2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f2.Write(buf2.Bytes()); err != nil {
		f2.Close()
		t.Fatal(err)
	}
	if err := f2.Close(); err != nil {
		t.Fatal(err)
	}
	before := len(col.errors())
	if err := mgr.ExtractRemoteArchive("test", zip2, ex3, true); err != nil {
		t.Fatalf("extract link.zip: %v", err)
	}
	// The symlink must be replaced by a regular file holding the zip entry.
	waitRemoteRegularFile(t, client, path.Join(ex3, "link.txt"), "link.txt replaced by regular file")
	waitRemoteContent(t, client, path.Join(ex3, "link.txt"), "replaced\n", "link.txt extracted content")
	st, err := client.Lstat(path.Join(ex3, "link.txt"))
	if err != nil || st.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("link.txt should be a regular file now: %v err %v", st.Mode(), err)
	}
	if got := readRemote(t, client, path.Join(outside, "file2.txt")); string(got) != "untouched\n" {
		t.Fatalf("symlink target was modified: %q", got)
	}
	if errs := col.errors(); len(errs) != before {
		t.Fatalf("unexpected extra transfer errors: %v", errs[before:])
	}
}

// buildZip builds an in-memory zip for client-side fixture upload.
func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildTarGz builds an in-memory tar.gz (arbitrary member names allowed) for
// client-side fixture upload.
func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
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

func uploadBytes(t *testing.T, client *sftp.Client, p string, data []byte) {
	t.Helper()
	f, err := client.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", p, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatalf("write %s: %v", p, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", p, err)
	}
}

// TestIntegrationExtractZipPath covers the streaming zip extract with flatten.
func TestIntegrationExtractZipPath(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	zipPath := path.Join(root, "zipped.zip")
	ex := path.Join(root, "ex-zip")
	_ = client.Remove(zipPath)
	_ = client.RemoveAll(ex)
	t.Cleanup(func() {
		_ = client.Remove(zipPath)
		_ = client.RemoveAll(ex)
	})
	uploadBytes(t, client, zipPath, buildZip(t, map[string]string{
		"src/file-a.txt":    "hello from vela\n",
		"src/sub/inner.txt": "nested\n",
	}))

	conflicts, err := mgr.ListArchiveConflicts("test", zipPath, ex)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("fresh zip conflicts: %v err %v", conflicts, err)
	}
	if err := mgr.ExtractRemoteArchive("test", zipPath, ex, false); err != nil {
		t.Fatalf("extract zip: %v", err)
	}
	waitRemoteContent(t, client, path.Join(ex, "file-a.txt"), "hello from vela\n", "zip flattened file")
	waitRemoteContent(t, client, path.Join(ex, "sub", "inner.txt"), "nested\n", "zip nested file")
	if _, statErr := client.Stat(path.Join(ex, "src")); statErr == nil {
		t.Fatal("zip extract recreated the root folder; expected flatten")
	}
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractFormats extracts the server-side tar variants built by
// the remote tar tool itself: .tgz, .tar.bz2 and .tar.xz.
func TestIntegrationExtractFormats(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"

	type fixture struct {
		file   string
		tarCmd string
		tool   string // required compression tool, "" for plain gzip
	}
	fixtures := []fixture{
		{"fmt.tgz", "tar czf %s -C " + shellQuote(root) + " src", ""},
		{"fmt.tar.bz2", "tar cjf %s -C " + shellQuote(root) + " src", "bzip2"},
		{"fmt.tar.xz", "tar cJf %s -C " + shellQuote(root) + " src", "xz"},
	}
	for _, fx := range fixtures {
		if fx.tool != "" {
			if out, err := mgr.runRemoteCommand(context.Background(), "test", "command -v "+fx.tool, 10*time.Second, 4096); err != nil {
				t.Logf("skip %s: %v (out=%q)", fx.file, err, out)
				continue
			}
		}
		archive := path.Join(root, fx.file)
		ex := path.Join(root, "ex-"+fx.file)
		_ = client.Remove(archive)
		_ = client.RemoveAll(ex)
		t.Cleanup(func() {
			_ = client.Remove(archive)
			_ = client.RemoveAll(ex)
		})
		remoteRun(t, mgr, strings.Replace(fx.tarCmd, "%s", shellQuote(archive), 1))

		conflicts, err := mgr.ListArchiveConflicts("test", archive, ex)
		if err != nil || len(conflicts) != 0 {
			t.Fatalf("%s conflicts: %v err %v", fx.file, conflicts, err)
		}
		if err := mgr.ExtractRemoteArchive("test", archive, ex, false); err != nil {
			t.Fatalf("extract %s: %v", fx.file, err)
		}
		waitRemoteContent(t, client, path.Join(ex, "file-a.txt"), "hello from vela\n", fx.file+" flattened file")
		waitRemoteContent(t, client, path.Join(ex, "sub", "inner.txt"), "nested\n", fx.file+" nested file")
		if _, statErr := client.Stat(path.Join(ex, "src")); statErr == nil {
			t.Fatalf("%s extract recreated the root folder; expected flatten", fx.file)
		}
	}
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractSingleStreamGz decompresses a bare .gz on the server.
func TestIntegrationExtractSingleStreamGz(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	gzPath := path.Join(root, "notes.txt.gz")
	ex := path.Join(root, "ex-gz")
	_ = client.Remove(gzPath)
	_ = client.RemoveAll(ex)
	t.Cleanup(func() {
		_ = client.Remove(gzPath)
		_ = client.RemoveAll(ex)
	})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("hello gz\n")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	uploadBytes(t, client, gzPath, buf.Bytes())

	// A same-named target file shows the detected name in the conflict list.
	if err := client.MkdirAll(ex); err != nil {
		t.Fatal(err)
	}
	writeRemote(t, client, path.Join(ex, "notes.txt"), "stale\n")
	conflicts, err := mgr.ListArchiveConflicts("test", gzPath, ex)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "notes.txt" {
		t.Fatalf("gz conflicts = %v err %v", conflicts, err)
	}
	if err := mgr.ExtractRemoteArchive("test", gzPath, ex, true); err != nil {
		t.Fatalf("extract gz: %v", err)
	}
	waitRemoteContent(t, client, path.Join(ex, "notes.txt"), "hello gz\n", "gunzipped file")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractUnsafeTar refuses a tar whose members escape the
// destination before running any remote extract.
func TestIntegrationExtractUnsafeTar(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	evil := path.Join(root, "evil.tar.gz")
	ex := path.Join(root, "ex-evil")
	_ = client.Remove(evil)
	_ = client.RemoveAll(ex)
	_ = client.Remove(path.Join(root, "evil-escaped.txt"))
	t.Cleanup(func() {
		_ = client.Remove(evil)
		_ = client.RemoveAll(ex)
		_ = client.Remove(path.Join(root, "evil-escaped.txt"))
	})
	uploadBytes(t, client, evil, buildTarGz(t, map[string]string{
		"../evil-escaped.txt": "pwned\n",
		"ok.txt":              "fine\n",
	}))

	_, err := mgr.ListArchiveConflicts("test", evil, ex)
	if usererr.CodeOf(err) != "sftp.archive_unsafe_entries" {
		t.Fatalf("unsafe listing error = %v", err)
	}
	// Extract refuses the whole archive up front, before any remote command.
	err = mgr.ExtractRemoteArchive("test", evil, ex, false)
	if usererr.CodeOf(err) != "sftp.archive_unsafe_entries" {
		t.Fatalf("unsafe extract error = %v", err)
	}
	if _, statErr := client.Stat(path.Join(root, "evil-escaped.txt")); statErr == nil {
		t.Fatal("extract wrote ../evil-escaped.txt outside the destination")
	}
	if _, statErr := client.Stat(path.Join(ex, "ok.txt")); statErr == nil {
		t.Fatal("the whole operation must be refused before extracting anything")
	}
}

// TestIntegrationArchiveCorrupt maps a real tool failure (corrupt archive,
// non-zero exit) to the remote_command_failed error code.
func TestIntegrationArchiveCorrupt(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	bad := path.Join(root, "bad.tar.gz")
	_ = client.Remove(bad)
	t.Cleanup(func() { _ = client.Remove(bad) })
	uploadBytes(t, client, bad, []byte("this is not a tarball"))

	_, err := mgr.ListArchiveConflicts("test", bad, root)
	if usererr.CodeOf(err) != "sftp.remote_command_failed" {
		t.Fatalf("corrupt archive error = %v", err)
	}
}

// TestIntegrationCompressDashName keeps a leading-dash source name out of tar's
// option parser and round-trips it through extract.
func TestIntegrationCompressDashName(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	dashFile := path.Join(root, "-odd.txt")
	archive := path.Join(root, "dash.tar.gz")
	ex := path.Join(root, "ex-dash")
	_ = client.Remove(dashFile)
	_ = client.Remove(archive)
	_ = client.RemoveAll(ex)
	t.Cleanup(func() {
		_ = client.Remove(dashFile)
		_ = client.Remove(archive)
		_ = client.RemoveAll(ex)
	})
	writeRemote(t, client, dashFile, "odd content\n")

	if err := mgr.CompressRemotePaths("test", []string{dashFile}, archive); err != nil {
		t.Fatalf("compress: %v", err)
	}
	entries := waitTarGzReady(t, client, archive)
	got := entries["./-odd.txt"]
	if got == nil {
		got = entries["-odd.txt"]
	}
	if string(got) != "odd content\n" {
		t.Fatalf("dash member content = %q; entries=%v", got, entries)
	}

	if err := mgr.ExtractRemoteArchive("test", archive, ex, false); err != nil {
		t.Fatalf("extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(ex, "-odd.txt"), "odd content\n", "dash-named file extracted")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationPasteRemoteCommandPath verifies the fast paste path: one
// silent remote cp/mv command per job, with the data staying on the server.
func TestIntegrationPasteRemoteCommandPath(t *testing.T) {
	mgr, client, col := newTestManager(t)
	var mu sync.Mutex
	var cmds []string
	inner := mgr.remoteExec
	mgr.remoteExec = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		mu.Lock()
		cmds = append(cmds, command)
		mu.Unlock()
		return inner(ctx, sessionID, command, timeout, maxOut)
	}
	recorded := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := append([]string(nil), cmds...)
		cmds = cmds[:0]
		return out
	}
	root := "/root/sftp-test"
	work := path.Join(root, "paste-cmd")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/mg-src "+work+"/dst/mg")
	writeRemote(t, client, path.Join(work, "a.txt"), "alpha\n")
	writeRemote(t, client, path.Join(work, "b.txt"), "beta\n")
	writeRemote(t, client, path.Join(work, "mg-src", "one.txt"), "one\n")
	writeRemote(t, client, path.Join(work, "dst", "mg", "zero.txt"), "zero\n")
	recorded() // drain fixture commands

	// Copy file: one cp staging to a hidden sibling, then rename.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "a.txt")}, nil, path.Join(work, "dst"), false, false); err != nil {
		t.Fatalf("copy file: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "a.txt"), "alpha\n", "file copied by remote command")
	if got := recorded(); len(got) == 0 || !strings.Contains(got[0], "cp -a") {
		t.Fatalf("copy file commands = %v", got)
	}

	// Cut file: one mv, source gone.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "b.txt")}, nil, path.Join(work, "dst"), true, false); err != nil {
		t.Fatalf("cut file: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "b.txt"), "beta\n", "file moved by remote command")
	waitRemote(t, client, path.Join(work, "b.txt"), false, "cut source removed")
	if got := recorded(); len(got) == 0 || !strings.Contains(got[0], "mv -f") {
		t.Fatalf("cut file commands = %v", got)
	}

	// Copy dir onto a same-named dir: merge keeps destination-only entries and
	// runs on the staged SFTP path — never a remote cp -a, which would broaden
	// existing destination directory modes and write replacement bytes over
	// existing destination files non-atomically.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "mg-src")}, []string{"mg"}, path.Join(work, "dst"), false, true); err != nil {
		t.Fatalf("copy dir merge: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "mg", "one.txt"), "one\n", "merged source entry")
	waitRemoteContent(t, client, path.Join(work, "dst", "mg", "zero.txt"), "zero\n", "destination-only entry kept")
	if got := recorded(); len(got) != 0 {
		t.Fatalf("merge must run no remote commands, got %v", got)
	}

	// Cut dir onto the merged target: the source tree disappears after the
	// staged merge succeeded, deleted over SFTP (no rm command).
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "mg-src")}, []string{"mg"}, path.Join(work, "dst"), true, true); err != nil {
		t.Fatalf("cut dir merge: %v", err)
	}
	waitRemote(t, client, path.Join(work, "mg-src"), false, "cut dir source removed")
	if got := recorded(); len(got) != 0 {
		t.Fatalf("cut merge must run no remote commands, got %v", got)
	}
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationPasteCommandFallbackToSFTP keeps the streaming SFTP paste
// working when remote command execution is unavailable on the server.
func TestIntegrationPasteCommandFallbackToSFTP(t *testing.T) {
	mgr, client, col := newTestManager(t)
	mgr.remoteExec = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		return "", usererr.Coded("test.exec_unavailable", "exec unavailable")
	}
	root := "/root/sftp-test"
	work := path.Join(root, "paste-fb")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	if err := client.MkdirAll(path.Join(work, "dst")); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	writeRemote(t, client, path.Join(work, "a.txt"), "alpha\n")
	writeRemote(t, client, path.Join(work, "b.txt"), "beta\n")

	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "a.txt")}, nil, path.Join(work, "dst"), false, false); err != nil {
		t.Fatalf("fallback copy: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "a.txt"), "alpha\n", "fallback file copied")
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "b.txt")}, nil, path.Join(work, "dst"), true, false); err != nil {
		t.Fatalf("fallback cut: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "b.txt"), "beta\n", "fallback file moved")
	waitRemote(t, client, path.Join(work, "b.txt"), false, "fallback cut source removed")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// ---- review round 2: merge staging, archive name decoding, extract staging ----

// assertNoStagingLeftovers fails when any staged-swap artifact remains in dir.
func assertNoStagingLeftovers(t *testing.T, client *sftp.Client, dir string) {
	t.Helper()
	entries, err := client.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "vela-edit") {
			t.Fatalf("staging leftover %s in %s", e.Name(), dir)
		}
	}
}

// TestIntegrationPasteMergeModeSemantics: the staged SFTP merge must not
// broaden existing destination directory modes (the old remote cp -a copied
// source modes over destination subdirectories) and must keep dest-only
// entries while replacing same-named files. Also covers a read-only (0555)
// source tree being copied with its mode applied only after children, and a
// cut of a whole directory tree on the fallback path.
func TestIntegrationPasteMergeModeSemantics(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "paste-mode")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/src/sub "+work+"/dst/mg/sub")
	writeRemote(t, client, path.Join(work, "src", "sub", "new.txt"), "new\n")
	writeRemote(t, client, path.Join(work, "src", "rep.txt"), "replacement\n")
	writeRemote(t, client, path.Join(work, "dst", "mg", "sub", "old.txt"), "old\n")
	writeRemote(t, client, path.Join(work, "dst", "mg", "rep.txt"), "original\n")
	remoteRun(t, mgr, "chmod 700 "+work+"/dst/mg "+work+"/dst/mg/sub && chmod 555 "+work+"/src")
	// Force the staged SFTP path even though exec works on this server.
	mgr.remoteExec = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		return "", usererr.Coded("test.exec_unavailable", "exec unavailable")
	}

	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "src")}, []string{"mg"}, path.Join(work, "dst"), false, true); err != nil {
		t.Fatalf("merge: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "mg", "rep.txt"), "replacement\n", "merged file replaced")
	waitRemoteContent(t, client, path.Join(work, "dst", "mg", "sub", "new.txt"), "new\n", "merged nested entry")
	waitRemoteContent(t, client, path.Join(work, "dst", "mg", "sub", "old.txt"), "old\n", "destination-only entry kept")
	for _, d := range []string{"mg", "mg/sub"} {
		st, statErr := client.Stat(path.Join(work, "dst", d))
		if statErr != nil {
			t.Fatalf("stat %s: %v", d, statErr)
		}
		if perm := st.Mode().Perm(); perm != 0o700 {
			t.Fatalf("destination dir %s mode broadened to %o", d, perm)
		}
	}

	// A 0555 source copies fine (mode applied after children) and keeps 0555.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "src")}, []string{"fresh"}, path.Join(work, "dst"), false, false); err != nil {
		t.Fatalf("copy 0555 dir: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "fresh", "rep.txt"), "replacement\n", "0555 source copied")
	st, statErr := client.Stat(path.Join(work, "dst", "fresh"))
	if statErr != nil {
		t.Fatalf("stat fresh: %v", statErr)
	}
	if perm := st.Mode().Perm(); perm != 0o555 {
		t.Fatalf("fresh dir mode = %o, want 555", perm)
	}

	// Cut of a whole tree on the fallback path: source disappears recursively.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "src")}, []string{"moved"}, path.Join(work, "dst"), true, false); err != nil {
		t.Fatalf("cut dir: %v", err)
	}
	waitRemote(t, client, path.Join(work, "src"), false, "cut source tree removed")
	waitRemoteContent(t, client, path.Join(work, "dst", "moved", "sub", "new.txt"), "new\n", "cut content arrived")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractTarNoOverwriteKeepsExisting: with overwrite false the
// remote tar keeps existing targets (-k) instead of silently replacing a file
// that appeared after the conflict scan; the original survives the failed run.
func TestIntegrationExtractTarNoOverwriteKeepsExisting(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-nw")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/ex")
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGz(t, map[string]string{"a.txt": "new\n"}))
	writeRemote(t, client, path.Join(work, "ex", "a.txt"), "old\n")

	err := mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, path.Join(work, "ex"), false, false)
	if usererr.CodeOf(err) != "sftp.remote_command_failed" {
		t.Fatalf("no-overwrite extract error = %v, want remote_command_failed", err)
	}
	waitRemoteContent(t, client, path.Join(work, "ex", "a.txt"), "old\n", "existing target preserved")

	if err := mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, path.Join(work, "ex"), false, true); err != nil {
		t.Fatalf("overwrite extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "ex", "a.txt"), "new\n", "overwrite extract replaced")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractSingleStreamStaging: a bare compressed stream
// decompresses to a staging sibling and is renamed into place — a symlink
// planted at the target is replaced, never written through, and with overwrite
// false an existing target is left untouched.
func TestIntegrationExtractSingleStreamStaging(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-single")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work)
	writeRemote(t, client, path.Join(work, "plain.txt"), "data\n")
	writeRemote(t, client, path.Join(work, "victim.txt"), "VICTIM\n")
	remoteRun(t, mgr, "gzip -c "+work+"/plain.txt > "+work+"/f.txt.gz")
	remoteRun(t, mgr, "ln -s "+work+"/victim.txt "+work+"/f.txt")

	if err := mgr.ExtractRemoteArchive("test", path.Join(work, "f.txt.gz"), work, true); err != nil {
		t.Fatalf("extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "f.txt"), "data\n", "symlink entry replaced by decompressed file")
	waitRemoteContent(t, client, path.Join(work, "victim.txt"), "VICTIM\n", "symlink target never written through")
	assertNoStagingLeftovers(t, client, work)

	writeRemote(t, client, path.Join(work, "g.txt"), "keep\n")
	remoteRun(t, mgr, "gzip -c "+work+"/plain.txt > "+work+"/g.txt.gz")
	err := mgr.extractSingleRemote(context.Background(), "test", formatGz, path.Join(work, "g.txt.gz"), work, "g.txt", false)
	if usererr.CodeOf(err) != "sftp.remote_command_failed" {
		t.Fatalf("no-overwrite single extract error = %v, want remote_command_failed", err)
	}
	waitRemoteContent(t, client, path.Join(work, "g.txt"), "keep\n", "existing target preserved on no-overwrite")
	assertNoStagingLeftovers(t, client, work)
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractCorruptSingleKeepsOriginal: a failed decompression
// removes its staging output and never touches the existing target.
func TestIntegrationExtractCorruptSingleKeepsOriginal(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-corrupt")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work)
	uploadBytes(t, client, path.Join(work, "bad.txt.gz"), []byte("this is not gzip data"))
	writeRemote(t, client, path.Join(work, "bad.txt"), "original\n")

	err := mgr.extractSingleRemote(context.Background(), "test", formatGz, path.Join(work, "bad.txt.gz"), work, "bad.txt", true)
	if usererr.CodeOf(err) != "sftp.remote_command_failed" {
		t.Fatalf("corrupt extract error = %v, want remote_command_failed", err)
	}
	waitRemoteContent(t, client, path.Join(work, "bad.txt"), "original\n", "original preserved after failed decompression")
	assertNoStagingLeftovers(t, client, work)
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationArchiveWeirdNamesRoundTrip: member names containing a
// backslash, a newline or spaces survive compress -> list -> extract
// unchanged (the listing is escape-quoted and decoded back).
func TestIntegrationArchiveWeirdNamesRoundTrip(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-weird")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work)
	names := []string{`c\d`, "a\nb", "sp ace.txt"}
	var srcs []string
	for i, name := range names {
		p := path.Join(work, name)
		writeRemote(t, client, p, "content-"+string(rune('0'+i))+"\n")
		srcs = append(srcs, p)
	}
	arc := path.Join(work, "weird.tar.gz")
	if err := mgr.CompressRemotePaths("test", srcs, arc); err != nil {
		t.Fatalf("compress: %v", err)
	}
	waitTarGzReady(t, client, arc)
	ex := path.Join(work, "ex")
	if err := mgr.ExtractRemoteArchive("test", arc, ex, false); err != nil {
		t.Fatalf("extract: %v", err)
	}
	for i, name := range names {
		waitRemoteContent(t, client, path.Join(ex, name), "content-"+string(rune('0'+i))+"\n", "weird name round-trip: "+name)
	}
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractDotRootMembers: archives whose members start with
// "./" extract with the same paths the conflict check lists — tar counts the
// "." as the stripped root component.
func TestIntegrationExtractDotRootMembers(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-dot")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/src")
	arc := path.Join(work, "dot.tar.gz")
	uploadBytes(t, client, arc, buildTarGz(t, map[string]string{"./src/a.txt": "dot\n", "./other.txt": "other\n"}))

	// Pre-create the flattened targets: the conflict check must report exactly
	// the paths extraction writes (the "." root is the stripped component).
	writeRemote(t, client, path.Join(work, "src", "a.txt"), "old\n")
	writeRemote(t, client, path.Join(work, "other.txt"), "old\n")
	conflicts, err := mgr.ListArchiveConflicts("test", arc, work)
	if err != nil {
		t.Fatalf("conflicts: %v", err)
	}
	seen := map[string]bool{}
	for _, c := range conflicts {
		seen[c] = true
	}
	if len(conflicts) != 2 || !seen["other.txt"] || !seen["src/a.txt"] {
		t.Fatalf("conflict paths = %v, want other.txt and src/a.txt", conflicts)
	}
	if err := mgr.ExtractRemoteArchive("test", arc, work, true); err != nil {
		t.Fatalf("extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "src", "a.txt"), "dot\n", "dot-rooted member extracted to listed path")
	waitRemoteContent(t, client, path.Join(work, "other.txt"), "other\n", "dot-rooted flat member extracted")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationPasteCutDotSpellingRejected: a cut whose source is spelled
// equivalently to the destination (dots, double slashes, trailing slash) is
// rejected up front — before the old flow could delete the "source" after a
// failed rename destroyed the destination.
func TestIntegrationPasteCutDotSpellingRejected(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-equiv")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/equiv")
	writeRemote(t, client, path.Join(work, "equiv", "a.txt"), "content\n")

	err := mgr.PasteRemotePaths("test", []string{path.Join(work, "equiv", ".", "a.txt")}, []string{"a.txt"}, path.Join(work, "equiv"), true, false)
	if usererr.CodeOf(err) != "sftp.paste_into_self" {
		t.Fatalf("equivalent-path cut error = %v, want paste_into_self", err)
	}
	waitRemoteContent(t, client, path.Join(work, "equiv", "a.txt"), "content\n", "source untouched by rejected cut")
}

// TestIntegrationExtractZipAppliesMode: extracted zip entries keep the
// archived permission bits instead of the server default (e.g. 0600 stays
// private).
func TestIntegrationExtractZipAppliesMode(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-zipmode")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "p.txt", Method: zip.Deflate}
	h.SetMode(0o600)
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write([]byte("secret\n")); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	arc := path.Join(work, "perm.zip")
	uploadBytes(t, client, arc, buf.Bytes())

	if err := mgr.ExtractRemoteArchive("test", arc, work, false); err != nil {
		t.Fatalf("extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "p.txt"), "secret\n", "zip entry extracted")
	st, statErr := client.Stat(path.Join(work, "p.txt"))
	if statErr != nil {
		t.Fatalf("stat p.txt: %v", statErr)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("zip entry mode = %o, want 600", perm)
	}
	assertNoStagingLeftovers(t, client, work)
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}
