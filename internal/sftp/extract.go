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
// no folder named after the archive or its root entry.
func flattenEntries(entries []archiveEntry) []archiveEntry {
	if len(entries) == 0 {
		return entries
	}
	firsts := make(map[string]struct{}, 1)
	for _, e := range entries {
		firsts[strings.SplitN(e.name, "/", 2)[0]] = struct{}{}
	}
	if len(firsts) != 1 {
		return entries
	}
	var root string
	for k := range firsts {
		root = k
	}
	// Only strip when the common root has children, so a lone "file.txt"
	// entry is never mangled.
	if len(entries) == 1 && !entries[0].isDir {
		return entries
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
	return out
}

// ListArchiveConflicts returns the flattened target paths under destDir that
// already exist, so the frontend can offer an overwrite confirm.
func (sft *Manager) ListArchiveConflicts(sessionID, archivePath, destDir string) ([]string, error) {
	if sessionID == "" || archivePath == "" || destDir == "" {
		return nil, usererr.Coded("sftp.invalid_extract", "Invalid extract parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return nil, err
	}
	arc, err := sft.openRemoteArchive(client, archivePath)
	if err != nil {
		return nil, err
	}
	defer arc.close()
	conflicts := make([]string, 0)
	seen := make(map[string]struct{}, len(arc.entries))
	for _, e := range flattenEntries(arc.entries) {
		if _, ok := seen[e.name]; ok {
			continue
		}
		seen[e.name] = struct{}{}
		if _, statErr := client.Stat(joinRemotePath(destDir, e.name)); statErr == nil {
			conflicts = append(conflicts, e.name)
		} else if !os.IsNotExist(statErr) {
			return nil, statErr
		}
	}
	return conflicts, nil
}

// ExtractRemoteArchive extracts a remote zip into destDir (current directory
// semantics: the archive's common root folder is stripped). Runs asynchronously
// under one upload-typed tracker; the remote pane refreshes on completion.
func (sft *Manager) ExtractRemoteArchive(sessionID string, archivePath, destDir string, overwrite bool) error {
	if sessionID == "" || archivePath == "" || destDir == "" {
		return usererr.Coded("sftp.invalid_extract", "Invalid extract parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	arc, err := sft.openRemoteArchive(client, archivePath)
	if err != nil {
		return err
	}
	flat := flattenEntries(arc.entries)
	if len(flat) == 0 {
		arc.close()
		return usererr.Coded("sftp.invalid_archive", "%s: no extractable entries", archivePath)
	}
	var total int64
	for _, e := range flat {
		if !e.isDir {
			total += e.size
		}
	}
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, archivePath, destDir, total, transfer.NewOpts{Overwrite: overwrite})
	if err != nil {
		arc.close()
		return err
	}
	go func() {
		err := sft.extractEntries(client, arc, flat, destDir, overwrite, tracker)
		arc.close()
		sft.stopTracker(tracker, err)
	}()
	sft.notifyQueued(sessionID, 1)
	return nil
}

// extractEntries writes every entry into destDir. Conflicts without overwrite
// abort the extraction (they are pre-checked via ListArchiveConflicts; this is
// the race backstop).
func (sft *Manager) extractEntries(client *sftp.Client, arc *openArchive, entries []archiveEntry, destDir string, overwrite bool, tracker *transfer.Tracker) error {
	for _, e := range entries {
		target := joinRemotePath(destDir, e.name)
		if st, statErr := client.Stat(target); statErr == nil {
			if !overwrite {
				return usererr.Coded("sftp.path_exists", "%s already exists", target)
			}
			if st.IsDir() != e.isDir {
				// Type conflict: replace regardless of which side is which.
				if err := client.RemoveAll(target); err != nil {
					return err
				}
			} else if !e.isDir {
				// zip entries cannot append; drop the stale file first.
				if err := client.Remove(target); err != nil {
					return err
				}
			}
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

// extractFile streams one zip entry straight onto a new remote file, wrapping
// the destination writes with the tracker for progress and cancellation.
func (sft *Manager) extractFile(client *sftp.Client, zf *zip.File, target string, tracker *transfer.Tracker) error {
	if err := client.MkdirAll(path.Dir(target)); err != nil {
		return err
	}
	if st, statErr := client.Stat(target); statErr == nil && st.IsDir() {
		return usererr.Coded("sftp.remote_is_dir", "Remote path %s is a directory", target)
	}
	src, err := zf.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := client.Create(target)
	if err != nil {
		return err
	}
	// Wrap the read side: sftp.File implements io.ReaderFrom, so a writer-side
	// wrapper would be bypassed by io.Copy.
	_, copyErr := io.Copy(out, &transfer.Reader{Reader: src, Tracker: tracker})
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
