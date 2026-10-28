package services

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

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
	logger *zap.Logger
	edits  *editBook
}

func NewSftpService(app *application.App, mgr *sftp.Manager, logger *zap.Logger) *SftpService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SftpService{app: app, mgr: mgr, logger: logger, edits: newEditBook()}
}

func (s *SftpService) ListFiles(sessionID, path string, showHidden bool) ([]FileInfo, error) {
	return s.mgr.ListFiles(sessionID, path, showHidden)
}

// ListLocal reads a local directory. An empty dir uses the user home directory.
func (s *SftpService) ListLocal(dir string, showHidden bool) (localfs.Listing, error) {
	return localfs.List(dir, showHidden)
}
func (s *SftpService) UploadFileDialog(sessionID, remotePath string) error {
	localPath, err := s.app.Dialog.OpenFile().SetTitle(i18n.T("dialog.select_file")).PromptForSingleSelection()
	if dialogCancelled(err) || localPath == "" {
		return nil
	}
	if err != nil {
		return err
	}
	return s.mgr.UploadFile(sessionID, localPath, remotePath)
}
func (s *SftpService) DownloadFileDialog(sessionID, remotePathFile string) error {
	localPathFile, err := promptSaveFile(s.app, i18n.T("dialog.save_file"), remotePathFile)
	if dialogCancelled(err) || localPathFile == "" {
		return nil
	}
	if err != nil {
		return err
	}
	return s.mgr.DownloadFile(sessionID, localPathFile, remotePathFile)
}
func (s *SftpService) UploadDirectoryDialog(sessionID, remotePath string) error {
	localPath, err := s.app.Dialog.OpenFile().SetTitle(i18n.T("dialog.select_dir")).
		CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
	if dialogCancelled(err) || localPath == "" {
		return nil
	}
	if err != nil {
		return err
	}
	return s.mgr.UploadDirectory(sessionID, localPath, remotePath)
}
func (s *SftpService) DownloadDirectoryDialog(sessionID, remotePath string) error {
	dest, err := promptSaveFile(s.app, i18n.T("dialog.save_dir"), remotePath)
	if dialogCancelled(err) || dest == "" {
		return nil
	}
	if err != nil {
		return err
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
	localDir, err := promptOpenDirectory(s.app, i18n.T("dialog.select_dir"))
	if dialogCancelled(err) || localDir == "" {
		return nil
	}
	if err != nil {
		return err
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
