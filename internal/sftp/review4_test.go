//go:build integration

package sftp

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"

	"github.com/ilaziness/vela/internal/usererr"
)

// waitRemoteLink waits for a symlink entry at p with the exact target text.
func waitRemoteLink(t *testing.T, client *sftp.Client, p, want, desc string) {
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

// TestIntegrationPasteSymlinkEntryIdentity: a paste entry is identified by its
// canonical parent directory plus its verbatim basename — the final symlink is
// never followed.
//
//  1. cutting link -> /other/parent over its own containing directory must be
//     rejected before the copy-and-delete path runs (resolving the link hides
//     the ancestry: /work/parent/parent compares as /other/parent);
//  2. without an explicit destination name a symlink keeps its own basename at
//     the target (current stays current, not the target's v1);
//  3. the destination keeps the same semantics: a destination NAME that is a
//     symlink is one directory slot, not its resolved target, so moving a tree
//     onto that slot is not a paste into itself.
func TestIntegrationPasteSymlinkEntryIdentity(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "paste-identity")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/other/parent "+work+"/parent "+work+"/dir/releases "+work+"/dst2 "+work+"/m/srcdir/sub")
	writeRemote(t, client, path.Join(work, "other", "parent", "keep.txt"), "KEEP\n")
	writeRemote(t, client, path.Join(work, "parent", "x.txt"), "X\n")
	writeRemote(t, client, path.Join(work, "dir", "releases", "v1"), "V\n")
	writeRemote(t, client, path.Join(work, "m", "srcdir", "f.txt"), "M\n")
	remoteRun(t, mgr, "ln -s -- ../other/parent "+work+"/parent/parent")
	remoteRun(t, mgr, "ln -s -- releases/v1 "+work+"/dir/current")
	remoteRun(t, mgr, "ln -s -- srcdir/sub "+work+"/m/slink")

	// 1: rejected cut of link over its containing directory.
	err := mgr.PasteRemotePaths("test", []string{path.Join(work, "parent", "parent")}, nil, work, true, true)
	if usererr.CodeOf(err) != "sftp.paste_into_self" {
		t.Fatalf("cut link over its ancestor dir = %v, want sftp.paste_into_self", err)
	}
	if got := string(readRemote(t, client, path.Join(work, "parent", "x.txt"))); got != "X\n" {
		t.Fatalf("containing dir disturbed by rejected cut: x.txt = %q", got)
	}
	if got, lerr := client.ReadLink(path.Join(work, "parent", "parent")); lerr != nil || got != "../other/parent" {
		t.Fatalf("source link after rejected cut = %q, %v", got, lerr)
	}
	if got := string(readRemote(t, client, path.Join(work, "other", "parent", "keep.txt"))); got != "KEEP\n" {
		t.Fatalf("link target tree disturbed by rejected cut: keep.txt = %q", got)
	}

	// 2: default destination name keeps the symlink's own basename.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "dir", "current")}, nil, path.Join(work, "dst2"), false, true); err != nil {
		t.Fatalf("paste link copy: %v", err)
	}
	waitRemoteLink(t, client, path.Join(work, "dst2", "current"), "releases/v1", "verbatim link basename")
	if _, err := client.Lstat(path.Join(work, "dst2", "v1")); err == nil {
		t.Fatalf("link was pasted under its target basename v1")
	}

	// 3: a destination name that is a symlink is one slot to replace.
	if err := mgr.PasteRemotePaths("test", []string{path.Join(work, "m", "srcdir")}, []string{"slink"}, path.Join(work, "m"), true, true); err != nil {
		t.Fatalf("cut onto a destination symlink slot: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "m", "slink", "f.txt"), "M\n", "tree moved onto dst-link slot")
	waitRemote(t, client, path.Join(work, "m", "srcdir"), false, "source tree moved")
}

// TestIntegrationPasteMergeNeighborTree: a directory merge reads the tree the
// server resolves. /srv/link/../tree is /data/tree (POSIX resolves ".." after
// following the link), so its a.txt is DATA-TREE — never the same-named
// neighbor /srv/tree. Joining child names onto a lexical spelling used to open
// the neighbor instead.
func TestIntegrationPasteMergeNeighborTree(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "paste-neighbor")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/data/tree/n "+work+"/data/sub "+work+"/srv/tree "+work+"/dst/tree")
	writeRemote(t, client, path.Join(work, "data", "tree", "a.txt"), "DATA-TREE\n")
	writeRemote(t, client, path.Join(work, "data", "tree", "n", "x.txt"), "DATA-X\n")
	writeRemote(t, client, path.Join(work, "srv", "tree", "a.txt"), "SRV-TREE\n")
	writeRemote(t, client, path.Join(work, "dst", "tree", "keep.txt"), "KEEP\n")
	remoteRun(t, mgr, "ln -s -- ../data/sub "+work+"/srv/link")

	// Literal spelling — path.Join would collapse "link/.." lexically.
	src := work + "/srv/link/../tree"
	if err := mgr.PasteRemotePaths("test", []string{src}, []string{"tree"}, path.Join(work, "dst"), false, true); err != nil {
		t.Fatalf("merge-copy link/../tree: %v", err)
	}
	waitRemoteContent(t, client, path.Join(work, "dst", "tree", "a.txt"), "DATA-TREE\n", "merge copies the server-resolved tree")
	waitRemoteContent(t, client, path.Join(work, "dst", "tree", "n", "x.txt"), "DATA-X\n", "nested content of the resolved tree")
	if got := string(readRemote(t, client, path.Join(work, "dst", "tree", "keep.txt"))); got != "KEEP\n" {
		t.Fatalf("merge disturbed pre-existing content: keep.txt = %q", got)
	}
}

// TestIntegrationExtractNoOverwriteInsideDestDir: a non-root extraction into a
// writable destination whose PARENT is not writable must succeed. The staging
// directory lives inside the destination — write access to the destination's
// parent must never be required (e.g. /home/alice with root-owned /home).
func TestIntegrationExtractNoOverwriteInsideDestDir(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	work := "/tmp/vela-extract-np"
	_ = client.RemoveAll(work)
	t.Cleanup(func() {
		_ = client.Chmod(work, 0o755)
		_ = client.RemoveAll(work)
	})
	remoteRun(t, mgr, "mkdir -p "+work+"/dest")
	arc := path.Join(work, "a.tar.gz")
	uploadBytes(t, client, arc, buildTarGzEntries(t, [][2]string{{"root/a.txt", "R\n"}}))
	remoteRun(t, mgr, "chown -R nobody "+work+"/dest && chmod 0644 "+arc+" && chmod 0555 "+work)

	cmd := extractTarNoOverwriteCommand(formatTarGz, arc, path.Join(work, "dest"), 1)
	remoteRun(t, mgr, "su -s /bin/sh -c "+shellQuote(cmd)+" nobody")

	if got := string(readRemote(t, client, path.Join(work, "dest", "a.txt"))); got != "R\n" {
		t.Fatalf("extracted content = %q, want %q", got, "R\n")
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "ls -A "+work+"/dest")); got != "a.txt" {
		t.Fatalf("dest entries = %q, want only a.txt (staging cleaned up inside dest)", got)
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "ls -A "+work)); got != "a.tar.gz\ndest" {
		t.Fatalf("work entries = %q, want no staging in the destination's parent", got)
	}
}

// TestIntegrationExtractThousandsOfEntries: the no-overwrite promotion walks
// the staging tree instead of inlining entry names, so an archive with several
// thousand members extracts fine (the whole command is one shell argument with
// a ~128KiB per-argument cap on Linux).
func TestIntegrationExtractThousandsOfEntries(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "extract-many")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	dest := path.Join(work, "dest")
	remoteRun(t, mgr, "mkdir -p "+dest)

	const n = 3000
	members := make([][2]string, n)
	for i := range members {
		members[i] = [2]string{fmt.Sprintf("root/f%05d.txt", i), fmt.Sprintf("C%05d\n", i)}
	}
	arc := path.Join(work, "many.tar.gz")
	uploadBytes(t, client, arc, buildTarGzEntries(t, members))

	if err := mgr.extractTarRemote(context.Background(), "test", formatTarGz, arc, dest, true, false); err != nil {
		t.Fatalf("extract %d entries: %v", n, err)
	}
	if got := strings.TrimSpace(remoteRun(t, mgr, "find "+dest+" -type f | wc -l")); got != "3000" {
		t.Fatalf("extracted file count = %q, want 3000", got)
	}
	if got := string(readRemote(t, client, path.Join(dest, "f00000.txt"))); got != "C00000\n" {
		t.Fatalf("first file = %q", got)
	}
	if got := string(readRemote(t, client, path.Join(dest, "f02999.txt"))); got != "C02999\n" {
		t.Fatalf("last file = %q", got)
	}
}

// TestIntegrationExtractConcurrentNoClobber: promotion installs entries with
// link(2)/symlink(2)/mkdir(2) semantics — creation fails on an existing name —
// so a destination entry created between the check and the promotion is a
// reported conflict, never a silent replacement. Two concurrent no-overwrite
// extractions of the same name: exactly one installs and one reports. Parent
// directories created concurrently are tolerated (a merge, not a replacement).
func TestIntegrationExtractConcurrentNoClobber(t *testing.T) {
	mgr, client, _ := newTestManager(t)
	root := "/root/sftp-test"
	work := path.Join(root, "extract-race")
	_ = client.RemoveAll(work)
	t.Cleanup(func() { _ = client.RemoveAll(work) })
	remoteRun(t, mgr, "mkdir -p "+work+"/dest1 "+work+"/dest2")

	arc1 := path.Join(work, "v1.tar.gz")
	arc2 := path.Join(work, "v2.tar.gz")
	uploadBytes(t, client, arc1, buildTarGzEntries(t, [][2]string{{"root/a.txt", "V1\n"}}))
	uploadBytes(t, client, arc2, buildTarGzEntries(t, [][2]string{{"root/a.txt", "V2\n"}}))

	dest := path.Join(work, "dest1")
	errs := make([]error, 2)
	arcs := []string{arc1, arc2}
	done := make(chan int, 2)
	for i := range arcs {
		go func(i int) {
			errs[i] = mgr.extractTarRemote(context.Background(), "test", formatTarGz, arcs[i], dest, true, false)
			done <- i
		}(i)
	}
	<-done
	<-done
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("concurrent same-name extraction errors = %v, %v, want exactly one success", errs[0], errs[1])
	}
	want := "V2\n"
	if errs[0] == nil {
		want = "V1\n"
	}
	if got := string(readRemote(t, client, path.Join(dest, "a.txt"))); got != want {
		t.Fatalf("installed content = %q, want the winner's %q", got, want)
	}

	p1 := path.Join(work, "p1.tar.gz")
	p2 := path.Join(work, "p2.tar.gz")
	uploadBytes(t, client, p1, buildTarGzEntries(t, [][2]string{{"root/src/x.txt", "X1\n"}}))
	uploadBytes(t, client, p2, buildTarGzEntries(t, [][2]string{{"root/src/y.txt", "Y1\n"}}))
	dest2 := path.Join(work, "dest2")
	errs2 := make([]error, 2)
	arcs2 := []string{p1, p2}
	done2 := make(chan int, 2)
	for i := range arcs2 {
		go func(i int) {
			errs2[i] = mgr.extractTarRemote(context.Background(), "test", formatTarGz, arcs2[i], dest2, true, false)
			done2 <- i
		}(i)
	}
	<-done2
	<-done2
	for i, err := range errs2 {
		if err != nil {
			t.Fatalf("sibling extraction %d failed on a concurrently created parent: %v", i, err)
		}
	}
	if got := string(readRemote(t, client, path.Join(dest2, "src", "x.txt"))); got != "X1\n" {
		t.Fatalf("src/x.txt = %q", got)
	}
	if got := string(readRemote(t, client, path.Join(dest2, "src", "y.txt"))); got != "Y1\n" {
		t.Fatalf("src/y.txt = %q", got)
	}
}
