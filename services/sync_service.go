package services

import (
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	internalsync "github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/internal/usererr"
)

var ErrSyncNotConfigured = usererr.Coded("sync.not_configured", "Sync is not configured")

type SyncProgress = internalsync.Progress

type SyncService struct {
	configService *ConfigService
	db            *database.Database
	logger        *zap.Logger
	progress      *internalsync.ProgressReporter
}

func NewSyncService(configService *ConfigService, db *database.Database, logger *zap.Logger) *SyncService {
	return &SyncService{
		configService: configService,
		db:            db,
		logger:        logger,
		progress:      internalsync.NewProgressReporter(nil),
	}
}

func (s *SyncService) GetSyncConfig() (*internalsync.SyncConfig, error) {
	return s.configService.GetSyncConfig(), nil
}

func (s *SyncService) SaveSyncConfig(config internalsync.SyncConfig) error {
	return s.configService.SaveSyncConfig(config)
}

func (s *SyncService) GetSyncProgress() SyncProgress {
	return s.progress.GetProgress()
}

func (s *SyncService) openManager() (*internalsync.SyncManager, error) {
	cfg := s.configService.GetSyncConfig()
	if !cfg.IsConfigured() {
		return nil, ErrSyncNotConfigured
	}
	plain, err := s.configService.decryptSyncConfig(*cfg)
	if err != nil {
		return nil, err
	}
	st, err := internalsync.OpenStore(&plain)
	if err != nil {
		return nil, err
	}
	return internalsync.NewSyncManager(st), nil
}

func (s *SyncService) UploadSync() error {
	manager, err := s.openManager()
	if err != nil {
		return err
	}
	password, err := s.configService.unlockedPassword()
	if err != nil {
		return err
	}
	s.progress.Reset()
	s.progress.UpdateProgress("preparing", 0, 0)
	err = s.uploadWithRetry(manager, s.configService.userDataDir(), password, 3)
	if err != nil {
		s.progress.SetError(err.Error())
		return err
	}
	s.progress.SetCompleted()
	return nil
}

func (s *SyncService) uploadWithRetry(manager *internalsync.SyncManager, userDataDir, password string, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			s.progress.SetStage(fmt.Sprintf("retrying (%d/%d)", i, maxRetries))
			time.Sleep(time.Second * time.Duration(i))
		}
		s.progress.SetStage("packing")
		err := manager.Upload(userDataDir, password)
		if err == nil {
			return nil
		}
		lastErr = err
		s.logger.Error("Upload failed, will retry", zap.Int("attempt", i+1), zap.Error(err))
	}
	return usererr.Coded("sync.upload_failed", "Upload failed after %d attempts: %s", strconv.Itoa(maxRetries), lastErr.Error())
}

func (s *SyncService) DownloadSync(name string) error {
	manager, err := s.openManager()
	if err != nil {
		return err
	}
	password, err := s.configService.unlockedPassword()
	if err != nil {
		return err
	}
	s.progress.Reset()
	s.progress.UpdateProgress("preparing", 0, 0)
	err = s.downloadWithRetry(manager, s.configService.userDataDir(), name, password, 3)
	if err != nil {
		s.progress.SetError(err.Error())
		return usererr.Coded("sync.restore_failed", "Data restore failed; please restart the app manually")
	}
	s.progress.SetCompleted()
	return nil
}

func (s *SyncService) downloadWithRetry(manager *internalsync.SyncManager, userDataDir, name, password string, maxRetries int) error {
	var lastErr error
	attempts := 0
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			s.progress.SetStage(fmt.Sprintf("retrying (%d/%d)", i, maxRetries))
			time.Sleep(time.Second * time.Duration(i))
		}
		s.progress.SetStage("downloading")
		attempts = i + 1
		err := manager.Download(userDataDir, name, password, s.db)
		if err == nil {
			return nil
		}
		lastErr = err
		s.logger.Error("Download failed", zap.Int("attempt", attempts), zap.Error(err))
		if !s.db.IsOpen() {
			break
		}
	}
	return usererr.Coded("sync.download_failed", "Download failed after %d attempts: %s", strconv.Itoa(attempts), lastErr.Error())
}

func (s *SyncService) ListBackups(limit, offset int) ([]internalsync.BackupInfo, error) {
	manager, err := s.openManager()
	if err != nil {
		return nil, err
	}
	return manager.ListBackups(limit, offset)
}

func (s *SyncService) DeleteBackup(name string) error {
	manager, err := s.openManager()
	if err != nil {
		return err
	}
	return manager.DeleteBackup(name)
}

func (s *SyncService) HealthCheck() error {
	manager, err := s.openManager()
	if err != nil {
		return err
	}
	return manager.HealthCheck()
}
