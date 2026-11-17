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

// pasteDestInvalid reports whether a paste would drop an entry onto itself,
// inside its own subtree (dir into own subdir), or copy a parent entry onto one
// of its own descendants (dst under src — the walk would chase the output).
// src and dst must already be server-canonical paths (see canonicalRemotePath):
// POSIX resolves ".." after following symlinks, so lexical cleaning alone can
// name a different file — the identity and ancestry checks must use the paths
// the server actually operates on. The lexical Clean below only normalizes
// spelling noise (".", repeated separators) for robustness. The dst-under-src
// check is limited to real directory sources: recreating a symlink leaf under
// its target's tree is safe (the link itself is moved/copied, never its
// resolved contents).
func pasteDestInvalid(src, dst string, srcIsRealDir bool) bool {
	s := path.Clean(src)
	d := path.Clean(dst)
	if d == s || strings.HasPrefix(s, d+"/") {
		return true
	}
	return srcIsRealDir && strings.HasPrefix(d, s+"/")
}

// splitRemoteEntry splits p into the parent spelling and the verbatim
// basename WITHOUT lexical cleaning: path.Dir cleans the result, collapsing
// ".." before the server has resolved it (POSIX follows symlinks first and
// then applies ".."), which would name a different directory than the server
// traverses.
func splitRemoteEntry(p string) (dir, base string) {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return ".", p
	}
	if i == 0 {
		return "/", p[1:]
	}
	return p[:i], p[i+1:]
}

// canonicalRemotePath resolves p to the server-canonical path of the entry it
// names. The server follows symlinks and resolves ".." after them (POSIX),
// which lexical cleaning cannot do. Falls back to canonicalizing the parent
// and re-attaching the base when the server refuses to resolve the lookup
// (some servers reject a missing leaf), and to lexical cleaning as the last
// resort.
func (sft *Manager) canonicalRemotePath(client *sftp.Client, p string) string {
	if rp, err := client.RealPath(p); err == nil && rp != "" {
		return path.Clean(rp)
	}
	dir, base := splitRemoteEntry(p)
	if rp, err := client.RealPath(dir); err == nil && rp != "" {
		return joinRemotePath(rp, base)
	}
	return path.Clean(p)
}

// canonicalRemoteEntry identifies a remote ENTRY by the server-canonical path
// of its parent directory plus its verbatim basename. The final component is
// never followed: a symlink entry is identified by its own directory slot, not
// by its target (which resolving the whole path would report — and which would
// both misname the entry at the destination and hide its true ancestry). Only
// the parent spelling is resolved, where the server follows symlinks and
// applies ".." afterwards (POSIX) — lexical cleaning cannot do that. A spelling
// whose base is "." or ".." names a directory rather than an entry and falls
// back to the full canonical path (real-directory semantics).
func (sft *Manager) canonicalRemoteEntry(client *sftp.Client, p string) (entry, name string) {
	_, base := splitRemoteEntry(p)
	if base == "." || base == ".." {
		full := sft.canonicalRemotePath(client, p)
		return full, path.Base(full)
	}
	dir, base := splitRemoteEntry(p)
	return joinRemotePath(sft.canonicalRemotePath(client, dir), base), base
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
	// The destination directory is resolved once; operations and ancestry
	// checks work on the server-canonical spelling and never on a lexical
	// collapse of symlink-sensitive ".." components.
	destCanon := sft.canonicalRemotePath(client, destDir)
	for i, src := range srcPaths {
		// Only trailing separators are stripped so entry semantics hold.
		src = strings.TrimRight(src, "/")
		if src == "" {
			return usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
		}
		info, statErr := client.Lstat(src)
		if statErr != nil {
			return usererr.Wrap(statErr, "sftp.remote_path", "Remote path %s: %s", src)
		}
		// Entry identity: canonical parent directory plus verbatim basename
		// (see canonicalRemoteEntry). Both the default destination name and
		// the identity/ancestry checks use it: a symlink gets its own name at
		// the destination (not its target's), and a cut of one spelling onto
		// another naming the same entry never reaches the copy-and-delete
		// path. Recursive work uses this canonical spelling too, so joining
		// child names cannot collapse ".." into a different tree.
		srcEntry, defName := sft.canonicalRemoteEntry(client, src)
		name := defName
		if i < len(destNames) && destNames[i] != "" {
			name = destNames[i]
		}
		dst := joinRemotePath(destCanon, name)
		if pasteDestInvalid(srcEntry, dst, isRealDir(info)) {
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
		total := info.Size()
		if isRealDir(info) {
			total, err = sft.calcRemoteDirSize(sessionID, srcEntry)
			if err != nil {
				return usererr.Wrap(err, "sftp.remote_path", "Remote path %s: %s", src)
			}
		}
		jobs = append(jobs, pasteJob{src: srcEntry, dst: dst, size: total})
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
	// Lstat: a symlink source is a leaf entry here (see the dispatch below),
	// not the tree or file its target happens to be.
	info, err := client.Lstat(job.src)
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
	if info.Mode()&os.ModeSymlink != 0 {
		// Recreate the link itself (verbatim link text) — never the content
		// it resolves to: directory links and dangling links survive as
		// links, matching the cp -a / mv command path.
		return sft.copyRemoteLink(client, job.src, job.dst)
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
		srcInfo, srcStatErr := client.Lstat(srcDir)
		if srcStatErr == nil {
			// Restrict the fresh directory while it is being populated: a
			// private source mode (e.g. 0700) must not sit at the server
			// default (0755 under umask 022) during the copy. The owner keeps
			// write access so children can be created; group/other never see
			// more than the source allows.
			if err := client.Chmod(dstDir, dirPopulateMode(srcInfo.Mode().Perm())); err != nil {
				sft.logger.Warn("paste chmod dir failed", zap.String("dst", dstDir), zap.Error(err))
			}
		}
		// A read-only source mode (e.g. 0555) must not be applied before the
		// children are copied — the fresh directory needs owner write
		// permission while it is being populated. The exact source mode is
		// applied after the copy completed. Pre-existing merge destinations
		// keep their own mode (nothing is chmod'ed for them).
		defer func() {
			if srcStatErr == nil {
				if err := client.Chmod(dstDir, srcInfo.Mode().Perm()); err != nil {
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
		if entry.Mode()&os.ModeSymlink != 0 {
			// File, directory and dangling symlinks are recreated as links.
			// copyRemoteFile would open and follow their targets instead.
			if err := sft.copyRemoteLink(client, src, dst); err != nil {
				return err
			}
			continue
		}
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

// copyRemoteLink recreates one symlink entry at dst without following it: the
// link text is copied verbatim (ReadLink -> Symlink), so relative links,
// directory links and dangling links survive a merge unchanged — unlike
// copyRemoteFile, which would open the link's target. The link is staged at a
// unique sibling and moved onto dst afterwards; a directory or symlink at dst
// is removed (a symlink is unlinked, never followed) only after the
// replacement link is complete, so no failed step destroys an overwritten
// original and nothing is ever written through a link.
func (sft *Manager) copyRemoteLink(client *sftp.Client, src, dst string) error {
	target, err := client.ReadLink(src)
	if err != nil {
		return err
	}
	staging := stagingRemotePath(dst)
	if staging == dst {
		return usererr.Coded("sftp.staging_equals_remote", "Edit staging path equals remote path")
	}
	// Symlink creation refuses a pre-existing staging name (EEXIST) and never
	// follows a link planted at it.
	if err := client.Symlink(target, staging); err != nil {
		return err
	}
	if st, lstErr := client.Lstat(dst); lstErr == nil && !st.Mode().IsRegular() {
		// A directory tree or a symlink at dst: never move into a symlinked
		// directory and never write through a link. The old entry is removed
		// only now, with the replacement link complete at staging.
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

// dirPopulateMode returns the mode used while a fresh destination directory is
// being populated: the source permission bits plus owner rwx. Group/other
// never see more than the source allows (a private 0700 source stays private
// during the copy) and the owner keeps write access so children can be created
// even for read-only sources such as 0555.
func dirPopulateMode(perm os.FileMode) os.FileMode {
	return perm | 0o700
}
