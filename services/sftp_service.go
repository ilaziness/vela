package services

import (
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

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
	app *application.App
	mgr *sftp.Manager
}

func NewSftpService(app *application.App, mgr *sftp.Manager) *SftpService {
	return &SftpService{app: app, mgr: mgr}
}

func (s *SftpService) ListFiles(sessionID, path string, showHidden bool) ([]FileInfo, error) {
	return s.mgr.ListFiles(sessionID, path, showHidden)
}
func (s *SftpService) UploadFileDialog(sessionID, remotePath string) error {
	localPath, err := s.app.Dialog.OpenFile().SetTitle("选择文件").PromptForSingleSelection()
	if dialogCancelled(err) || localPath == "" {
		return nil
	}
	if err != nil {
		return err
	}
	return s.mgr.UploadFile(sessionID, localPath, remotePath)
}
func (s *SftpService) DownloadFileDialog(sessionID, remotePathFile string) error {
	localPathFile, err := s.app.Dialog.SaveFile().
		SetMessage("保存文件").SetFilename(filepath.Base(remotePathFile)).
		CanCreateDirectories(true).PromptForSingleSelection()
	if dialogCancelled(err) || localPathFile == "" {
		return nil
	}
	if err != nil {
		return err
	}
	return s.mgr.DownloadFile(sessionID, localPathFile, remotePathFile)
}
func (s *SftpService) UploadDirectoryDialog(sessionID, remotePath string) error {
	localPath, err := s.app.Dialog.OpenFile().SetTitle("选择目录").
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
	localPath, err := s.app.Dialog.OpenFile().SetTitle("选择目录").
		CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
	if dialogCancelled(err) || localPath == "" {
		return nil
	}
	if err != nil {
		return err
	}
	return s.mgr.DownloadDirectory(sessionID, localPath, remotePath)
}
func (s *SftpService) UploadPaths(sessionID, remotePath string, localPaths []string) error {
	return s.mgr.UploadPaths(sessionID, remotePath, localPaths)
}
func (s *SftpService) DownloadPathsDialog(sessionID string, remotePaths []string) error {
	localDir, err := s.app.Dialog.OpenFile().SetTitle("选择保存目录").
		CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
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
