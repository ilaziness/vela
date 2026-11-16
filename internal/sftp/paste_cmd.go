package sftp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/usererr"
)

// errPasteCommandFallback marks that the remote copy/move command path could
// not be used; the caller should replay the paste through the streaming SFTP
// implementation so the operation still succeeds wherever SFTP works. It is
// also returned for operations that deliberately run on the staged SFTP path
// (directory merges build every replacement in its own staging entry and keep
// destination directory modes untouched).
var errPasteCommandFallback = errors.New("paste: remote command path unavailable")

// pasteCommand builds one remote shell command that performs a whole copy or
// cut of src onto dst entirely on the server (cp/mv on a silent,
// non-interactive SSH session — the data never crosses the client and the
// interactive terminal is untouched). Overwrite semantics mirror the SFTP
// fallback in pasteTree:
//   - a real directory pasted onto a real directory merges and therefore
//     returns errPasteCommandFallback: the staged SFTP merge never writes
//     replacement bytes directly over originals and never broadens existing
//     destination directory modes;
//   - every non-merge copy writes to the unique hidden staging sibling of dst
//     first and only then moves onto dst, so a failed copy never destroys an
//     overwritten original;
//   - cut moves with mv and never copies in place;
//   - mv(1) would move a source *into* a symlinked destination directory, so
//     a symlink at dst is removed before the final move (the SFTP path
//     replaces such a link too).
//
// srcIsDir reports whether the source is a directory tree; dstInfo is the
// Lstat view of the destination and nil means it is missing. staging is the
// unique staging sibling of dst (see stagingRemotePath). The second return
// value is a best-effort cleanup command removing the staging entry after a
// failed copy (empty when nothing may be removed — cut keeps the only
// remaining data at staging until the move completes).
func pasteCommand(src, dst string, srcIsDir, cut bool, dstInfo os.FileInfo, staging string) (cmd, cleanup string, err error) {
	if src == "" || dst == "" {
		return "", "", usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
	}
	if staging == "" || staging == dst {
		return "", "", usererr.Coded("sftp.staging_equals_remote", "Edit staging path equals remote path")
	}
	dstExists := dstInfo != nil
	dstRealDir := dstExists && isRealDir(dstInfo)
	dstSymlink := dstExists && dstInfo.Mode()&os.ModeSymlink != 0
	// Type conflict: the destination exists and its kind differs from the
	// source (file over directory or directory over file/symlink). mv cannot
	// swap kinds in one rename, so the destination is removed first — the
	// user confirmed this overwrite in the conflict prompt.
	typeConflict := dstExists && (srcIsDir != dstRealDir)

	qs, qd, qst := shellQuote(src), shellQuote(dst), shellQuote(staging)

	if srcIsDir && dstRealDir {
		// Directory merge: cp -a would apply the source tree's modes onto
		// existing destination subdirectories (broadening previously private
		// ones) and write replacement bytes directly over existing
		// destination files (lost when the copy fails midway). Run the merge
		// on the staged SFTP path instead.
		return "", "", errPasteCommandFallback
	}
	if cut {
		if typeConflict || dstSymlink {
			return fmt.Sprintf("rm -rf -- %s && mv -f -- %s %s", qd, qs, qd), "", nil
		}
		return fmt.Sprintf("mv -f -- %s %s", qs, qd), "", nil
	}
	// Copy: build at the staging sibling, then swap onto dst.
	parts := []string{
		"rm -rf -- " + qst,
		fmt.Sprintf("cp -a -- %s %s", qs, qst),
	}
	if typeConflict || dstSymlink {
		parts = append(parts, "rm -rf -- "+qd)
	}
	parts = append(parts, fmt.Sprintf("mv -f -- %s %s", qst, qd))
	return strings.Join(parts, " && "), "rm -rf -- " + qst, nil
}

// pasteRemoteCommand tries to perform one paste job as a single remote shell
// command (see pasteCommand) on a fresh, non-interactive SSH session — data
// stays on the server and the interactive terminal is never involved. Any
// failure of the command path (no exec channel, missing tools, a rejected
// flag, a real command error) returns an error wrapping errPasteCommandFallback
// so the caller can replay the paste via the streaming SFTP implementation;
// staging leftovers of a failed copy are removed first. Hard errors (bad
// parameters, connection problems) are returned as-is.
//
// The command runs under ctx (the tracker context): cancellation closes the
// remote session, is never replayed over SFTP and is reported as a cancelled
// result even when the command happens to finish after the user's request.
func (sft *Manager) pasteRemoteCommand(ctx context.Context, sessionID string, job pasteJob, cut, overwrite bool) error {
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	// Lstat: a symlink source is a leaf entry (cp -a / mv recreate the link
	// itself), not the tree or file its target happens to be.
	srcInfo, err := client.Lstat(job.src)
	if err != nil {
		return usererr.Wrap(err, "sftp.remote_path", "Remote path %s: %s", job.src)
	}
	dstInfo, dstErr := client.Lstat(job.dst)
	if dstErr != nil && !os.IsNotExist(dstErr) {
		return dstErr
	}
	if dstErr == nil && !overwrite {
		return usererr.Coded("sftp.path_exists", "%s already exists", job.dst)
	}
	var dst os.FileInfo
	if dstErr == nil {
		dst = dstInfo
	}
	staging := stagingRemotePath(job.dst)
	cmd, cleanup, err := pasteCommand(job.src, job.dst, srcInfo.IsDir(), cut, dst, staging)
	if err != nil {
		return err
	}
	sft.logger.Info("paste remote command",
		zap.String("src", job.src), zap.String("dst", job.dst),
		zap.Bool("cut", cut), zap.String("command", cmd))
	if _, runErr := sft.runRemoteCommand(ctx, sessionID, cmd, 0, ssh.DefaultExecMaxOut); runErr != nil {
		if cleanup != "" {
			// Cleanup runs even after cancellation: it only removes this
			// operation's own staging entry, on its own session.
			if _, rmErr := sft.runRemoteCommand(context.Background(), sessionID, cleanup, 0, ssh.DefaultExecMaxOut); rmErr != nil {
				sft.logger.Warn("paste staging cleanup failed", zap.String("command", cleanup), zap.Error(rmErr))
			}
		}
		if ctx.Err() != nil {
			// Cancelled: stop here. Do not replay the paste over SFTP (that
			// would keep modifying files after cancellation).
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", errPasteCommandFallback, runErr)
	}
	if ctx.Err() != nil {
		// The command may have completed after the user cancelled; report the
		// cancellation so success cleanup (cut source deletion) does not run.
		return ctx.Err()
	}
	return nil
}
