//go:build integration

package sftp

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/pkg/sftp"

	"github.com/ilaziness/vela/internal/usererr"
)

// buildTarGzEntries builds a tar.gz with possibly repeated member names in
// the given order (an archive updated with tar -r keeps several versions of
// one pathname).
func buildTarGzEntries(t *testing.T, members [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		hdr := &tar.Header{Name: m[0], Mode: 0o644, Size: int64(len(m[1])), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header %s: %v", m[0], err)
		}
		if _, err := tw.Write([]byte(m[1])); err != nil {
			t.Fatalf("tar write %s: %v", m[0], err)
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

// waitRemoteEntry waits for a path entry with Lstat semantics: a dangling
// symlink exists as an entry even though Stat (which follows links) fails on
// it.
func waitRemoteEntry(t *testing.T, client *sftp.Client, p, desc string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.Lstat(p); err == nil {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s (%s)", p, desc)
}

// TestIntegrationPasteCutSymlinkDotDot: a cut of link/../file must act on the
// entry the server resolves — POSIX resolves ".." after following symlinks, so
// /srv/link/../config is /data/config, not /srv/config. The lexical
// path.Clean spelling picked the wrong file before: it moved the neighbor and
// a cut could even delete the wrong source.
func TestIntegrationPasteCutSymlinkDotDot(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "paste-linkdot")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/data/sub "+work+"/srv "+work+"/dst")
	writeRemote(t, client, path.Join(work, "data", "config"), "DATA\n")
	writeRemote(t, client, path.Join(work, "srv", "config"), "SRV\n")
	remoteRun(t, mgr, "ln -s ../data/sub "+work+"/srv/link")

	src := work + "/srv/link/../config"

	// Self-guard: the spelling resolves to /data/config, so cutting it into
	// /data must be rejected up front.
	err := mgr.PasteRemotePaths("test", []string{src}, []string{"config"}, path.Join(work, "data"), true, true)
	if usererr.CodeOf(err) != "sftp.paste_into_self" {
		t.Fatalf("self cut error = %v, want paste_into_self", err)
	}
	waitRemoteContent(t, client, path.Join(work, "data", "config"), "DATA\n", "self cut left source")
	waitRemoteContent(t, client, path.Join(work, "srv", "config"), "SRV\n", "self cut left neighbor")

	// The actual cut: DATA content moves to /dst/config, /srv/config stays.
	if err := mgr.PasteRemotePaths("test", []string{src}, []string{"config"}, path.Join(work, "dst"), true, true); err != nil {
		t.Fatalf("cut via link spelling: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "config"), "DATA\n", "resolved entry moved")
	waitRemote(t, client, path.Join(work, "data", "config"), false, "resolved source removed")
	waitRemoteContent(t, client, path.Join(work, "srv", "config"), "SRV\n", "same-spelled neighbor untouched")
	waitRemoteEntry(t, client, path.Join(work, "srv", "link"), "link itself untouched")
	if got, rerr := client.ReadLink(path.Join(work, "srv", "link")); rerr != nil || got != "../data/sub" {
		t.Fatalf("link text = %q, %v; want ../data/sub", got, rerr)
	}
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationPasteMergeSymlinks: merging a tree must recreate every
// symlink as a link (verbatim link text) instead of opening its target:
// directory links, dangling links and relative file links survive unchanged.
// Merges run on the SFTP fallback path; the lone-link case runs the command
// path (cp -a / mv). Cut merges recreate links as well.
func TestIntegrationPasteMergeSymlinks(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "paste-merge-link")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/merge-src/releases "+work+"/merge-dst/merge-src "+work+"/merge-dst2/merge-src")
	writeRemote(t, client, path.Join(work, "merge-src", "a.txt"), "A\n")
	writeRemote(t, client, path.Join(work, "merge-src", "releases", "v.txt"), "V\n")
	writeRemote(t, client, path.Join(work, "outside.txt"), "O\n")
	writeRemote(t, client, path.Join(work, "merge-dst", "merge-src", "keep.txt"), "K\n")
	writeRemote(t, client, path.Join(work, "merge-dst2", "merge-src", "keep.txt"), "K\n")
	remoteRun(t, mgr, "ln -s releases "+work+"/merge-src/current")
	remoteRun(t, mgr, "ln -s nowhere "+work+"/merge-src/dangling")
	remoteRun(t, mgr, "ln -s ../outside.txt "+work+"/merge-src/rel")

	assertLinks := func(dstDir, desc string) {
		t.Helper()
		waitRemoteContent(t, client, path.Join(dstDir, "a.txt"), "A\n", desc+": file")
		waitRemoteContent(t, client, path.Join(dstDir, "releases", "v.txt"), "V\n", desc+": dir child")
		for _, l := range []struct{ name, target string }{
			{"current", "releases"},
			{"dangling", "nowhere"},
			{"rel", "../outside.txt"},
		} {
			p := path.Join(dstDir, l.name)
			waitRemoteEntry(t, client, p, desc+": "+l.name+" exists")
			if got, err := client.ReadLink(p); err != nil || got != l.target {
				t.Fatalf("%s: ReadLink(%s) = %q, %v; want %q", desc, p, got, err, l.target)
			}
		}
		waitRemoteContent(t, client, path.Join(dstDir, "keep.txt"), "K\n", desc+": kept")
	}

	// Copy-merge into an existing directory.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "merge-src")}, []string{"merge-src"}, path.Join(work, "merge-dst"), false, true); err != nil {
		t.Fatalf("copy merge: %v", err)
	}
	assertLinks(path.Join(work, "merge-dst", "merge-src"), "copy-merge")

	// Cut-merge into another existing directory.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "merge-src")}, []string{"merge-src"}, path.Join(work, "merge-dst2"), true, true); err != nil {
		t.Fatalf("cut merge: %v", err)
	}
	assertLinks(path.Join(work, "merge-dst2", "merge-src"), "cut-merge")
	waitRemote(t, client, path.Join(work, "merge-src"), false, "cut source removed")

	// A lone top-level symlink is recreated as a link (command path).
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "merge-dst2", "merge-src", "current")}, []string{"linkcopy"}, path.Join(work, "merge-dst2"), false, true); err != nil {
		t.Fatalf("copy link leaf: %v", err)
	}
	waitRemoteEntry(t, client, path.Join(work, "merge-dst2", "linkcopy"), "link leaf exists")
	if got, err := client.ReadLink(path.Join(work, "merge-dst2", "linkcopy")); err != nil || got != "releases" {
		t.Fatalf("link leaf ReadLink = %q, %v; want releases", got, err)
	}
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractPreservesTargetMode: extracting over an existing file
// must never broaden its permissions. The single-stream path (staged
// decompression) and the zip path (entry without a recorded mode) keep the
// replaced file's mode; only entries with a recorded Unix mode apply their
// own bits.
func TestIntegrationExtractPreservesTargetMode(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-mode")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/ex "+work+"/exz")

	// Single stream: gzip file over an existing 0600 target.
	writeRemote(t, client, path.Join(work, "ex", "f.txt"), "old\n")
	remoteRun(t, mgr, "chmod 600 "+work+"/ex/f.txt")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write([]byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	uploadBytes(t, client, path.Join(work, "f.txt.gz"), buf.Bytes())
	if err := mgr.ExtractRemoteArchive("test", path.Join(work, "f.txt.gz"), path.Join(work, "ex"), true); err != nil {
		t.Fatalf("gzip extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "ex", "f.txt"), "new\n", "gzip replaced content")
	waitRemoteCond(t, client, path.Join(work, "ex", "f.txt"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Mode().Perm() == 0o600
	}, "gzip extract kept 0600")

	// Zip entry without a recorded mode over an existing 0600 target.
	writeRemote(t, client, path.Join(work, "exz", "p.txt"), "old\n")
	remoteRun(t, mgr, "chmod 600 "+work+"/exz/p.txt")
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	w, err := zw.Create("p.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	uploadBytes(t, client, path.Join(work, "p.zip"), zbuf.Bytes())
	if err := mgr.ExtractRemoteArchive("test", path.Join(work, "p.zip"), path.Join(work, "exz"), true); err != nil {
		t.Fatalf("zip extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "exz", "p.txt"), "new\n", "zip replaced content")
	waitRemoteCond(t, client, path.Join(work, "exz", "p.txt"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Mode().Perm() == 0o600
	}, "zip extract kept 0600")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationPastePrivateDirMode: a freshly copied 0700 directory ends at
// the exact source mode on both the command path (cp -a) and the SFTP fallback
// (fresh directory is restricted while being populated and finalized to the
// source mode).
func TestIntegrationPastePrivateDirMode(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "paste-priv")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/priv-src "+work+"/dst "+work+"/dst2")
	writeRemote(t, client, path.Join(work, "priv-src", "secret.txt"), "S\n")
	remoteRun(t, mgr, "chmod 700 "+work+"/priv-src")

	// Command path (exec available).
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "priv-src")}, []string{"priv"}, path.Join(work, "dst"), false, false); err != nil {
		t.Fatalf("copy private dir (command): %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "priv", "secret.txt"), "S\n", "private dir copied (command)")
	waitRemoteCond(t, client, path.Join(work, "dst", "priv"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Mode().Perm() == 0o700
	}, "private dir keeps 0700 (command)")

	// SFTP fallback path (exec unavailable).
	mgr.remoteExec = func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error) {
		return "", usererr.Coded("test.exec_unavailable", "exec unavailable")
	}
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "priv-src")}, []string{"priv"}, path.Join(work, "dst2"), false, false); err != nil {
		t.Fatalf("copy private dir (fallback): %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst2", "priv", "secret.txt"), "S\n", "private dir copied (fallback)")
	waitRemoteCond(t, client, path.Join(work, "dst2", "priv"), func(st os.FileInfo, err error) bool {
		return err == nil && st.Mode().Perm() == 0o700
	}, "private dir keeps 0700 (fallback)")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}

// TestIntegrationExtractTarRepeatedMembers: an archive updated with tar -r
// stores several versions of one pathname. With overwrite false the extraction
// must not abort on the repeated member (a plain tar -k did) and must restore
// the newest content — nested members included. The overwrite path keeps
// last-member-wins semantics too.
func TestIntegrationExtractTarRepeatedMembers(t *testing.T) {
	mgr, client, col := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "ex-repeat")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/ex "+work+"/ex2")
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzEntries(t, [][2]string{
		{"root/a.txt", "old\n"},
		{"root/a.txt", "new\n"},
		{"root/src/b.txt", "b-old\n"},
		{"root/src/b.txt", "b-new\n"},
	}))
	if err := mgr.ExtractRemoteArchive("test", arc, path.Join(work, "ex"), false); err != nil {
		t.Fatalf("repeated-member extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "ex", "a.txt"), "new\n", "last member wins")
	waitRemoteContent(t, client, path.Join(work, "ex", "src", "b.txt"), "b-new\n", "nested last member wins")

	// Overwrite=true keeps plain last-member-wins semantics; repeated
	// top-level members are not a strip root.
	uploadBytes(t, client, arc, buildTarGzEntries(t, [][2]string{
		{"a.txt", "v1\n"},
		{"a.txt", "v2\n"},
	}))
	if err := mgr.ExtractRemoteArchive("test", arc, path.Join(work, "ex2"), true); err != nil {
		t.Fatalf("overwrite repeated-member extract: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "ex2", "a.txt"), "v2\n", "overwrite last member wins")
	if errs := col.errors(); len(errs) > 0 {
		t.Fatalf("transfer errors: %v", errs)
	}
}
