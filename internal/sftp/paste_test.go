package sftp

import "testing"

func TestPasteDestInvalid(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		destDir  string
		destName string
		invalid  bool
	}{
		{"self", "/a/b", "/a", "b", true},
		{"into own subdir", "/a/b", "/a/b", "sub", true},
		{"parent onto descendant", "/a/b/sub", "/a", "b", true},
		{"renamed duplicate", "/a/b", "/a", "b (1)", false},
		{"unrelated", "/a/b", "/c", "d", false},
		{"name prefix only", "/a/bc", "/a", "b", false},
		{"dir prefix only", "/a/bc/d", "/a/b", "d", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dst, invalid := pasteDestInvalid(c.src, c.destDir, c.destName)
			if invalid != c.invalid {
				t.Fatalf("pasteDestInvalid(%q, %q, %q) = (%q, %v), want invalid=%v",
					c.src, c.destDir, c.destName, dst, invalid, c.invalid)
			}
		})
	}
}
