package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ValidateAllowedDir returns the cleaned absolute path if path is an existing directory.
// Relative / drive-relative paths are rejected so allowlist entries do not depend on CWD.
func ValidateAllowedDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("目录路径不能为空")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("请使用绝对路径")
	}
	if runtime.GOOS == "windows" && filepath.VolumeName(path) == "" {
		return "", fmt.Errorf("请使用带盘符的绝对路径")
	}
	abs := filepath.Clean(path)
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("目录不存在: %s", abs)
		}
		return "", fmt.Errorf("无法访问目录: %w", err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("路径不是目录: %s", abs)
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
