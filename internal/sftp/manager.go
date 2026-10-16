package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/transfer"
)

const (
	ErrTrackerRequired = "tracker is required"
)

// FileInfo represents file information for SFTP operations
type FileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	ModeBits uint32 `json:"modeBits"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	ModTime  string `json:"modTime"`
	IsDir    bool   `json:"isDir"`
}

func convertFileInfo(osInfo os.FileInfo) FileInfo {
	fm := osInfo.Mode()
	fi := FileInfo{
		Name:    osInfo.Name(),
		Size:    osInfo.Size(),
		Mode:    fm.String(),
		ModTime: osInfo.ModTime().Format(time.RFC3339),
		IsDir:   osInfo.IsDir(),
	}
	if stat, ok := osInfo.Sys().(*sftp.FileStat); ok && stat != nil {
		fm = stat.FileMode()
		fi.Mode = fm.String()
		fi.UID = int(stat.UID)
		fi.GID = int(stat.GID)
	}
	// Unix permission bits (0o7777) so frontend octal edit preserves setuid/setgid/sticky.
	fi.ModeBits = fileModeToUnix(fm)
	return fi
}

func convertFileInfos(osInfos []os.FileInfo) []FileInfo {
	result := make([]FileInfo, len(osInfos))
	for i, osInfo := range osInfos {
		result[i] = convertFileInfo(osInfo)
	}
	return result
}

type Manager struct {
	logger    *zap.Logger
	ssh       *ssh.Manager
	transfers *transfer.Registry
	queue     transfer.QueueStore
	onQueued  func(sessionID string, count int)
	clients   sync.Map
	owners    sync.Map // sessionID -> ownerKey
}

func NewManager(logger *zap.Logger, sshMgr *ssh.Manager, transfers *transfer.Registry, queue transfer.QueueStore) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{logger: logger, ssh: sshMgr, transfers: transfers, queue: queue}
}

// SetOnQueued registers a callback after transfers are successfully enqueued.
func (sft *Manager) SetOnQueued(fn func(sessionID string, count int)) {
	sft.onQueued = fn
}

func (sft *Manager) notifyQueued(sessionID string, count int) {
	if count <= 0 || sft.onQueued == nil {
		return
	}
	sft.onQueued(sessionID, count)
}

func joinRemotePath(elem ...string) string {
	return path.Join(elem...)
}

func (sft *Manager) getSftpClient(sessionID string) (*sftp.Client, error) {
	connVal, ok := sft.clients.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf("SSH session with ID %s not found", sessionID)
	}
	return connVal.(*sftp.Client), nil
}

func (sft *Manager) ownerKey(sessionID string) string {
	if v, ok := sft.owners.Load(sessionID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Connect opens an SFTP client and binds ownerKey for transfer queue ownership.
func (sft *Manager) Connect(sessionID, ownerKey string) error {
	if ownerKey != "" {
		sft.owners.Store(sessionID, ownerKey)
	}
	if _, err := sft.getSftpClient(sessionID); err == nil {
		return nil
	}
	sshClient, err := sft.ssh.GetClient(sessionID)
	if err != nil {
		return err
	}
	ftpClient, err := sftp.NewClient(
		sshClient,
		sftp.MaxPacket(32768),
		sftp.UseConcurrentWrites(true),
		sftp.UseConcurrentReads(true),
	)
	if err != nil {
		return err
	}
	sft.clients.Store(sessionID, ftpClient)
	return nil
}

func (sft *Manager) ListFiles(sessionID string, path string, showHidden bool) ([]FileInfo, error) {
	sft.logger.Debug("ListFiles", zap.String("sessionID", sessionID), zap.String("path", path), zap.Bool("showHidden", showHidden))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()
	entries, err := ftpClient.ReadDirContext(ctx, path)
	if err != nil {
		return nil, err
	}

	if !showHidden {
		var filteredEntries []os.FileInfo
		for _, entry := range entries {
			if len(entry.Name()) > 0 && entry.Name()[0] != '.' {
				filteredEntries = append(filteredEntries, entry)
			}
		}
		entries = filteredEntries
	}

	return convertFileInfos(entries), nil
}

func (sft *Manager) newTracker(sessionID, transferType, localFile, remoteFile string, total int64, opts transfer.NewOpts) (*transfer.Tracker, error) {
	if opts.OwnerKey == "" {
		opts.OwnerKey = sft.ownerKey(sessionID)
	}
	t, err := sft.transfers.NewWith(sessionID, transferType, localFile, remoteFile, total, opts)
	if err != nil {
		return nil, err
	}
	sft.queueUpsertRunning(t)
	return t, nil
}

func (sft *Manager) queueUpsertRunning(t *transfer.Tracker) {
	if sft.queue == nil || t.OwnerKey() == "" {
		return
	}
	if err := sft.queue.Upsert(transfer.QueueRecord{
		ID:           t.ID(),
		OwnerKey:     t.OwnerKey(),
		TransferType: t.TransferType(),
		LocalFile:    t.LocalFile(),
		RemoteFile:   t.RemoteFile(),
		TotalSize:    t.TotalSize(),
		Status:       transfer.StatusRunning,
		Overwrite:    t.Overwrite(),
	}); err != nil {
		sft.logger.Warn("queue upsert failed", zap.String("id", t.ID()), zap.Error(err))
	}
}

func (sft *Manager) stopTracker(t *transfer.Tracker, err error) {
	t.Stop(err)
	if sft.queue == nil {
		return
	}
	if err == nil {
		if delErr := sft.queue.Delete(t.ID()); delErr != nil {
			sft.logger.Warn("queue delete failed", zap.String("id", t.ID()), zap.Error(delErr))
		}
		return
	}
	status := transfer.StatusFailed
	if t.WasInterrupted() {
		status = transfer.StatusInterrupted
	} else if t.WasCancelled() {
		status = transfer.StatusCancelled
	}
	if setErr := sft.queue.SetStatus(t.ID(), status, err.Error()); setErr != nil {
		sft.logger.Warn("queue set status failed", zap.String("id", t.ID()), zap.Error(setErr))
	}
}

// uploadFile uploads a local file; remoteDir is the remote parent directory.
func (sft *Manager) uploadFile(sessionID string, localPathFile, remoteDir string, tracker *transfer.Tracker) error {
	sft.logger.Debug("uploadFile", zap.String("sessionID", sessionID), zap.String("localPathFile", localPathFile), zap.String("remoteDir", remoteDir))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}

	localInfo, err := os.Stat(localPathFile)
	if err != nil {
		return err
	}
	sourceSize := localInfo.Size()

	localFile, err := os.Open(localPathFile)
	if err != nil {
		return err
	}
	defer localFile.Close()

	_, err = ftpClient.Stat(remoteDir)
	if err != nil && os.IsNotExist(err) {
		if err = ftpClient.MkdirAll(remoteDir); err != nil {
			return err
		}
	}

	remoteFilePath := joinRemotePath(remoteDir, filepath.Base(localPathFile))
	if tracker != nil && tracker.Overwrite() {
		if tracker.RemoteFile() != "" {
			remoteFilePath = tracker.RemoteFile()
		}
		return sft.uploadFileReplace(ftpClient, localFile, remoteFilePath, tracker)
	}

	var destSize int64
	if st, statErr := ftpClient.Stat(remoteFilePath); statErr == nil && !st.IsDir() {
		destSize = st.Size()
	}

	action, offset := DecideResume(destSize, sourceSize)
	if action == ResumeSkip {
		if tracker != nil {
			tracker.AddTransferred(sourceSize)
		}
		return nil
	}

	var remoteFile *sftp.File
	if action == ResumeContinue {
		remoteFile, err = ftpClient.OpenFile(remoteFilePath, os.O_WRONLY)
		if err != nil {
			return err
		}
		if _, err = remoteFile.Seek(offset, io.SeekStart); err != nil {
			_ = remoteFile.Close()
			return err
		}
		if _, err = localFile.Seek(offset, io.SeekStart); err != nil {
			_ = remoteFile.Close()
			return err
		}
		if tracker != nil {
			tracker.AddTransferred(offset)
		}
	} else {
		sft.logger.Debug("uploadFile Create File", zap.String("file", remoteFilePath))
		remoteFile, err = ftpClient.Create(remoteFilePath)
		if err != nil {
			return err
		}
	}
	defer remoteFile.Close()

	if tracker == nil {
		return errors.New(ErrTrackerRequired)
	}

	progressReader := &transfer.Reader{Reader: localFile, Tracker: tracker}
	_, err = io.Copy(remoteFile, progressReader)
	return err
}

const posixRenameExt = "posix-rename@openssh.com"

func editStagingRemotePath(final string) string {
	dir := path.Dir(final)
	name := "." + path.Base(final) + ".vela-edit"
	if dir == "." || dir == "" {
		return name
	}
	return path.Join(dir, name)
}

// stagingReplacer is the subset of *sftp.Client used to swap in an edit upload.
type stagingReplacer interface {
	HasExtension(name string) (string, bool)
	PosixRename(oldname, newname string) error
	Rename(oldname, newname string) error
	Stat(p string) (os.FileInfo, error)
	Remove(path string) error
}

// uploadFileReplace writes a sibling temp file, then replaces the original.
// It does not truncate the original first and does not call DecideResume.
func (sft *Manager) uploadFileReplace(client *sftp.Client, local *os.File, remoteFinal string, tracker *transfer.Tracker) error {
	if tracker == nil {
		return errors.New(ErrTrackerRequired)
	}
	if remoteFinal == "" {
		return fmt.Errorf("remote path is empty")
	}
	staging := editStagingRemotePath(remoteFinal)
	if staging == remoteFinal {
		return fmt.Errorf("edit staging path equals remote path")
	}
	if _, err := local.Seek(0, io.SeekStart); err != nil {
		return err
	}
	remoteFile, err := client.Create(staging)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(remoteFile, &transfer.Reader{Reader: local, Tracker: tracker})
	closeErr := remoteFile.Close()
	if copyErr != nil {
		sft.removeStaging(client, staging)
		return copyErr
	}
	if closeErr != nil {
		sft.removeStaging(client, staging)
		return closeErr
	}
	return sft.replaceStagedFile(client, staging, remoteFinal)
}

// replaceStagedFile moves staging onto remoteFinal.
// PosixRename replaces an existing file without deleting it first.
// Without that extension the original is removed and then renamed; if that
// rename fails the new bytes exist only at staging.
func (sft *Manager) replaceStagedFile(client stagingReplacer, staging, remoteFinal string) error {
	if _, ok := client.HasExtension(posixRenameExt); ok {
		err := client.PosixRename(staging, remoteFinal)
		if err == nil {
			return nil
		}
		if !sftpOpUnsupported(err) {
			sft.removeStaging(client, staging)
			return err
		}
		sft.logger.Warn("posix-rename unsupported, falling back to remove then rename",
			zap.String("staging", staging), zap.String("remote", remoteFinal), zap.Error(err))
	} else {
		sft.logger.Debug("sftp server has no posix-rename; remove then rename",
			zap.String("staging", staging), zap.String("remote", remoteFinal))
	}
	removed := false
	if st, statErr := client.Stat(remoteFinal); statErr == nil {
		if st.IsDir() {
			sft.removeStaging(client, staging)
			return fmt.Errorf("remote path is a directory")
		}
		if rmErr := client.Remove(remoteFinal); rmErr != nil {
			sft.removeStaging(client, staging)
			return rmErr
		}
		removed = true
	} else if !os.IsNotExist(statErr) {
		sft.removeStaging(client, staging)
		return statErr
	}
	if err := client.Rename(staging, remoteFinal); err != nil {
		if _, stErr := client.Stat(remoteFinal); stErr == nil {
			sft.removeStaging(client, staging)
			return err
		}
		if removed {
			sft.logger.Error("edit rename failed after removing original",
				zap.String("staging", staging), zap.String("remote", remoteFinal), zap.Error(err))
			return fmt.Errorf("original remote file %s was removed but renaming %s into place failed; new contents remain at %s: %w",
				remoteFinal, staging, staging, err)
		}
		sft.logger.Error("edit rename failed; new contents remain in staging",
			zap.String("staging", staging), zap.String("remote", remoteFinal), zap.Error(err))
		return fmt.Errorf("rename %s to %s failed; new contents remain at %s: %w", staging, remoteFinal, staging, err)
	}
	return nil
}

func sftpOpUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sftp.ErrSSHFxOpUnsupported) {
		return true
	}
	var st *sftp.StatusError
	return errors.As(err, &st) && st.FxCode() == sftp.ErrSSHFxOpUnsupported
}

func (sft *Manager) removeStaging(client stagingReplacer, staging string) {
	if err := client.Remove(staging); err != nil && !os.IsNotExist(err) {
		sft.logger.Warn("delete remote edit temp failed", zap.String("path", staging), zap.Error(err))
	}
}

func (sft *Manager) downloadFileTruncate(client *sftp.Client, localPathFile, remotePathFile string, tracker *transfer.Tracker) error {
	if tracker == nil {
		return errors.New(ErrTrackerRequired)
	}
	remoteFile, err := client.Open(remotePathFile)
	if err != nil {
		return err
	}
	defer remoteFile.Close()
	localFile, err := os.OpenFile(localPathFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer localFile.Close()
	_, err = io.Copy(&transfer.Writer{Writer: localFile, Tracker: tracker}, remoteFile)
	return err
}

func (sft *Manager) UploadFile(sessionID, localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if _, err := sft.startUploadFileAsync(sessionID, localPath, remotePath, info.Size(), transfer.NewOpts{}); err != nil {
		return err
	}
	sft.notifyQueued(sessionID, 1)
	return nil
}

func (sft *Manager) startUploadFileAsync(sessionID, localPath, remotePath string, size int64, opts transfer.NewOpts) (*transfer.Tracker, error) {
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, localPath, joinRemotePath(remotePath, filepath.Base(localPath)), size, opts)
	if err != nil {
		return nil, err
	}
	go func() {
		err := sft.uploadFile(sessionID, localPath, remotePath, tracker)
		sft.stopTracker(tracker, err)
	}()
	return tracker, nil
}

// startUploadReplaceAsync uploads to remoteFile by writing a sibling temp then renaming.
// remoteFile is the final path stored on the queue, not the staging name.
func (sft *Manager) startUploadReplaceAsync(sessionID, localPath, remoteFile string, size int64, opts transfer.NewOpts) (*transfer.Tracker, error) {
	opts.Overwrite = true
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, localPath, remoteFile, size, opts)
	if err != nil {
		return nil, err
	}
	go func() {
		err := sft.uploadFile(sessionID, localPath, path.Dir(remoteFile), tracker)
		sft.stopTracker(tracker, err)
	}()
	return tracker, nil
}

func (sft *Manager) downloadFile(sessionID string, localPathFile, remotePathFile string, tracker *transfer.Tracker) error {
	sft.logger.Debug("downloadFile", zap.String("sessionID", sessionID), zap.String("localPath", localPathFile), zap.String("remotePath", remotePathFile))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}

	remoteInfo, err := ftpClient.Stat(remotePathFile)
	if err != nil {
		return err
	}
	sourceSize := remoteInfo.Size()

	localDir := filepath.Dir(localPathFile)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}
	if tracker != nil && tracker.Overwrite() {
		return sft.downloadFileTruncate(ftpClient, localPathFile, remotePathFile, tracker)
	}

	var destSize int64
	if st, statErr := os.Stat(localPathFile); statErr == nil && !st.IsDir() {
		destSize = st.Size()
	}

	action, offset := DecideResume(destSize, sourceSize)
	if action == ResumeSkip {
		if tracker != nil {
			tracker.AddTransferred(sourceSize)
		}
		return nil
	}

	remoteFile, err := ftpClient.Open(remotePathFile)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	var localFile *os.File
	if action == ResumeContinue {
		localFile, err = os.OpenFile(localPathFile, os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		if _, err = localFile.Seek(offset, io.SeekStart); err != nil {
			_ = localFile.Close()
			return err
		}
		if _, err = remoteFile.Seek(offset, io.SeekStart); err != nil {
			_ = localFile.Close()
			return err
		}
		if tracker != nil {
			tracker.AddTransferred(offset)
		}
	} else {
		localFile, err = os.OpenFile(localPathFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
	}
	defer localFile.Close()

	if tracker == nil {
		return errors.New(ErrTrackerRequired)
	}

	progressWriter := &transfer.Writer{Writer: localFile, Tracker: tracker}
	_, err = io.Copy(progressWriter, remoteFile)
	return err
}

func (sft *Manager) DownloadFile(sessionID, localPathFile, remotePathFile string) error {
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	info, err := ftpClient.Stat(remotePathFile)
	if err != nil {
		return err
	}
	if _, err := sft.startDownloadFileAsync(sessionID, localPathFile, remotePathFile, info.Size(), transfer.NewOpts{}); err != nil {
		return err
	}
	sft.notifyQueued(sessionID, 1)
	return nil
}

func (sft *Manager) startDownloadFileAsync(sessionID, localPathFile, remotePathFile string, size int64, opts transfer.NewOpts) (*transfer.Tracker, error) {
	tracker, err := sft.newTracker(sessionID, transfer.TypeDownload, localPathFile, remotePathFile, size, opts)
	if err != nil {
		return nil, err
	}
	go func() {
		err := sft.downloadFile(sessionID, localPathFile, remotePathFile, tracker)
		sft.stopTracker(tracker, err)
	}()
	return tracker, nil
}

// StartOverwriteDownload queues a full download that truncates the local file.
func (sft *Manager) StartOverwriteDownload(sessionID, localFile, remoteFile string, size int64) (*transfer.Tracker, error) {
	t, err := sft.startDownloadFileAsync(sessionID, localFile, remoteFile, size, transfer.NewOpts{Overwrite: true})
	if err != nil {
		return nil, err
	}
	sft.notifyQueued(sessionID, 1)
	return t, nil
}

// StartOverwriteUpload queues a replace upload. remoteFile is the final path.
func (sft *Manager) StartOverwriteUpload(sessionID, localFile, remoteFile string, size int64) (*transfer.Tracker, error) {
	t, err := sft.startUploadReplaceAsync(sessionID, localFile, remoteFile, size, transfer.NewOpts{Overwrite: true})
	if err != nil {
		return nil, err
	}
	sft.notifyQueued(sessionID, 1)
	return t, nil
}

func (sft *Manager) UploadDirectory(sessionID, localPath, remotePath string) error {
	total, err := sft.calcLocalDirSize(localPath)
	if err != nil {
		return err
	}
	if err := sft.startUploadDirectoryAsync(sessionID, localPath, remotePath, total, transfer.NewOpts{}); err != nil {
		return err
	}
	sft.notifyQueued(sessionID, 1)
	return nil
}

func (sft *Manager) startUploadDirectoryAsync(sessionID, localPath, remotePath string, total int64, opts transfer.NewOpts) error {
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, localPath, joinRemotePath(remotePath, filepath.Base(localPath)), total, opts)
	if err != nil {
		return err
	}
	go func() {
		err := sft.uploadDirectory(sessionID, localPath, remotePath, tracker)
		sft.stopTracker(tracker, err)
	}()
	return nil
}

type uploadJob struct {
	path  string
	isDir bool
	size  int64
}

func (sft *Manager) UploadPaths(sessionID, remotePath string, localPaths []string) error {
	if sessionID == "" || remotePath == "" || len(localPaths) == 0 {
		return fmt.Errorf("invalid upload parameters")
	}
	if _, err := sft.getSftpClient(sessionID); err != nil {
		return err
	}
	jobs := make([]uploadJob, 0, len(localPaths))
	for _, p := range localPaths {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("local path %s: %w", p, err)
		}
		job := uploadJob{path: p, isDir: info.IsDir(), size: info.Size()}
		if info.IsDir() {
			total, err := sft.calcLocalDirSize(p)
			if err != nil {
				return fmt.Errorf("local path %s: %w", p, err)
			}
			job.size = total
		}
		jobs = append(jobs, job)
	}
	queued := 0
	for _, job := range jobs {
		var startErr error
		if job.isDir {
			startErr = sft.startUploadDirectoryAsync(sessionID, job.path, remotePath, job.size, transfer.NewOpts{})
		} else {
			_, startErr = sft.startUploadFileAsync(sessionID, job.path, remotePath, job.size, transfer.NewOpts{})
		}
		if startErr != nil {
			sft.logger.Warn("enqueue upload failed", zap.String("path", job.path), zap.Error(startErr))
			continue
		}
		queued++
	}
	sft.notifyQueued(sessionID, queued)
	return nil
}

func (sft *Manager) uploadDirectory(sessionID string, localPath, remotePath string, tracker *transfer.Tracker) error {
	sft.logger.Debug("uploadDirectory", zap.String("sessionID", sessionID), zap.String("localPath", localPath), zap.String("remotePath", remotePath))

	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if tracker == nil {
		return errors.New(ErrTrackerRequired)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	remoteDir := joinRemotePath(remotePath, filepath.Base(localPath))
	if err := sft.ensureRemoteDirExists(ftpClient, remoteDir); err != nil {
		return err
	}

	return sft.processDirectoryEntries(sessionID, localPath, remoteDir, tracker)
}

func (sft *Manager) ensureRemoteDirExists(ftpClient *sftp.Client, remoteDir string) error {
	_, err := ftpClient.Stat(remoteDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		return ftpClient.MkdirAll(remoteDir)
	}
	return nil
}

func (sft *Manager) processDirectoryEntries(sessionID, localPath, remoteDir string, tracker *transfer.Tracker) error {
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		localEntryPath := filepath.Join(localPath, entry.Name())
		if entry.IsDir() {
			err = sft.uploadDirectory(sessionID, localEntryPath, remoteDir, tracker)
			if err != nil {
				return err
			}
		} else {
			err = sft.uploadFile(sessionID, localEntryPath, remoteDir, tracker)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (sft *Manager) DownloadDirectory(sessionID, localPath, remotePath string) error {
	total, err := sft.calcRemoteDirSize(sessionID, remotePath)
	if err != nil {
		return err
	}
	if err := sft.startDownloadDirectoryAsync(sessionID, localPath, remotePath, total, transfer.NewOpts{}); err != nil {
		return err
	}
	sft.notifyQueued(sessionID, 1)
	return nil
}

func (sft *Manager) startDownloadDirectoryAsync(sessionID, localPath, remotePath string, total int64, opts transfer.NewOpts) error {
	tracker, err := sft.newTracker(sessionID, transfer.TypeDownload, localPath, remotePath, total, opts)
	if err != nil {
		return err
	}
	go func() {
		err := sft.downloadDirectory(sessionID, localPath, remotePath, tracker)
		sft.stopTracker(tracker, err)
	}()
	return nil
}

func (sft *Manager) downloadDirectory(sessionID string, localPath, remotePath string, tracker *transfer.Tracker) error {
	sft.logger.Debug("downloadDirectory", zap.String("sessionID", sessionID), zap.String("localPath", localPath), zap.String("remotePath", remotePath))

	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if tracker == nil {
		return errors.New(ErrTrackerRequired)
	}

	info, err := ftpClient.Stat(remotePath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	localDir := filepath.Join(localPath, path.Base(remotePath))
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}

	return sft.processRemoteDirectoryEntries(sessionID, localDir, remotePath, tracker)
}

func (sft *Manager) processRemoteDirectoryEntries(sessionID, localDir, remotePath string, tracker *transfer.Tracker) error {
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}

	entries, err := ftpClient.ReadDir(remotePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		remoteEntryPath := joinRemotePath(remotePath, entry.Name())
		localEntryPath := filepath.Join(localDir, entry.Name())

		if entry.IsDir() {
			err = sft.downloadDirectory(sessionID, localDir, remoteEntryPath, tracker)
			if err != nil {
				return err
			}
		} else {
			err = sft.downloadFile(sessionID, localEntryPath, remoteEntryPath, tracker)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type downloadJob struct {
	remote string
	isDir  bool
	size   int64
}

// DownloadPaths downloads remote files/dirs into localDir asynchronously.
func (sft *Manager) DownloadPaths(sessionID, localDir string, remotePaths []string) error {
	if sessionID == "" || localDir == "" || len(remotePaths) == 0 {
		return fmt.Errorf("invalid download parameters")
	}
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}
	jobs := make([]downloadJob, 0, len(remotePaths))
	for _, rp := range remotePaths {
		info, err := ftpClient.Stat(rp)
		if err != nil {
			return fmt.Errorf("remote path %s: %w", rp, err)
		}
		job := downloadJob{remote: rp, isDir: info.IsDir(), size: info.Size()}
		if info.IsDir() {
			total, err := sft.calcRemoteDirSize(sessionID, rp)
			if err != nil {
				return fmt.Errorf("remote path %s: %w", rp, err)
			}
			job.size = total
		}
		jobs = append(jobs, job)
	}
	queued := 0
	for _, job := range jobs {
		var startErr error
		if job.isDir {
			startErr = sft.startDownloadDirectoryAsync(sessionID, localDir, job.remote, job.size, transfer.NewOpts{})
		} else {
			localFile := filepath.Join(localDir, path.Base(job.remote))
			_, startErr = sft.startDownloadFileAsync(sessionID, localFile, job.remote, job.size, transfer.NewOpts{})
		}
		if startErr != nil {
			sft.logger.Warn("enqueue download failed", zap.String("path", job.remote), zap.Error(startErr))
			continue
		}
		queued++
	}
	sft.notifyQueued(sessionID, queued)
	return nil
}

func (sft *Manager) DeleteFile(sessionID string, path string) error {
	sft.logger.Debug("DeleteFile", zap.String("sessionID", sessionID), zap.String("path", path))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}

	info, err := ftpClient.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return ftpClient.RemoveAll(path)
	}
	return ftpClient.Remove(path)
}

// DeleteFiles deletes multiple remote paths; returns the first error after logging others.
func (sft *Manager) DeleteFiles(sessionID string, paths []string) error {
	var first error
	for _, p := range paths {
		if err := sft.DeleteFile(sessionID, p); err != nil {
			sft.logger.Error("DeleteFiles item failed", zap.String("path", p), zap.Error(err))
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func (sft *Manager) RenameFile(sessionID string, oldPath, newPath string) error {
	sft.logger.Debug("RenameFile", zap.String("sessionID", sessionID), zap.String("oldPath", oldPath), zap.String("newPath", newPath))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if _, err := ftpClient.Stat(newPath); err == nil {
		return fmt.Errorf("%s exists", newPath)
	}

	return ftpClient.Rename(oldPath, newPath)
}

func (sft *Manager) Chmod(sessionID, path string, modeBits uint32) error {
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	return ftpClient.Chmod(path, unixToFileMode(modeBits))
}

func (sft *Manager) Chown(sessionID, path string, uid, gid int) error {
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	return ftpClient.Chown(path, uid, gid)
}

func (sft *Manager) Stat(sessionID, remotePath string) (FileInfo, error) {
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := ftpClient.Stat(remotePath)
	if err != nil {
		return FileInfo{}, err
	}
	return convertFileInfo(info), nil
}

func (sft *Manager) GetWd(sessionID string) (string, error) {
	sft.logger.Debug("GetWd", zap.String("sessionID", sessionID))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return "", err
	}
	return ftpClient.Getwd()
}

func (sft *Manager) CloseSession(sessionID string) {
	sft.interruptSession(sessionID)
	sft.owners.Delete(sessionID)
	if val, ok := sft.clients.LoadAndDelete(sessionID); ok {
		if err := val.(*sftp.Client).Close(); err != nil {
			sft.logger.Warn("Error closing SFTP connection:", zap.Error(err))
		}
	}
}

func (sft *Manager) interruptSession(sessionID string) {
	ids := sft.transfers.InterruptSession(sessionID)
	if sft.queue == nil {
		return
	}
	for _, id := range ids {
		if err := sft.queue.SetStatus(id, transfer.StatusInterrupted, "interrupted"); err != nil {
			sft.logger.Warn("queue interrupt status failed", zap.String("id", id), zap.Error(err))
		}
	}
}

func (sft *Manager) CloseAll() {
	ids := sft.transfers.InterruptAll()
	if sft.queue != nil {
		for _, id := range ids {
			if err := sft.queue.SetStatus(id, transfer.StatusInterrupted, "interrupted"); err != nil {
				sft.logger.Warn("queue interrupt status failed", zap.String("id", id), zap.Error(err))
			}
		}
	}
	sft.clients.Range(func(key, value any) bool {
		if err := value.(*sftp.Client).Close(); err != nil {
			sft.logger.Warn("Error closing SFTP connection:", zap.Error(err))
		}
		sft.clients.Delete(key)
		sft.owners.Delete(key)
		return true
	})
}

func (sft *Manager) CreateFile(sessionID string, path string) error {
	sft.logger.Debug("CreateFile", zap.String("sessionID", sessionID), zap.String("path", path))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if _, err := ftpClient.Stat(path); err == nil {
		return fmt.Errorf("file already exists")
	}
	_, err = ftpClient.Create(path)
	return err
}

func (sft *Manager) CreateDirectory(sessionID string, path string) error {
	sft.logger.Debug("CreateDirectory", zap.String("sessionID", sessionID), zap.String("path", path))
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if _, err := ftpClient.Stat(path); err == nil {
		return fmt.Errorf("directory already exists")
	}
	return ftpClient.MkdirAll(path)
}

func (sft *Manager) CancelTransfer(transferID string) error {
	sft.logger.Debug("CancelTransfer", zap.String("transferID", transferID))
	return sft.transfers.Cancel(transferID)
}

func (sft *Manager) ListPendingTransfers(ownerKey string) ([]transfer.ProgressData, error) {
	if ownerKey == "" || sft.queue == nil {
		return nil, nil
	}
	recs, err := sft.queue.ListByOwner(ownerKey)
	if err != nil {
		return nil, err
	}
	out := make([]transfer.ProgressData, 0, len(recs))
	for _, rec := range recs {
		if rec.Status == transfer.StatusRunning {
			if sft.transfers.HasActive(rec.ID) {
				// Another session still owns this transfer; omit from pending UI.
				continue
			}
			rec.Status = transfer.StatusInterrupted
			if rec.Error == "" {
				rec.Error = "interrupted"
			}
			_ = sft.queue.SetStatus(rec.ID, transfer.StatusInterrupted, rec.Error)
		}
		out = append(out, transfer.RecordToProgress(rec, ""))
	}
	return out, nil
}

func (sft *Manager) DismissTransfer(transferID string) error {
	if sft.queue == nil {
		return nil
	}
	return sft.queue.Delete(transferID)
}

// RetryTransfer re-queues a persisted transfer using the same ID.
func (sft *Manager) RetryTransfer(sessionID, transferID string) error {
	if sft.queue == nil {
		return fmt.Errorf("transfer queue unavailable")
	}
	rec, err := sft.queue.Get(transferID)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("transfer not found")
	}
	switch rec.Status {
	case transfer.StatusFailed, transfer.StatusCancelled, transfer.StatusInterrupted:
	default:
		return fmt.Errorf("transfer is not retryable (status=%s)", rec.Status)
	}
	owner := sft.ownerKey(sessionID)
	if owner == "" || rec.OwnerKey != owner {
		return fmt.Errorf("transfer owner mismatch")
	}
	if sft.transfers.HasActive(transferID) {
		return fmt.Errorf("transfer already active")
	}
	if _, err := sft.getSftpClient(sessionID); err != nil {
		return err
	}

	opts := transfer.NewOpts{ID: rec.ID, OwnerKey: rec.OwnerKey, Overwrite: rec.Overwrite}
	var startErr error
	switch strings.ToLower(rec.TransferType) {
	case transfer.TypeUpload:
		info, err := os.Stat(rec.LocalFile)
		if err != nil {
			return err
		}
		remoteParent := path.Dir(rec.RemoteFile)
		if remoteParent == "." || remoteParent == "" {
			remoteParent = "/"
		}
		switch {
		case info.IsDir():
			total, err := sft.calcLocalDirSize(rec.LocalFile)
			if err != nil {
				return err
			}
			startErr = sft.startUploadDirectoryAsync(sessionID, rec.LocalFile, remoteParent, total, opts)
		case rec.Overwrite:
			_, startErr = sft.startUploadReplaceAsync(sessionID, rec.LocalFile, rec.RemoteFile, info.Size(), opts)
		default:
			_, startErr = sft.startUploadFileAsync(sessionID, rec.LocalFile, remoteParent, info.Size(), opts)
		}
	case transfer.TypeDownload:
		ftpClient, err := sft.getSftpClient(sessionID)
		if err != nil {
			return err
		}
		info, err := ftpClient.Stat(rec.RemoteFile)
		if err != nil {
			return err
		}
		if info.IsDir() {
			total, err := sft.calcRemoteDirSize(sessionID, rec.RemoteFile)
			if err != nil {
				return err
			}
			startErr = sft.startDownloadDirectoryAsync(sessionID, rec.LocalFile, rec.RemoteFile, total, opts)
		} else {
			_, startErr = sft.startDownloadFileAsync(sessionID, rec.LocalFile, rec.RemoteFile, info.Size(), opts)
		}
	default:
		return fmt.Errorf("unknown transfer type: %s", rec.TransferType)
	}
	if startErr != nil {
		return startErr
	}
	sft.notifyQueued(sessionID, 1)
	return nil
}

func (sft *Manager) calcLocalDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func (sft *Manager) calcRemoteDirSize(sessionID, remotePath string) (int64, error) {
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return 0, err
	}
	var size int64
	var walk func(string) error
	walk = func(p string) error {
		entries, err := ftpClient.ReadDir(p)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				if err := walk(joinRemotePath(p, entry.Name())); err != nil {
					return err
				}
			} else {
				size += entry.Size()
			}
		}
		return nil
	}
	err = walk(remotePath)
	return size, err
}
