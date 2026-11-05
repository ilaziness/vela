package services

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/transfer"
)

const (
	EventProgress        = "eventProgress"
	EventTransferQueued  = "eventTransferQueued"
	TransferTypeUpload   = transfer.TypeUpload
	TransferTypeDownload = transfer.TypeDownload
)

func init() {
	application.RegisterEvent[ProgressData](EventProgress)
	application.RegisterEvent[TransferQueuedData](EventTransferQueued)
}

type ProgressData = transfer.ProgressData
type FileInfo = sftp.FileInfo

// TransferQueuedData notifies the UI that transfers were enqueued asynchronously.
type TransferQueuedData struct {
	SessionID string `json:"sessionID"`
	Count     int    `json:"count"`
}

type SftpService struct {
	app    *application.App
	mgr    *sftp.Manager
	store  *config.Store
	logger *zap.Logger
	edits  *editBook
}

func NewSftpService(app *application.App, mgr *sftp.Manager, store *config.Store, logger *zap.Logger) *SftpService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SftpService{app: app, mgr: mgr, store: store, logger: logger, edits: newEditBook()}
}

func (s *SftpService) ListFiles(sessionID, path string, showHidden bool) ([]FileInfo, error) {
	return s.mgr.ListFiles(sessionID, path, showHidden)
}

// ListLocal reads a local directory. An empty dir uses the remembered path when
// set, otherwise the user home directory. Successful listings update the shared
// remembered directory.
func (s *SftpService) ListLocal(dir string, showHidden bool) (localfs.Listing, error) {
	if dir == "" {
		dir = s.store.LastSftpLocalDir()
	}
	listing, err := localfs.List(dir, showHidden)
	if err != nil {
		return listing, err
	}
	if saveErr := s.store.SetLastSftpLocalDir(listing.Dir); saveErr != nil {
		s.logger.Debug("remember sftp local dir failed", zap.Error(saveErr))
	}
	return listing, nil
}

// ListUploadConflicts returns destination basenames under remotePath that already exist.
func (s *SftpService) ListUploadConflicts(sessionID, remotePath string, localPaths []string) ([]string, error) {
	return s.mgr.UploadConflicts(sessionID, remotePath, localPaths)
}

// ListDownloadConflicts returns destination basenames under localDir that already exist.
func (s *SftpService) ListDownloadConflicts(sessionID, localDir string, remotePaths []string) ([]string, error) {
	return s.mgr.DownloadConflicts(sessionID, localDir, remotePaths)
}

func (s *SftpService) UploadFileDialog(sessionID, remotePath string) error {
	localPath, err := promptOpenFile(s.app, s.store, i18n.T("dialog.select_file"))
	if dialogCancelled(err) || localPath == "" {
		return nil
	}
	if err != nil {
		return err
	}
	ok, err := s.confirmConflicts(s.mgr.UploadConflicts(sessionID, remotePath, []string{localPath}))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return s.mgr.UploadFile(sessionID, localPath, remotePath)
}

func (s *SftpService) DownloadFileDialog(sessionID, remotePathFile string) error {
	localPathFile, err := promptSaveFile(s.app, s.store, i18n.T("dialog.save_file"), remotePathFile)
	if dialogCancelled(err) || localPathFile == "" {
		return nil
	}
	if err != nil {
		return err
	}
	ok, err := s.confirmLocalPathOverwrite(localPathFile)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return s.mgr.DownloadFile(sessionID, localPathFile, remotePathFile)
}

func (s *SftpService) UploadDirectoryDialog(sessionID, remotePath string) error {
	localPath, err := promptOpenDirectory(s.app, s.store, i18n.T("dialog.select_dir"))
	if dialogCancelled(err) || localPath == "" {
		return nil
	}
	if err != nil {
		return err
	}
	ok, err := s.confirmConflicts(s.mgr.UploadConflicts(sessionID, remotePath, []string{localPath}))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return s.mgr.UploadDirectory(sessionID, localPath, remotePath)
}

func (s *SftpService) DownloadDirectoryDialog(sessionID, remotePath string) error {
	dest, err := promptSaveFile(s.app, s.store, i18n.T("dialog.save_dir"), remotePath)
	if dialogCancelled(err) || dest == "" {
		return nil
	}
	if err != nil {
		return err
	}
	ok, err := s.confirmLocalPathOverwrite(dest)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return s.mgr.DownloadDirectoryTo(sessionID, dest, remotePath)
}

func (s *SftpService) UploadPaths(sessionID, remotePath string, localPaths []string) error {
	return s.mgr.UploadPaths(sessionID, remotePath, localPaths)
}

// DownloadPaths downloads remote files and directories into localDir.
func (s *SftpService) DownloadPaths(sessionID, localDir string, remotePaths []string) error {
	return s.mgr.DownloadPaths(sessionID, localDir, remotePaths)
}

func (s *SftpService) DownloadPathsDialog(sessionID string, remotePaths []string) error {
	localDir, err := promptOpenDirectory(s.app, s.store, i18n.T("dialog.select_dir"))
	if dialogCancelled(err) || localDir == "" {
		return nil
	}
	if err != nil {
		return err
	}
	ok, err := s.confirmConflicts(s.mgr.DownloadConflicts(sessionID, localDir, remotePaths))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return s.mgr.DownloadPaths(sessionID, localDir, remotePaths)
}

func (s *SftpService) DeleteFiles(sessionID string, paths []string) error {
	return s.mgr.DeleteFiles(sessionID, paths)
}

func (s *SftpService) RenameFile(sessionID, oldPath, newPath string) error {
	return s.mgr.RenameFile(sessionID, oldPath, newPath)
}

func (s *SftpService) Chmod(sessionID, path string, modeBits uint32) error {
	return s.mgr.Chmod(sessionID, path, modeBits)
}

func (s *SftpService) Chown(sessionID, path string, uid, gid int) error {
	return s.mgr.Chown(sessionID, path, uid, gid)
}

func (s *SftpService) GetWd(sessionID string) (string, error) {
	return s.mgr.GetWd(sessionID)
}

func (s *SftpService) CreateFile(sessionID, path string) error {
	return s.mgr.CreateFile(sessionID, path)
}

func (s *SftpService) CreateDirectory(sessionID, path string) error {
	return s.mgr.CreateDirectory(sessionID, path)
}

func (s *SftpService) CancelTransfer(transferID string) error {
	return s.mgr.CancelTransfer(transferID)
}

func (s *SftpService) ListPendingTransfers(ownerKey string) ([]ProgressData, error) {
	return s.mgr.ListPendingTransfers(ownerKey)
}

func (s *SftpService) RetryTransfer(sessionID, transferID string) error {
	return s.mgr.RetryTransfer(sessionID, transferID)
}

func (s *SftpService) DismissTransfer(transferID string) error {
	return s.mgr.DismissTransfer(transferID)
}
