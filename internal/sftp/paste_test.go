package sftp

import (
	"os"
	"testing"
)

func TestPasteDestInvalid(t *testing.T) {
	cases := []struct {
		name         string
		src          string
		destDir      string
		destName     string
		srcIsRealDir bool
		invalid      bool
	}{
		{"self", "/a/b", "/a", "b", true, true},
		// Equivalent spellings of the same path are still a self-copy: the
		// identity check runs on server-canonical paths, the lexical Clean in
		// the helper only tolerates spelling noise.
		{"self via dot segment", "/a/./b", "/a", "b", true, true},
		{"self via double slash", "/a//b", "/a/", "b", true, true},
		{"self via trailing slash", "/a/b/", "/a", "b", true, true},
		{"self via redundant dots", "/a/x/../b", "/a", "b", true, true},
		{"into own subdir", "/a/b", "/a/b", "sub", true, true},
		{"parent onto descendant", "/a/b/sub", "/a", "b", true, true},
		{"renamed duplicate", "/a/b", "/a", "b (1)", true, false},
		{"unrelated", "/a/b", "/c", "d", true, false},
		{"name prefix only", "/a/bc", "/a", "b", true, false},
		{"dir prefix only", "/a/bc/d", "/a/b", "d", true, false},
		// A symlink source is a leaf entry: recreating it under its target's
		// tree moves/copies the link itself, never its resolved contents.
		{"symlink leaf into target subtree", "/a/link", "/a/link", "sub", false, false},
		{"symlink leaf renamed under target", "/srv/link", "/data", "x", false, false},
		// Equality and ancestry are still rejected for a symlink source.
		{"symlink self", "/a/link", "/a", "link", false, true},
		// Copying a child onto its own parent destroys the source inside the
		// replaced tree — even when the child is a symlink leaf.
		{"symlink parent onto descendant", "/a/link/sub", "/a", "link", false, true},
		// A sibling destination is unrelated to the source entry.
		{"symlink sibling", "/a/link/sub", "/a/link", "x", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dst := joinRemotePath(c.destDir, c.destName)
			if invalid := pasteDestInvalid(c.src, dst, c.srcIsRealDir); invalid != c.invalid {
				t.Fatalf("pasteDestInvalid(%q, %q, %v) = %v, want invalid=%v",
					c.src, dst, c.srcIsRealDir, invalid, c.invalid)
			}
		})
	}
}

func TestDirPopulateMode(t *testing.T) {
	cases := []struct{ perm, want os.FileMode }{
		// A private mode stays private while the directory is populated.
		{0o700, 0o700},
		{0o000, 0o700},
		// Owner access is kept for read-only sources (children must be
		// created); group/other never see more than the source allows.
		{0o555, 0o755},
		{0o755, 0o755},
		{0o750, 0o750},
		{0o777, 0o777},
	}
	for _, c := range cases {
		if got := dirPopulateMode(c.perm); got != c.want {
			t.Fatalf("dirPopulateMode(%o) = %o, want %o", c.perm, got, c.want)
		}
	}
}
