package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NormalizeAllowedDirs trims, Abs-cleans, and dedupes allowlist entries.
func NormalizeAllowedDirs(dirs []string) []string {
	seen := make(map[string]struct{}, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		abs, err := filepath.Abs(filepath.Clean(d))
		if err != nil {
			continue
		}
		key := pathKey(abs)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, abs)
	}
	return out
}

func pathKey(p string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(p)
	}
	return p
}

// ResolveUnderAllowlist returns a path usable for IO that must fall under one allow root.
// Both the cleaned absolute path and its symlink-resolved form must stay inside an allow root.
// The returned path is the resolved form when the leaf exists; otherwise the cleaned abs path
// (with ancestors verified) for creating new files.
func ResolveUnderAllowlist(path string, allowed []string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	roots := NormalizeAllowedDirs(allowed)
	if len(roots) == 0 {
		return "", fmt.Errorf("no allowed directories configured")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	resolved, err := resolveExisting(abs)
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		rootResolved, err := resolveExisting(root)
		if err != nil {
			continue
		}
		// Lexical abs and fully resolved location must both stay under the root.
		if underPath(abs, root) && underPath(resolved, rootResolved) {
			if _, err := os.Lstat(abs); err == nil {
				return resolved, nil
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("path %q is outside allowed directories", abs)
}

// resolveExisting resolves symlinks for the deepest existing ancestor, then
// re-joins any missing trailing segments (needed for create/write of new files).
func resolveExisting(path string) (string, error) {
	path = filepath.Clean(path)
	cur := path
	var missing []string
	for {
		fi, err := os.Lstat(cur)
		if err == nil {
			resolved := cur
			if link, err := filepath.EvalSymlinks(cur); err == nil {
				resolved = link
			} else if fi.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("broken symlink: %s", cur)
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("cannot resolve path: %s", path)
		}
		missing = append(missing, filepath.Base(cur))
		cur = parent
	}
}

func underPath(target, root string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	sep := string(filepath.Separator)
	if rel == ".." || strings.HasPrefix(rel, ".."+sep) {
		return false
	}
	if runtime.GOOS == "windows" {
		t := strings.ToLower(filepath.Clean(target))
		r := strings.ToLower(filepath.Clean(root))
		if t == r {
			return true
		}
		if !strings.HasSuffix(r, sep) {
			r += sep
		}
		return strings.HasPrefix(t, r)
	}
	return true
}
