//go:build integration

package sftp

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"github.com/pkg/sftp"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/transfer"
)

// Integration harness against a real SSH/SFTP server.
// Set VELA_SFTP_TEST_ADDR (host:port), VELA_SFTP_TEST_USER, VELA_SFTP_TEST_PASS
// and build with: go test -tags integration ./internal/sftp/ -run TestIntegration -v
func dialTestClient(t *testing.T) *sftp.Client {
	addr := os.Getenv("VELA_SFTP_TEST_ADDR")
	user := os.Getenv("VELA_SFTP_TEST_USER")
	pass := os.Getenv("VELA_SFTP_TEST_PASS")
	if addr == "" || user == "" || pass == "" {
		t.Skip("integration env vars not set")
	}
	cfg := &gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{gossh.Password(pass)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	conn, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("ssh dial %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
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
	mgr.clients.Store("test", client)
	mgr.owners.Store("test", "owner-test")
	return mgr, client, col
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
	archive := path.Join(root, "out.zip")

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

	// 7. compress: zip src tree, verify entries and contents.
	if err := mgr.CompressRemotePaths("test", []string{src}, archive); err != nil {
		t.Fatalf("compress: %v", err)
	}
	waitRemote(t, client, archive, true, "archive")
	// The zip's central directory is written on Close; poll until the remote
	// file parses as a complete archive rather than trusting existence.
	var data []byte
	var zr2 *zip.Reader
	var parseErr error
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		data = readRemote(t, client, archive)
		if len(data) == 0 {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		zr2, parseErr = zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if parseErr == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if parseErr != nil {
		t.Fatalf("zip never completed: %v (head=%q tail=%q)", parseErr, data[:min(8, len(data))], data[max(0, len(data)-8):])
	}
	want := map[string]uint64{
		"src/file-a.txt":    uint64(len("hello from vela\n")),
		"src/file-b.log":    uint64(len("line two\n")),
		"src/sub/inner.txt": uint64(len("nested\n")),
		"src/big.bin":       1000000,
		// Non-empty directories are implied by their contents; only empty dirs
		// get an explicit "name/" entry.
	}
	seen := map[string]uint64{}
	for _, f := range zr2.File {
		seen[f.Name] = f.UncompressedSize64
		if f.Name == "src/big.bin" || f.Name == "src/file-a.txt" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", f.Name, err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			if f.Name == "src/file-a.txt" && string(content) != "hello from vela\n" {
				t.Fatalf("zip content mismatch: %q", content)
			}
			if f.Name == "src/big.bin" && int64(len(content)) != 1000000 {
				t.Fatalf("zip big.bin size: %d", len(content))
			}
		}
	}
	for name, size := range want {
		if got, ok := seen[name]; !ok || got != size {
			t.Fatalf("zip entry %s: got (%d,%v) want %d; seen=%v", name, got, ok, size, seen)
		}
	}

	// 8. compress onto existing archive must fail.
	if err := mgr.CompressRemotePaths("test", []string{src}, archive); err == nil {
		t.Fatalf("expected exists error for second compress")
	}

	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// waitZipReady polls until the remote file parses as a complete zip.
func waitZipReady(t *testing.T, client *sftp.Client, p string) (*zip.Reader, []byte) {
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
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err == nil {
				return zr, data
			}
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("zip never completed: %v", lastErr)
	return nil, nil
}

func TestIntegrationExtract(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	src := path.Join(root, "src")
	archive := path.Join(root, "extract-me.zip")
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
	waitZipReady(t, client, archive)

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
	multi := path.Join(root, "multi.zip")
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
	waitZipReady(t, client, multi)
	if err := mgr.ExtractRemoteArchive("test", multi, ex2, false); err != nil {
		t.Fatalf("extract multi: %v", err)
	}
	waitRemote(t, client, path.Join(ex2, "file-a.txt"), true, "multi-root file a")
	waitRemote(t, client, path.Join(ex2, "file-b.log"), true, "multi-root file b")

	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}
