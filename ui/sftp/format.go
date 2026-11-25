package sftpui

import (
	"time"

	"github.com/ilaziness/vela/internal/utils"
)

// formatSize renders a byte count via the shared human formatter.
func formatSize(n int64) string {
	if n < 0 {
		return ""
	}
	return utils.FormatBytes(n)
}

// formatModTime renders an RFC3339 timestamp for the modified column.
func formatModTime(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04")
}
