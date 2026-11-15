package sftp

import (
	"io"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/usererr"
)

// PasteConflicts returns destination basenames under destDir that already exist
// for the given source paths. Mirrors UploadConflicts for remote-to-remote paste.
func (sft *Manager) PasteConflicts(sessionID, destDir string, srcPaths []string) ([]string, error) {
	if sessionID == "" || destDir == "" || len(srcPaths) == 0 {
		return nil, usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return nil, err
	}
	conflicts := make([]string, 0)
	seen := make(map[string]struct{}, len(srcPaths))
	for _, src := range srcPaths {
		name := path.Base(src)
		if name == "" || name == "." || name == "/" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		dest := joinRemotePath(destDir, name)
		if _, statErr := client.Stat(dest); statErr == nil {
			conflicts = append(conflicts, name)
		} else if !os.IsNotExist(statErr) {
			return nil, statErr
		}
	}
	return conflicts, nil
}

type pasteJob struct {
	src  string
	dst  string
	size int64
}

// pasteDestInvalid reports dst when a paste would drop an entry onto itself or
// inside its own subtree (dir into own subdir).
func pasteDestInvalid(src, destDir, destName string) (string, bool) {
	dst := joinRemotePath(destDir, destName)
	if dst == src || strings.HasPrefix(src, dst+"/") {
		return dst, true
	}
	return dst, false
}

// PasteRemotePaths copies srcPaths into destDir within one session, one tracker
// per top-level entry. destNames optionally renames each entry at the target
// (same length as srcPaths; used for same-directory duplicates); empty keeps
// the source basename. With cut, sources are deleted after their copy succeeds.
func (sft *Manager) PasteRemotePaths(sessionID string, srcPaths []string, destNames []string, destDir string, cut, overwrite bool) error {
	if sessionID == "" || destDir == "" || len(srcPaths) == 0 {
		return usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
	}
	if len(destNames) != 0 && len(destNames) != len(srcPaths) {
		return usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	jobs := make([]pasteJob, 0, len(srcPaths))
	for i, src := range srcPaths {
		name := path.Base(src)
		if i < len(destNames) && destNames[i] != "" {
			name = destNames[i]
		}
		dst, invalid := pasteDestInvalid(src, destDir, name)
		if invalid {
			return usererr.Coded("sftp.paste_into_self", "Cannot paste %s into itself", src)
		}
		// Synchronous conflict guard: without overwrite an existing destination
		// fails the call up front (frontend pre-checks via PasteConflicts; this
		// catches the rest). preparePasteDest remains the race backstop.
		if !overwrite {
			if _, statErr := client.Stat(dst); statErr == nil {
				return usererr.Coded("sftp.path_exists", "%s already exists", dst)
			} else if !os.IsNotExist(statErr) {
				return statErr
			}
		}
		info, statErr := client.Stat(src)
		if statErr != nil {
			return usererr.Wrap(statErr, "sftp.remote_path", "Remote path %s: %s", src)
		}
		total := info.Size()
		if info.IsDir() {
			total, err = sft.calcRemoteDirSize(sessionID, src)
			if err != nil {
				return usererr.Wrap(err, "sftp.remote_path", "Remote path %s: %s", src)
			}
		}
		jobs = append(jobs, pasteJob{src: src, dst: dst, size: total})
	}
	queued := 0
	for _, job := range jobs {
		if err := sft.startPasteAsync(sessionID, job, cut, overwrite); err != nil {
			sft.logger.Warn("enqueue paste failed", zap.String("src", job.src), zap.Error(err))
			continue
		}
		queued++
	}
	sft.notifyQueued(sessionID, queued)
	return nil
}

func (sft *Manager) startPasteAsync(sessionID string, job pasteJob, cut, overwrite bool) error {
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, job.src, job.dst, job.size, transfer.NewOpts{Overwrite: overwrite})
	if err != nil {
		return err
	}
	go func() {
		err := sft.pasteTree(sessionID, job, overwrite, tracker)
		if err == nil && cut {
			// Remove the source only when its copy fully succeeded.
			if rmErr := sft.DeleteFile(sessionID, job.src); rmErr != nil {
				sft.logger.Error("paste cut: delete source failed", zap.String("src", job.src), zap.Error(rmErr))
				err = rmErr
			}
		}
		sft.stopTracker(tracker, err)
	}()
	return nil
}

// pasteTree copies one top-level entry (file or directory tree) to dst.
func (sft *Manager) pasteTree(sessionID string, job pasteJob, overwrite bool, tracker *transfer.Tracker) error {
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	info, err := client.Stat(job.src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := sft.preparePasteDest(client, job.dst, overwrite); err != nil {
			return err
		}
		if err := client.MkdirAll(job.dst); err != nil {
			return err
		}
		return sft.copyRemoteDir(client, job.src, job.dst, tracker)
	}
	if err := sft.preparePasteDest(client, job.dst, overwrite); err != nil {
		return err
	}
	return sft.copyRemoteFile(client, job.src, job.dst, info.Mode().Perm(), tracker)
}

// preparePasteDest removes an existing destination when overwriting; without
// overwrite an existing entry is an error (frontend checks conflicts up front,
// this guards races between check and copy).
func (sft *Manager) preparePasteDest(client *sftp.Client, dst string, overwrite bool) error {
	st, statErr := client.Stat(dst)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return statErr
	}
	if !overwrite {
		return usererr.Coded("sftp.path_exists", "%s already exists", dst)
	}
	if st.IsDir() {
		return client.RemoveAll(dst)
	}
	return client.Remove(dst)
}

// copyRemoteDir copies srcDir's entries into dstDir, which must already exist.
func (sft *Manager) copyRemoteDir(client *sftp.Client, srcDir, dstDir string, tracker *transfer.Tracker) error {
	entries, err := client.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := joinRemotePath(srcDir, entry.Name())
		dst := joinRemotePath(dstDir, entry.Name())
		if entry.IsDir() {
			if err := client.MkdirAll(dst); err != nil {
				return err
			}
			if err := client.Chmod(dst, entry.Mode().Perm()); err != nil {
				sft.logger.Warn("paste chmod dir failed", zap.String("dst", dst), zap.Error(err))
			}
			if err := sft.copyRemoteDir(client, src, dst, tracker); err != nil {
				return err
			}
		} else {
			if err := sft.copyRemoteFile(client, src, dst, entry.Mode().Perm(), tracker); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyRemoteFile streams one file across the same SFTP session; the tracker
// wraps the read side for progress and user cancellation.
func (sft *Manager) copyRemoteFile(client *sftp.Client, src, dst string, perm os.FileMode, tracker *transfer.Tracker) error {
	in, err := client.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := client.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, &transfer.Reader{Reader: in, Tracker: tracker})
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if chErr := client.Chmod(dst, perm); chErr != nil {
		sft.logger.Warn("paste chmod file failed", zap.String("dst", dst), zap.Error(chErr))
	}
	return closeErr
}
