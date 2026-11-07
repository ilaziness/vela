package store

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
)

// SanitizeName returns a zip basename safe to use as an object key.
func SanitizeName(name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	name = path.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid backup name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return "", fmt.Errorf("invalid backup name")
	}
	for _, r := range name {
		if r < 32 || r == '/' || unicode.IsControl(r) {
			return "", fmt.Errorf("invalid backup name")
		}
	}
	return name, nil
}

// NameFromKey returns the backup zip name for an object stored at prefix+name.
// Nested keys are skipped so list entries always match Download(name).
func NameFromKey(key, prefix string) (string, error) {
	name := strings.TrimPrefix(key, prefix)
	if strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid backup name")
	}
	return SanitizeName(name)
}

// NormalizePrefix ensures a non-empty prefix ends with "/".
func NormalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

// NormalizeRemotePath returns a remote directory path with a leading slash and no trailing slash.
func NormalizeRemotePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

// SortBackupsDesc sorts by CreatedAt then Name, newest first.
func SortBackupsDesc(backups []BackupInfo) {
	sort.Slice(backups, func(i, j int) bool {
		if !backups[i].CreatedAt.Equal(backups[j].CreatedAt) {
			return backups[i].CreatedAt.After(backups[j].CreatedAt)
		}
		return backups[i].Name > backups[j].Name
	})
}

// PaginateBackups returns a limit/offset slice. limit<=0 means no limit.
func PaginateBackups(backups []BackupInfo, limit, offset int) []BackupInfo {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(backups) {
		return nil
	}
	backups = backups[offset:]
	if limit > 0 && len(backups) > limit {
		backups = backups[:limit]
	}
	return backups
}
