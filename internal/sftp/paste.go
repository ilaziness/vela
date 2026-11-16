package sftp

import (
	"errors"
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
		if _, statErr := client.Lstat(dest); statErr == nil {
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

// pasteDestInvalid reports dst when a paste would drop an entry onto itself,
// inside its own subtree (dir into own subdir), or copy a parent entry onto one
// of its own descendants (dst under src — the walk would chase the output).
// Both paths are canonicalized lexically first (".", repeated and trailing
// separators) so equivalent spellings such as /tmp/./a and /tmp/a are
// recognized as the same entry — a cut of one onto the other must never reach
// the copy-and-delete fallback.
func pasteDestInvalid(src, destDir, destName string) (string, bool) {
	dst := joinRemotePath(destDir, destName)
	s := path.Clean(src)
	d := path.Clean(dst)
	if d == s || strings.HasPrefix(s, d+"/") || strings.HasPrefix(d, s+"/") {
		return dst, true
	}
	return dst, false
}

// PasteRemotePaths copies srcPaths into destDir within one session, one tracker
// per top-level entry. destNames optionally renames each entry at the target
// (same length as srcPaths; used for same-directory duplicates); empty keeps
// the source basename. With cut, sources are deleted after their copy succeeds.
// Each entry is pasted by one silent remote cp/mv command whenever the server
// allows command execution — the data stays on the server — and falls back to
// a streaming copy through this SFTP connection otherwise (never staged as
// local files).
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
		if src == "" {
			return usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
		}
		// Canonical spelling: "a/./b", "a//b" and "a/b/" name the same remote
		// entry; every guard and the cut cleanup compare canonical paths.
		src = path.Clean(src)
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
		// catches the rest). pasteTree re-checks before writing.
		if !overwrite {
			if _, statErr := client.Lstat(dst); statErr == nil {
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
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, job.src, job.dst, job.size, transfer.NewOpts{Overwrite: overwrite, RemoteOp: true})
	if err != nil {
		return err
	}
	go func() {
		// Prefer one silent remote cp/mv command: the data stays on the
		// server and the interactive terminal is never involved. The
		// streaming SFTP copy below is the fallback when remote command
		// execution is unavailable (or its tools reject the invocation).
		// The command runs under the tracker context: cancellation stops the
		// remote work and is never replayed over SFTP.
		err := sft.pasteRemoteCommand(tracker.Context(), sessionID, job, cut, overwrite)
		if errors.Is(err, errPasteCommandFallback) {
			sft.logger.Warn("paste remote command unavailable; falling back to SFTP copy",
				zap.String("src", job.src), zap.String("dst", job.dst), zap.Error(err))
			err = sft.pasteTree(sessionID, job, overwrite, tracker)
			if err == nil && cut && !tracker.WasCancelled() {
				// Remove the source only when its copy fully succeeded and
				// the user did not cancel; directories are removed with their
				// whole tree and symlinks are unlinked, never followed.
				rmErr := sft.removePasteSource(sessionID, job.src)
				if rmErr != nil {
					sft.logger.Error("paste cut: delete source failed", zap.String("src", job.src), zap.Error(rmErr))
					err = rmErr
				}
			}
		}
		sft.stopTracker(tracker, err)
	}()
	return nil
}

// pasteTree copies one top-level entry (file or directory tree) to dst.
// Overwrite semantics match the frontend conflict prompt: same-named
// directories merge (destination-only entries are kept) and same-named files
// are replaced. Every replacement is staged — the new content is fully copied
// to a hidden sibling path first and only moved onto dst afterwards — so a
// failed or cancelled copy never destroys an overwritten original.
func (sft *Manager) pasteTree(sessionID string, job pasteJob, overwrite bool, tracker *transfer.Tracker) error {
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	info, err := client.Stat(job.src)
	if err != nil {
		return err
	}
	dstInfo, dstErr := client.Lstat(job.dst)
	dstExists := dstErr == nil
	if dstErr != nil && !os.IsNotExist(dstErr) {
		return dstErr
	}
	if dstExists && !overwrite {
		return usererr.Coded("sftp.path_exists", "%s already exists", job.dst)
	}
	if !info.IsDir() {
		return sft.copyRemoteFile(client, job.src, job.dst, info.Mode().Perm(), tracker)
	}
	if dstExists && isRealDir(dstInfo) {
		// Directory conflict: merge into the existing destination directory,
		// keeping its mode and its destination-only entries.
		return sft.copyRemoteDir(client, job.src, job.dst, tracker)
	}
	// Missing destination or type conflict (file / symlink at dst): build the
	// tree at a staging sibling and move it into place only when complete.
	return sft.copyDirReplace(client, job.src, job.dst, tracker)
}

// isRealDir reports whether info describes a directory that is not a symlink.
func isRealDir(info os.FileInfo) bool {
	return info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

// removeRemoteEntry removes p; a symlink at p is unlinked, never followed.
func (sft *Manager) removeRemoteEntry(client *sftp.Client, p string) error {
	if st, err := client.Lstat(p); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return client.Remove(p)
	}
	return client.RemoveAll(p)
}

// removePasteSource deletes a cut source after its copy succeeded. Files and
// symlinks are unlinked; directory trees are removed recursively.
func (sft *Manager) removePasteSource(sessionID, src string) error {
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	return sft.removeRemoteEntry(client, src)
}

// copyDirReplace copies srcDir's whole tree to a staging sibling of dst and
// moves it onto dst afterwards. A conflicting file or symlink at dst is
// removed only after the full tree was copied.
func (sft *Manager) copyDirReplace(client *sftp.Client, srcDir, dst string, tracker *transfer.Tracker) error {
	staging := stagingRemotePath(dst)
	if staging == dst {
		return usererr.Coded("sftp.staging_equals_remote", "Edit staging path equals remote path")
	}
	// Defensive: anything sitting at this operation's unique staging name can
	// only be foreign (e.g. a pre-planted symlink). Unlink it without
	// following before building, so MkdirAll below cannot write through it.
	if err := sft.removeRemoteEntry(client, staging); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := sft.copyRemoteDir(client, srcDir, staging, tracker); err != nil {
		// Recursive cleanup: the partial staging tree must not survive.
		sft.removeStagingTree(client, staging)
		return err
	}
	if _, statErr := client.Lstat(dst); statErr == nil {
		if err := sft.removeRemoteEntry(client, dst); err != nil {
			sft.removeStagingTree(client, staging)
			return err
		}
	} else if !os.IsNotExist(statErr) {
		sft.removeStagingTree(client, staging)
		return statErr
	}
	if err := client.Rename(staging, dst); err != nil {
		sft.logger.Error("paste tree rename failed; copy remains at staging",
			zap.String("staging", staging), zap.String("dst", dst), zap.Error(err))
		return usererr.Wrap(err, "sftp.rename_failed",
			"Failed to rename %s to %s; new contents remain at %s: %s", staging, dst, staging)
	}
	return nil
}

// copyRemoteDir copies srcDir's entries onto dstDir. A missing dstDir is
// created with the source directory's permission bits; an existing real
// directory is merged — its mode and its destination-only entries are kept,
// and same-named files are replaced via staged copies.
func (sft *Manager) copyRemoteDir(client *sftp.Client, srcDir, dstDir string, tracker *transfer.Tracker) error {
	if st, statErr := client.Lstat(dstDir); statErr == nil {
		if !isRealDir(st) {
			return usererr.Coded("sftp.path_exists", "%s already exists", dstDir)
		}
	} else if os.IsNotExist(statErr) {
		if err := client.MkdirAll(dstDir); err != nil {
			return err
		}
		// A read-only source mode (e.g. 0555) must not be applied before the
		// children are copied — the fresh directory needs owner write
		// permission while it is being populated. The source mode is applied
		// after the copy completed. Pre-existing merge destinations keep
		// their own mode (nothing is chmod'ed for them).
		defer func() {
			if info, statErr := client.Stat(srcDir); statErr == nil {
				if err := client.Chmod(dstDir, info.Mode().Perm()); err != nil {
					sft.logger.Warn("paste chmod dir failed", zap.String("dst", dstDir), zap.Error(err))
				}
			}
		}()
	} else {
		return statErr
	}
	entries, err := client.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := joinRemotePath(srcDir, entry.Name())
		dst := joinRemotePath(dstDir, entry.Name())
		if entry.IsDir() {
			if lst, lstErr := client.Lstat(dst); lstErr == nil && isRealDir(lst) {
				// Merge into the existing real directory, keep its mode.
				if err := sft.copyRemoteDir(client, src, dst, tracker); err != nil {
					return err
				}
				continue
			}
			if err := sft.copyDirReplace(client, src, dst, tracker); err != nil {
				return err
			}
			continue
		}
		if err := sft.copyRemoteFile(client, src, dst, entry.Mode().Perm(), tracker); err != nil {
			return err
		}
	}
	return nil
}

// copyRemoteFile streams one file across the same SFTP session; the tracker
// wraps the read side for progress and user cancellation. The content is
// staged at a hidden sibling path and only moved onto dst after the full copy
// succeeded, so a failed copy never destroys an overwritten original.
func (sft *Manager) copyRemoteFile(client *sftp.Client, src, dst string, perm os.FileMode, tracker *transfer.Tracker) error {
	in, err := client.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	staging := stagingRemotePath(dst)
	if staging == dst {
		return usererr.Coded("sftp.staging_equals_remote", "Edit staging path equals remote path")
	}
	// Exclusive create: never follow a symlink pre-planted at the staging name.
	out, err := client.OpenFile(staging, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, &transfer.Reader{Reader: in, Tracker: tracker})
	closeErr := out.Close()
	if copyErr != nil {
		sft.removeStaging(client, staging)
		return copyErr
	}
	if closeErr != nil {
		sft.removeStaging(client, staging)
		return closeErr
	}
	if chErr := client.Chmod(staging, perm); chErr != nil {
		sft.logger.Warn("paste chmod file failed", zap.String("dst", dst), zap.Error(chErr))
	}
	if st, lstErr := client.Lstat(dst); lstErr == nil && isRealDir(st) {
		// File-over-directory type conflict: the existing tree is removed
		// only now, with the replacement content complete at staging.
		if rmErr := sft.removeRemoteEntry(client, dst); rmErr != nil {
			sft.removeStaging(client, staging)
			return rmErr
		}
		if err := client.Rename(staging, dst); err != nil {
			sft.logger.Error("paste rename failed after removing original",
				zap.String("staging", staging), zap.String("dst", dst), zap.Error(err))
			return usererr.Wrap(err, "sftp.rename_original_removed",
				"Original %s was removed but renaming %s into place failed; new contents remain at %s: %s",
				dst, staging, staging)
		}
		return nil
	}
	return sft.replaceStagedFile(client, staging, dst)
}
