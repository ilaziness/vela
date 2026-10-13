package services

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	internalsync "github.com/ilaziness/vela/internal/sync"
)

var ErrSyncNotConfigured = errors.New("sync not configured")

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

func (s *SyncService) UploadSync() error {
	config := s.configService.GetSyncConfig()
	if !config.IsConfigured() {
		return ErrSyncNotConfigured
	}
	s.progress.UpdateProgress("preparing", 0, 0)
	manager := internalsync.NewSyncManager(config)
	err := s.uploadWithRetry(manager, s.configService.userDataDir(), 3)
	if err != nil {
		s.progress.SetError(err.Error())
		return err
	}
	s.progress.SetCompleted()
	return nil
}

func (s *SyncService) uploadWithRetry(manager *internalsync.SyncManager, userDataDir string, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			s.progress.SetStage(fmt.Sprintf("retrying (%d/%d)", i, maxRetries))
			time.Sleep(time.Second * time.Duration(i))
		}
		s.progress.SetStage("packing")
		err := manager.Upload(userDataDir)
		if err == nil {
			return nil
		}
		lastErr = err
		s.logger.Error("Upload failed, will retry", zap.Int("attempt", i+1), zap.Error(err))
	}
	return fmt.Errorf("upload failed after %d attempts: %w", maxRetries, lastErr)
}

func (s *SyncService) DownloadSync(version int) error {
	config := s.configService.GetSyncConfig()
	if !config.IsConfigured() {
		return ErrSyncNotConfigured
	}
	s.progress.UpdateProgress("preparing", 0, 0)
	manager := internalsync.NewSyncManager(config)
	err := s.downloadWithRetry(manager, s.configService.userDataDir(), version, 3)
	if err != nil {
		s.progress.SetError(err.Error())
		return fmt.Errorf("%w: 数据恢复失败，请手动重启应用", err)
	}
	s.progress.SetCompleted()
	return nil
}

func (s *SyncService) downloadWithRetry(manager *internalsync.SyncManager, userDataDir string, version int, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			s.progress.SetStage(fmt.Sprintf("retrying (%d/%d)", i, maxRetries))
			time.Sleep(time.Second * time.Duration(i))
		}
		s.progress.SetStage("downloading")
		err := manager.Download(userDataDir, version, s.db)
		if err == nil {
			return nil
		}
		lastErr = err
		s.logger.Error("Download failed, will retry", zap.Int("attempt", i+1), zap.Error(err))
	}
	return fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
}

func (s *SyncService) ListSyncVersions(limit, offset int) ([]internalsync.VersionInfo, error) {
	config := s.configService.GetSyncConfig()
	if !config.IsConfigured() {
		return nil, ErrSyncNotConfigured
	}
	return internalsync.NewSyncManager(config).ListVersions(limit, offset)
}

func (s *SyncService) DeleteSyncVersion(version int) error {
	config := s.configService.GetSyncConfig()
	if !config.IsConfigured() {
		return ErrSyncNotConfigured
	}
	return internalsync.NewSyncManager(config).DeleteVersion(version)
}

func (s *SyncService) HealthCheck() error {
	config := s.configService.GetSyncConfig()
	if !config.IsConfigured() {
		return ErrSyncNotConfigured
	}
	return internalsync.NewSyncManager(config).HealthCheck()
}

func (s *SyncService) CancelSync() {
	s.progress.SetError("cancelled by user")
}

func (s *SyncService) StartSyncContext()  {}
func (s *SyncService) CancelSyncContext() {}
