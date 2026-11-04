package tool

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ilaziness/vela/internal/usererr"
)

// ValidateAllowedDir returns the cleaned absolute path if path is an existing directory.
// Relative / drive-relative paths are rejected so allowlist entries do not depend on CWD.
func ValidateAllowedDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", usererr.Coded("ai.fs.path_required", "Directory path is required")
	}
	if !filepath.IsAbs(path) {
		return "", usererr.Coded("ai.fs.path_absolute", "Use an absolute path")
	}
	if runtime.GOOS == "windows" && filepath.VolumeName(path) == "" {
		return "", usererr.Coded("ai.fs.path_drive", "Use an absolute path with a drive letter")
	}
	abs := filepath.Clean(path)
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", usererr.Coded("ai.fs.dir_missing", "Directory does not exist: %s", abs)
		}
		return "", usererr.Wrap(err, "ai.fs.dir_inaccessible", "Cannot access directory: %s")
	}
	if !st.IsDir() {
		return "", usererr.Coded("ai.fs.not_directory", "Path is not a directory: %s", abs)
	}
	if link, err := filepath.EvalSymlinks(abs); err == nil {
		abs = link
	}
	return abs, nil
}

// SanitizeAllowedDirs keeps only existing absolute directories (validated).
func SanitizeAllowedDirs(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	seen := make(map[string]struct{}, len(dirs))
	for _, d := range dirs {
		abs, err := ValidateAllowedDir(d)
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
