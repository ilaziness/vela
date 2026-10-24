package tool

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	defaultReadMaxBytes   = 512 * 1024
	defaultWriteMaxBytes  = 5 * 1024 * 1024
	defaultListMaxEntries = 200
)

// ReadFile reads a UTF-8 (or raw) file under the allowlist.
func ReadFile(path string, allowed []string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultReadMaxBytes
	}
	abs, err := ResolveUnderAllowlist(path, allowed)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory; use list_local_directory")
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		for !utf8.Valid(data) && len(data) > 0 {
			data = data[:len(data)-1]
		}
		truncated = true
	}
	body := string(data)
	if !utf8.ValidString(body) {
		body = strings.ToValidUTF8(body, "�")
	}
	if truncated {
		body += "\n…[truncated]"
	}
	return body, nil
}

// WriteFile writes content to a file under the allowlist (creates parent dirs).
func WriteFile(path, content string, allowed []string) (string, error) {
	if len(content) > defaultWriteMaxBytes {
		return "", fmt.Errorf("content too large: %d bytes (max %d)", len(content), defaultWriteMaxBytes)
	}
	abs, err := ResolveUnderAllowlist(path, allowed)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("create parent dir: %w", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), abs), nil
}

// DirEntry is one list_local_directory row.
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

// ListDirResult is the list_local_directory payload.
type ListDirResult struct {
	Entries   []DirEntry `json:"entries"`
	Truncated bool       `json:"truncated,omitempty"`
	Omitted   int        `json:"omitted,omitempty"`
}

// ListDir lists entries under an allowed directory.
func ListDir(path string, allowed []string, maxEntries int) (ListDirResult, error) {
	if maxEntries <= 0 {
		maxEntries = defaultListMaxEntries
	}
	abs, err := ResolveUnderAllowlist(path, allowed)
	if err != nil {
		return ListDirResult{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return ListDirResult{}, err
	}
	if !info.IsDir() {
		return ListDirResult{}, fmt.Errorf("path is not a directory")
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return ListDirResult{}, err
	}
	out := make([]DirEntry, 0, min(len(entries), maxEntries))
	for i, e := range entries {
		if i >= maxEntries {
			break
		}
		full := filepath.Join(abs, e.Name())
		row := DirEntry{Name: e.Name(), Path: full, IsDir: e.IsDir()}
		if fi, err := e.Info(); err == nil && !e.IsDir() {
			row.Size = fi.Size()
		}
		out = append(out, row)
	}
	res := ListDirResult{Entries: out}
	if len(entries) > maxEntries {
		res.Truncated = true
		res.Omitted = len(entries) - maxEntries
	}
	return res, nil
}
