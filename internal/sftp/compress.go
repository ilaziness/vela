package sftp

import (
	"context"
	"os"
	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/usererr"
)

// CompressRemotePaths builds a tar.gz archive at archivePath (remote) from
// srcPaths with one remote tar command on the server — the data never leaves
// it and the interactive terminal is not involved. Compression is fixed to
// tar; the file name only labels the result. On failure the partial archive
// is removed. Runs asynchronously under one upload-typed tracker so the
// transfer panel shows progress and the remote pane refreshes on completion.
// Requires remote command execution and a server-side tar: on SFTP-only
// servers the operation fails with the remote-exec-unavailable error instead
// of streaming a local compression.
func (sft *Manager) CompressRemotePaths(sessionID string, srcPaths []string, archivePath string) error {
	if sessionID == "" || len(srcPaths) == 0 || archivePath == "" {
		return usererr.Coded("sftp.invalid_compress", "Invalid compress parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if _, statErr := client.Stat(archivePath); statErr == nil {
		return usererr.Coded("sftp.compress_target_exists", "%s already exists", archivePath)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	// Built up front so argument problems surface synchronously.
	cmd, err := compressCommand(archivePath, srcPaths)
	if err != nil {
		return err
	}
	var total int64
	for _, src := range srcPaths {
		info, statErr := client.Stat(src)
		if statErr != nil {
			return usererr.Wrap(statErr, "sftp.remote_path", "Remote path %s: %s", src)
		}
		if info.IsDir() {
			dirTotal, err := sft.calcRemoteDirSize(sessionID, src)
			if err != nil {
				return usererr.Wrap(err, "sftp.remote_path", "Remote path %s: %s", src)
			}
			total += dirTotal
		} else {
			total += info.Size()
		}
	}
	names := make([]string, 0, len(srcPaths))
	for _, src := range srcPaths {
		names = append(names, path.Base(src))
	}
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, strings.Join(names, ", "), archivePath, total, transfer.NewOpts{RemoteOp: true})
	if err != nil {
		return err
	}
	go func() {
		// The command runs under the tracker context: cancellation closes the
		// remote session and stops the tar run.
		err := sft.compressRemoteTar(tracker.Context(), sessionID, cmd, archivePath)
		sft.stopTracker(tracker, err)
	}()
	sft.notifyQueued(sessionID, 1)
	return nil
}

// compressRemoteTar runs the remote tar command that builds the archive on
// the server. On failure (including cancellation via ctx) the partial archive
// is removed. The tracker stays indeterminate: the command reports no byte
// progress.
func (sft *Manager) compressRemoteTar(ctx context.Context, sessionID, cmd, archivePath string) error {
	_, err := sft.runRemoteCommand(ctx, sessionID, cmd, 0, ssh.DefaultExecMaxOut)
	if err != nil {
		// Cleanup runs even after cancellation: it only removes the partial
		// archive this call created, on its own session.
		cleanupCmd := "rm -f " + shellQuote(archivePath)
		if _, rmErr := sft.runRemoteCommand(context.Background(), sessionID, cleanupCmd, 0, ssh.DefaultExecMaxOut); rmErr != nil {
			sft.logger.Warn("compress: remove partial archive failed", zap.String("archive", archivePath), zap.Error(rmErr))
		}
		return err
	}
	return nil
}
