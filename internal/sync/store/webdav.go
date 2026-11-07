package store

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
)

// WebDAVOptions configures a WebDAV backup store.
type WebDAVOptions struct {
	URL        string
	Username   string
	Password   string
	RemotePath string
}

type webDAVStore struct {
	client *gowebdav.Client
	root   string
}

// NewWebDAV creates a Store backed by a WebDAV server.
func NewWebDAV(opts WebDAVOptions) (Store, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return nil, fmt.Errorf("webdav url required")
	}
	if strings.TrimSpace(opts.Username) == "" {
		return nil, fmt.Errorf("webdav username required")
	}
	if strings.TrimSpace(opts.Password) == "" {
		return nil, fmt.Errorf("webdav password required")
	}
	client := gowebdav.NewClient(opts.URL, opts.Username, opts.Password)
	client.SetTimeout(storeHTTPTimeout)
	return &webDAVStore{client: client, root: NormalizeRemotePath(opts.RemotePath)}, nil
}

func (s *webDAVStore) objectPath(name string) string {
	return path.Join(s.root, name)
}

func (s *webDAVStore) ensureRoot() error {
	if err := s.client.MkdirAll(s.root, 0755); err != nil {
		if _, statErr := s.client.Stat(s.root); statErr != nil {
			return fmt.Errorf("webdav ensure dir: %w", err)
		}
	}
	return nil
}

func (s *webDAVStore) listAll() ([]BackupInfo, error) {
	files, err := s.client.ReadDir(s.root)
	if err != nil {
		if gowebdav.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("webdav list: %w", err)
	}
	var backups []BackupInfo
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name, err := SanitizeName(f.Name())
		if err != nil {
			continue
		}
		backups = append(backups, BackupInfo{
			Name:      name,
			FileSize:  f.Size(),
			CreatedAt: f.ModTime(),
		})
	}
	SortBackupsDesc(backups)
	return backups, nil
}

func (s *webDAVStore) HealthCheck(_ context.Context) error {
	return s.ensureRoot()
}

func (s *webDAVStore) Upload(_ context.Context, name string, r io.Reader, size int64) (*BackupInfo, error) {
	name, err := SanitizeName(name)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRoot(); err != nil {
		return nil, err
	}
	if err := s.client.WriteStreamWithLength(s.objectPath(name), r, size, 0644); err != nil {
		return nil, fmt.Errorf("webdav upload: %w", err)
	}
	return &BackupInfo{
		Name:      name,
		FileSize:  size,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (s *webDAVStore) Download(_ context.Context, name string) (io.ReadCloser, error) {
	name, err := SanitizeName(name)
	if err != nil {
		return nil, err
	}
	rc, err := s.client.ReadStream(s.objectPath(name))
	if err != nil {
		return nil, fmt.Errorf("webdav download: %w", err)
	}
	return rc, nil
}

func (s *webDAVStore) ListBackups(_ context.Context, limit, offset int) ([]BackupInfo, error) {
	backups, err := s.listAll()
	if err != nil {
		return nil, err
	}
	return PaginateBackups(backups, limit, offset), nil
}

func (s *webDAVStore) DeleteBackup(_ context.Context, name string) error {
	name, err := SanitizeName(name)
	if err != nil {
		return err
	}
	if err := s.client.Remove(s.objectPath(name)); err != nil {
		if gowebdav.IsErrNotFound(err) {
			return nil
		}
		return fmt.Errorf("webdav delete: %w", err)
	}
	return nil
}
