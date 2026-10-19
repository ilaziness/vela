package localfs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestListHiddenAndOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "afile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".secret"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	hidden, err := List(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(hidden.Entries))
	}
	if hidden.Entries[0].Name != "bdir" || !hidden.Entries[0].IsDir {
		t.Fatalf("first entry = %+v, want directory bdir", hidden.Entries[0])
	}
	if hidden.Entries[1].Name != "afile" || hidden.Entries[1].IsDir {
		t.Fatalf("second entry = %+v, want file afile", hidden.Entries[1])
	}
	if hidden.AtRoot {
		t.Fatal("temp dir reported as root")
	}

	shown, err := List(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	foundSecret := false
	for _, entry := range shown.Entries {
		if entry.Name == ".secret" {
			foundSecret = true
		}
	}
	if !foundSecret {
		t.Fatal("hidden file missing when showHidden is true")
	}
}

func TestListAtRoot(t *testing.T) {
	root := string(filepath.Separator)
	if vol := filepath.VolumeName(t.TempDir()); vol != "" {
		root = vol + string(filepath.Separator)
	}
	listing, err := List(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !listing.AtRoot {
		t.Fatalf("AtRoot false for %s (parent %s)", listing.Dir, listing.Parent)
	}
	if listing.Parent != listing.Dir {
		t.Fatalf("parent %s != dir %s", listing.Parent, listing.Dir)
	}
}

func TestListSymlinkDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linkdir")
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlink requires privilege:", err)
		}
		t.Fatal(err)
	}

	listing, err := List(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	var found *Entry
	for i := range listing.Entries {
		if listing.Entries[i].Name == "linkdir" {
			found = &listing.Entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("symlink directory missing")
	}
	if !found.IsDir {
		t.Fatal("symlink directory not reported as directory")
	}
	if found.Path != link {
		t.Fatalf("path = %s, want %s", found.Path, link)
	}
}

func TestListBrokenSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "broken")
	if err := os.Symlink(filepath.Join(dir, "missing"), link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlink requires privilege:", err)
		}
		t.Fatal(err)
	}
	listing, err := List(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(listing.Entries))
	}
	if listing.Entries[0].IsDir {
		t.Fatal("broken symlink reported as directory")
	}
}

func TestListRejectsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := List(file, false); err == nil {
		t.Fatal("expected error for file path")
	}
}
