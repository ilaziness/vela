package backup

import (
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/internal/usererr"
)

const backupPageSize = 50

func uploadSync(d *Deps) error {
	manager, err := openManager(d)
	if err != nil {
		return err
	}
	password, err := vaultPassword(d)
	if err != nil {
		return err
	}
	var lastErr error
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Second * time.Duration(i))
		}
		err := manager.Upload(d.Store.UserDataDir(), password)
		if err == nil {
			return nil
		}
		lastErr = err
		d.Logger.Error("Upload failed, will retry", zap.Int("attempt", i+1), zap.Error(err))
	}
	return usererr.Coded("sync.upload_failed", "Upload failed after %d attempts: %s", strconv.Itoa(maxRetries), lastErr.Error())
}

func downloadSync(d *Deps, name string) error {
	manager, err := openManager(d)
	if err != nil {
		return err
	}
	password, err := vaultPassword(d)
	if err != nil {
		return err
	}
	var lastErr error
	attempts := 0
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Second * time.Duration(i))
		}
		attempts = i + 1
		err := manager.Download(d.Store.UserDataDir(), name, password, d.DB)
		if err == nil {
			return nil
		}
		lastErr = err
		d.Logger.Error("Download failed", zap.Int("attempt", attempts), zap.Error(err))
		if !d.DB.IsOpen() {
			break
		}
	}
	if !d.DB.IsOpen() {
		return usererr.Coded("sync.restore_failed", "Data restore failed; please restart the app manually")
	}
	return usererr.Coded("sync.download_failed", "Download failed after %d attempts: %s", strconv.Itoa(attempts), lastErr.Error())
}

func listBackups(d *Deps, limit, offset int) ([]sync.BackupInfo, error) {
	manager, err := openManager(d)
	if err != nil {
		return nil, err
	}
	return manager.ListBackups(limit, offset)
}
