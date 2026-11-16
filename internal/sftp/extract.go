package sftp

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/usererr"
)

// archiveEntry is a normalized, flattened zip entry.
type archiveEntry struct {
	name  string // clean relative path under destDir, no trailing slash
	isDir bool
	size  int64
	file  *zip.File
}

// openArchive wraps a remote zip held open for entry streaming.
type openArchive struct {
	file    *sftp.File
	entries []archiveEntry
}

func (a *openArchive) close() {
	if err := a.file.Close(); err != nil {
		// logged by callers via manager logger when relevant
		_ = err
	}
}

// normalizeZipName validates one entry name and strips backslashes.
// Rejects absolute paths and ".." segments (zip-slip); returns "" for the
// archive root itself or anything that collapses to an empty path.
func normalizeZipName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	if len(name) >= 2 && name[1] == ':' { // windows drive letter
		return ""
	}
	parts := strings.Split(name, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return ""
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

// openRemoteArchive opens the remote zip and returns normalized entries.
func (sft *Manager) openRemoteArchive(client *sftp.Client, archivePath string) (*openArchive, error) {
	st, err := client.Stat(archivePath)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, usererr.Coded("sftp.invalid_archive", "%s: not a file", archivePath)
	}
	f, err := client.Open(archivePath)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		if closeErr := f.Close(); closeErr != nil {
			sft.logger.Warn("close archive failed", zap.String("archive", archivePath), zap.Error(closeErr))
		}
		return nil, usererr.Coded("sftp.invalid_archive", "%s: %s", archivePath, err.Error())
	}
	entries := make([]archiveEntry, 0, len(zr.File))
	for _, zf := range zr.File {
		clean := normalizeZipName(zf.Name)
		if clean == "" || clean == "." {
			continue
		}
		entries = append(entries, archiveEntry{
			name:  clean,
			isDir: zf.FileInfo().IsDir(),
			size:  int64(zf.UncompressedSize64),
			file:  zf,
		})
	}
	return &openArchive{file: f, entries: entries}, nil
}

// flattenEntries strips the single common top-level directory shared by all
// entries, so "src/a.txt, src/sub/b.txt" extracts as "a.txt, sub/b.txt" —
// no folder named after the archive or its root entry. stripped reports
// whether that root was actually dropped (the remote tar path turns it into
// --strip-components=1).
func flattenEntries(entries []archiveEntry) (flattened []archiveEntry, stripped bool) {
	if len(entries) == 0 {
		return entries, false
	}
	firsts := make(map[string]struct{}, 1)
	for _, e := range entries {
		firsts[strings.SplitN(e.name, "/", 2)[0]] = struct{}{}
	}
	if len(firsts) != 1 {
		return entries, false
	}
	var root string
	for k := range firsts {
		root = k
	}
	// Only strip when the common root has children, so a lone "file.txt"
	// entry is never mangled.
	if len(entries) == 1 && !entries[0].isDir {
		return entries, false
	}
	out := make([]archiveEntry, 0, len(entries))
	for _, e := range entries {
		if e.name == root {
			// The root dir entry itself maps to destDir; nothing to create.
			continue
		}
		trimmed := strings.TrimPrefix(e.name, root+"/")
		if trimmed == "" || trimmed == "." {
			continue
		}
		out = append(out, archiveEntry{name: trimmed, isDir: e.isDir, size: e.size, file: e.file})
	}
	return out, true
}

// ListArchiveConflicts returns the flattened target paths under destDir that
// already exist, so the frontend can offer an overwrite confirm. Zip archives
// are listed by streaming through SFTP; tar-family archives with a remote
// tar t command on the server.
func (sft *Manager) ListArchiveConflicts(sessionID, archivePath, destDir string) ([]string, error) {
	if sessionID == "" || archivePath == "" || destDir == "" {
		return nil, usererr.Coded("sftp.invalid_extract", "Invalid extract parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return nil, err
	}
	entries, cleanup, err := sft.archiveEntries(sessionID, client, archivePath)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	flat, _ := flattenEntries(entries)
	return sft.conflictList(client, flat, destDir)
}

// conflictList returns the flattened target paths under destDir that already
// exist, counting every name once.
func (sft *Manager) conflictList(client *sftp.Client, flat []archiveEntry, destDir string) ([]string, error) {
	conflicts := make([]string, 0)
	seen := make(map[string]struct{}, len(flat))
	for _, e := range flat {
		if _, ok := seen[e.name]; ok {
			continue
		}
		seen[e.name] = struct{}{}
		if _, statErr := client.Lstat(joinRemotePath(destDir, e.name)); statErr == nil {
			conflicts = append(conflicts, e.name)
		} else if !os.IsNotExist(statErr) {
			return nil, statErr
		}
	}
	return conflicts, nil
}

// ExtractRemoteArchive extracts a remote archive into destDir (current
// directory semantics: the archive's common root folder is stripped). Runs
// asynchronously under one upload-typed tracker; the remote pane refreshes on
// completion. Zip archives stream through SFTP with staged writes; tar-family
// archives and bare compressed streams are extracted by a remote command on
// the server — the data never leaves it and the interactive terminal is not
// involved.
func (sft *Manager) ExtractRemoteArchive(sessionID string, archivePath, destDir string, overwrite bool) error {
	if sessionID == "" || archivePath == "" || destDir == "" {
		return usererr.Coded("sftp.invalid_extract", "Invalid extract parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	format := detectArchiveFormat(archivePath)
	entries, cleanup, err := sft.archiveEntries(sessionID, client, archivePath)
	if err != nil {
		return err
	}
	flat, stripped := flattenEntries(entries)
	if len(flat) == 0 {
		cleanup()
		return usererr.Coded("sftp.invalid_archive", "%s: no extractable entries", archivePath)
	}
	if format != formatZip {
		// The remote tar/gzip write into destDir via -C / redirection, which
		// does not create it; make sure it exists first.
		if err := client.MkdirAll(destDir); err != nil {
			cleanup()
			return err
		}
	}
	var total int64
	for _, e := range flat {
		if !e.isDir {
			total += e.size
		}
	}
	if format != formatZip {
		// tar t lists names only, so the uncompressed size is unknown;
		// the tracker runs indeterminate until the command completes.
		total = 0
	}
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, archivePath, destDir, total, transfer.NewOpts{Overwrite: overwrite, RemoteOp: true})
	if err != nil {
		cleanup()
		return err
	}
	go func() {
		var runErr error
		switch {
		case format == formatZip:
			runErr = sft.extractEntries(client, flat, destDir, overwrite, tracker)
		case isTarFamily(format):
			runErr = sft.extractTarRemote(sessionID, format, archivePath, destDir, stripped)
		case isSingleStream(format):
			runErr = sft.extractSingleRemote(sessionID, format, archivePath, destDir, flat[0].name)
		}
		cleanup()
		sft.stopTracker(tracker, runErr)
	}()
	sft.notifyQueued(sessionID, 1)
	return nil
}

// extractParentsSafe reports whether every existing directory component of
// rel below destDir is a real directory. A symlink anywhere in the parent
// chain would redirect writes outside destDir; extraction refuses such paths.
func extractParentsSafe(client *sftp.Client, destDir, rel string) bool {
	dir := path.Dir(rel)
	if dir == "." || dir == "/" || dir == "" {
		return true
	}
	cur := destDir
	for _, part := range strings.Split(dir, "/") {
		if part == "" || part == "." {
			continue
		}
		cur = joinRemotePath(cur, part)
		lst, err := client.Lstat(cur)
		if err != nil {
			// Missing components are created as real directories and deeper
			// paths cannot exist yet.
			return os.IsNotExist(err)
		}
		if !isRealDir(lst) {
			return false
		}
	}
	return true
}

// extractEntries writes every entry into destDir. Conflicts without overwrite
// abort the extraction (they are pre-checked via ListArchiveConflicts; this is
// the race backstop). File contents are staged and swapped onto the target
// only after a complete write, so a failed extraction never destroys an
// overwritten original; symlinked parents and symlink targets are replaced,
// never followed.
func (sft *Manager) extractEntries(client *sftp.Client, entries []archiveEntry, destDir string, overwrite bool, tracker *transfer.Tracker) error {
	for _, e := range entries {
		target := joinRemotePath(destDir, e.name)
		if !extractParentsSafe(client, destDir, e.name) {
			return usererr.Coded("sftp.extract_symlink_path", "%s: entry path crosses a symbolic link", e.name)
		}
		if st, statErr := client.Lstat(target); statErr == nil {
			if !overwrite {
				return usererr.Coded("sftp.path_exists", "%s already exists", target)
			}
			if e.isDir && !isRealDir(st) {
				// A symlink at the target is unlinked, never merged through;
				// a file is replaced by a real directory below.
				if err := client.Remove(target); err != nil {
					return err
				}
			}
			// File entries: extractFile stages the content and replaces the
			// original only after the full write succeeded.
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		if e.isDir {
			if err := client.MkdirAll(target); err != nil {
				return err
			}
			continue
		}
		if err := sft.extractFile(client, e.file, target, tracker); err != nil {
			return err
		}
	}
	return nil
}

// extractFile streams one zip entry to a staging sibling and swaps it onto
// target only after the full write succeeded, so a failed extraction never
// destroys an overwritten original.
func (sft *Manager) extractFile(client *sftp.Client, zf *zip.File, target string, tracker *transfer.Tracker) error {
	if err := client.MkdirAll(path.Dir(target)); err != nil {
		return err
	}
	src, err := zf.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	staging := editStagingRemotePath(target)
	if staging == target {
		return usererr.Coded("sftp.staging_equals_remote", "Edit staging path equals remote path")
	}
	out, err := client.Create(staging)
	if err != nil {
		return err
	}
	// Wrap the read side: sftp.File implements io.ReaderFrom, so a writer-side
	// wrapper would be bypassed by io.Copy.
	_, copyErr := io.Copy(out, &transfer.Reader{Reader: src, Tracker: tracker})
	closeErr := out.Close()
	if copyErr != nil {
		sft.removeStaging(client, staging)
		return copyErr
	}
	if closeErr != nil {
		sft.removeStaging(client, staging)
		return closeErr
	}
	if st, statErr := client.Lstat(target); statErr == nil && isRealDir(st) {
		// File-over-directory type conflict: the existing tree is removed
		// only now, with the replacement content complete at staging.
		if err := sft.removeRemoteEntry(client, target); err != nil {
			sft.removeStaging(client, staging)
			return err
		}
		if err := client.Rename(staging, target); err != nil {
			sft.logger.Error("extract rename failed after removing original",
				zap.String("staging", staging), zap.String("target", target), zap.Error(err))
			return usererr.Wrap(err, "sftp.rename_original_removed",
				"Original %s was removed but renaming %s into place failed; new contents remain at %s: %s",
				target, staging, staging)
		}
		return nil
	}
	return sft.replaceStagedFile(client, staging, target)
}
