package sftp

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/sftp"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/usererr"
)

// archiveListTimeout bounds remote listing runs (a large .tar.gz listing
// decompresses the whole stream server-side).
const archiveListTimeout = 5 * time.Minute

// listTarEntries runs the remote listing command for a tar-family archive and
// returns normalized member entries. Any unsafe member name aborts the whole
// operation: the remote tar extracts everything at once and cannot skip a
// single bad entry the way the streaming zip path does. tar t prints one
// member name per line, directories with a trailing slash.
func (sft *Manager) listTarEntries(sessionID string, f archiveFormat, archivePath string) ([]archiveEntry, error) {
	out, err := sft.runRemoteCommand(context.Background(), sessionID, listTarCommand(f, archivePath), archiveListTimeout, archiveListMaxOut)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(out, "\n…[truncated]") {
		return nil, usererr.Coded("sftp.archive_list_too_large", "Too many entries in %s", archivePath)
	}
	entries := make([]archiveEntry, 0, 256)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		isDir := strings.HasSuffix(line, "/")
		name, unsafe := archiveEntrySafety(line)
		if unsafe {
			return nil, usererr.Coded("sftp.archive_unsafe_entries", "Archive %s contains unsafe entry paths", archivePath)
		}
		if name == "" {
			// The archive root member ("./") maps onto destDir itself.
			continue
		}
		entries = append(entries, archiveEntry{name: name, isDir: isDir})
	}
	return entries, nil
}

// archiveEntries resolves the normalized entry list of any supported archive
// for the conflict pre-check and extraction. cleanup must be called once the
// entries are done; it closes the remote handle the streaming zip path keeps
// open (other paths have nothing to close).
func (sft *Manager) archiveEntries(sessionID string, client *sftp.Client, archivePath string) ([]archiveEntry, func(), error) {
	format := detectArchiveFormat(archivePath)
	switch {
	case format == formatZip:
		arc, err := sft.openRemoteArchive(client, archivePath)
		if err != nil {
			return nil, nil, err
		}
		return arc.entries, arc.close, nil
	case isTarFamily(format):
		entries, err := sft.listTarEntries(sessionID, format, archivePath)
		if err != nil {
			return nil, nil, err
		}
		return entries, func() {}, nil
	case isSingleStream(format):
		name := singleStreamName(archivePath)
		if name == "" {
			return nil, nil, usererr.Coded("sftp.invalid_archive", "%s: not a valid archive", archivePath)
		}
		return []archiveEntry{{name: name}}, func() {}, nil
	default:
		return nil, nil, usererr.Coded("sftp.invalid_archive", "%s: unsupported archive format", archivePath)
	}
}

// extractTarRemote extracts a tar-family archive with one remote command on
// the server. stripped mirrors the flatten decision, so the archive's common
// root folder is dropped exactly like the streaming zip path does.
func (sft *Manager) extractTarRemote(sessionID string, f archiveFormat, archivePath, destDir string, stripped bool) error {
	strip := 0
	if stripped {
		strip = 1
	}
	_, err := sft.runRemoteCommand(context.Background(), sessionID, extractTarCommand(f, archivePath, destDir, strip), 0, ssh.DefaultExecMaxOut)
	return err
}

// extractSingleRemote decompresses one bare compressed stream (file.gz and
// friends) onto name below destDir — gunzip semantics, on the server.
func (sft *Manager) extractSingleRemote(sessionID string, f archiveFormat, archivePath, destDir, name string) error {
	target := joinRemotePath(destDir, name)
	_, err := sft.runRemoteCommand(context.Background(), sessionID, extractSingleCommand(f, archivePath, target), 0, ssh.DefaultExecMaxOut)
	return err
}
