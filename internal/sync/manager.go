package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/sync/store"
)

// BackupInfo is re-exported for RPC bindings.
type BackupInfo = store.BackupInfo

// SyncManager orchestrates pack/encrypt and transport.
type SyncManager struct {
	store store.Store
}

// NewSyncManager creates a manager with an already-opened store.
func NewSyncManager(st store.Store) *SyncManager {
	return &SyncManager{store: st}
}

// Upload packs, encrypts with password, wraps payload+manifest in a zip, then uploads.
func (sm *SyncManager) Upload(srcDir, password string) error {
	if sm.store == nil {
		return fmt.Errorf("sync not configured")
	}
	if password == "" {
		return fmt.Errorf("password required")
	}

	tmpDir, err := newSyncTempDir()
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	payloadPath := filepath.Join(tmpDir, payloadEntryName)
	payloadFile, err := os.OpenFile(payloadPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create payload: %w", err)
	}
	if err := PackStream(srcDir, payloadFile, password); err != nil {
		payloadFile.Close()
		return fmt.Errorf("failed to pack: %w", err)
	}
	if err := payloadFile.Close(); err != nil {
		return fmt.Errorf("failed to close payload: %w", err)
	}

	sum, size, err := hashFileSHA256(payloadPath)
	if err != nil {
		return fmt.Errorf("failed to hash payload: %w", err)
	}
	manifestPath := filepath.Join(tmpDir, manifestEntryName)
	if err := writeManifest(manifestPath, sum, size); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	zipName, err := backupZipFileName()
	if err != nil {
		return fmt.Errorf("failed to name backup zip: %w", err)
	}
	zipPath := filepath.Join(tmpDir, zipName)
	if err := createStoreZip(zipPath, payloadPath, manifestPath); err != nil {
		return fmt.Errorf("failed to create backup zip: %w", err)
	}

	zipInfo, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("failed to stat backup zip: %w", err)
	}
	zipFile, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open backup zip: %w", err)
	}
	defer zipFile.Close()

	if _, err := sm.store.Upload(context.Background(), zipName, zipFile, zipInfo.Size()); err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if shouldSkipSyncPath(relPath) {
			return nil
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func restoreBackup(backupDir, dstDir string) error {
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return nil
	}
	if err := os.RemoveAll(dstDir); err != nil {
		_ = os.Rename(backupDir, dstDir)
		return fmt.Errorf("failed to remove dst dir before restore: %w", err)
	}
	return os.Rename(backupDir, dstDir)
}

// Download downloads a backup zip, verifies the payload hash, then decrypts over local data.
func (sm *SyncManager) Download(dstDir, name, password string, db *database.Database) error {
	if sm.store == nil {
		return fmt.Errorf("sync not configured")
	}
	if password == "" {
		return fmt.Errorf("password required")
	}

	reader, err := sm.store.Download(context.Background(), name)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer reader.Close()

	tmpDir, err := newSyncTempDir()
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "download.zip")
	zipFile, err := os.OpenFile(zipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create download file: %w", err)
	}
	if _, err := io.Copy(zipFile, reader); err != nil {
		zipFile.Close()
		return fmt.Errorf("failed to save download: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		return fmt.Errorf("failed to close download: %w", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	payloadPath, manifestPath, err := extractBackupZip(zipPath, extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract backup zip: %w", err)
	}
	if err := verifyPayload(payloadPath, manifestPath); err != nil {
		return fmt.Errorf("backup integrity check failed: %w", err)
	}

	unpackDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(unpackDir, 0o700); err != nil {
		return fmt.Errorf("failed to create unpack dir: %w", err)
	}
	payloadFile, err := os.Open(payloadPath)
	if err != nil {
		return fmt.Errorf("failed to open payload: %w", err)
	}
	err = UnpackStream(payloadFile, unpackDir, password)
	payloadFile.Close()
	if err != nil {
		return fmt.Errorf("failed to unpack: %w", err)
	}

	entries, err := os.ReadDir(unpackDir)
	if err != nil {
		return fmt.Errorf("failed to read unpack dir: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("unpack dir is empty, unpack may have failed")
	}

	if db != nil {
		db.Close()
	}

	backupDir := dstDir + ".backup"
	if _, err := os.Stat(dstDir); err == nil {
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("failed to remove old backup: %w", err)
		}
		if err := os.Rename(dstDir, backupDir); err != nil {
			return fmt.Errorf("failed to backup: %w", err)
		}
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		if err := restoreBackup(backupDir, dstDir); err != nil {
			return fmt.Errorf("failed to create dst dir and restore backup: %w", err)
		}
		return fmt.Errorf("failed to create dst dir: %w", err)
	}

	if err := copyDir(unpackDir, dstDir); err != nil {
		os.RemoveAll(dstDir)
		if err := restoreBackup(backupDir, dstDir); err != nil {
			return fmt.Errorf("failed to copy data and restore backup: %w", err)
		}
		return fmt.Errorf("failed to copy data: %w", err)
	}

	os.RemoveAll(backupDir)
	return nil
}

// ListBackups returns stored backup zips (limit=0 means no limit).
func (sm *SyncManager) ListBackups(limit, offset int) ([]BackupInfo, error) {
	if sm.store == nil {
		return nil, fmt.Errorf("sync not configured")
	}
	return sm.store.ListBackups(context.Background(), limit, offset)
}

// DeleteBackup deletes a backup zip by name.
func (sm *SyncManager) DeleteBackup(name string) error {
	if sm.store == nil {
		return fmt.Errorf("sync not configured")
	}
	return sm.store.DeleteBackup(context.Background(), name)
}

// HealthCheck probes the active store.
func (sm *SyncManager) HealthCheck() error {
	if sm.store == nil {
		return fmt.Errorf("sync not configured")
	}
	return sm.store.HealthCheck(context.Background())
}
