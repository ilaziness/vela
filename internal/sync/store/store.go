package store

import (
	"context"
	"io"
	"time"
)

const storeHTTPTimeout = 5 * time.Minute

// BackupInfo describes one stored backup zip.
type BackupInfo struct {
	Name      string    `json:"name"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is a backup transport backend.
type Store interface {
	HealthCheck(ctx context.Context) error
	Upload(ctx context.Context, name string, r io.Reader, size int64) (*BackupInfo, error)
	Download(ctx context.Context, name string) (io.ReadCloser, error)
	ListBackups(ctx context.Context, limit, offset int) ([]BackupInfo, error)
	DeleteBackup(ctx context.Context, name string) error
}
