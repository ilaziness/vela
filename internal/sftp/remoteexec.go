package sftp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"

	"github.com/ilaziness/vela/internal/usererr"
)

// Remote archive formats. The format string doubles as the archive
// extension; tar.bz2 is decompress-only (no compress format uses it).
const (
	ArchZip    = "zip"
	ArchTar    = "tar"
	ArchTarGz  = "tar.gz"
	ArchTarBz2 = "tar.bz2"
	ArchTarXz  = "tar.xz"
	ArchTarZst = "tar.zst"
)

// CompressFormats returns the formats offered by the compress dialog.
func CompressFormats() []string {
	return []string{ArchZip, ArchTar, ArchTarGz, ArchTarXz, ArchTarZst}
}

func isCompressFormat(format string) bool {
	switch format {
	case ArchZip, ArchTar, ArchTarGz, ArchTarXz, ArchTarZst:
		return true
	}
	return false
}

// remoteExecMaxOut bounds the CombinedOutput captured per remote command.
const remoteExecMaxOut = 256 * 1024

// remoteErrDetailMax bounds the stderr excerpt merged into an error that
// is shown in a toast (zip prints one line per member).
const remoteErrDetailMax = 4 * 1024

// execFunc matches ssh.Manager.Exec; a field on Manager keeps it
// injectable for tests without package-level mutable state.
type execFunc func(ctx context.Context, sessionID, command string, timeout time.Duration, maxOut int) (string, error)

// shellQuote single-quotes s for POSIX shells (' → '\”).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shCmd wraps inner in `sh -c` so operators like && work regardless of
// the account's login shell (csh/tcsh lack &&).
func shCmd(inner string) string {
	return "sh -c " + shellQuote(inner)
}

// execRemote runs a remote shell command on the session's SSH client.
// The context is cancellable via CancelRemoteOp; cancel surfaces as
// context.Canceled. Non-zero exits merge stderr/stdout into the error.
func (sft *Manager) execRemote(sessionID, command string) (string, error) {
	if sft.execFn == nil {
		return "", usererr.Coded("sftp.exec_unavailable", "Remote command execution is unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, loaded := sft.execCancels.LoadOrStore(sessionID, cancel); loaded {
		cancel()
		return "", usererr.Coded("sftp.op_running", "Another remote operation is already running")
	}
	defer func() {
		sft.execCancels.Delete(sessionID)
		cancel()
	}()
	out, err := sft.execFn(ctx, sessionID, command, 0, remoteExecMaxOut)
	if err == nil {
		return out, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return out, err
	}
	detail := strings.TrimSpace(out)
	if detail == "" {
		return out, err
	}
	if len(detail) > remoteErrDetailMax {
		detail = strings.ToValidUTF8(detail[:remoteErrDetailMax], "") + "\n…"
	}
	return out, errors.New(detail + "\n" + err.Error())
}

// CancelRemoteOp cancels the in-flight remote exec for sessionID, if any.
func (sft *Manager) CancelRemoteOp(sessionID string) {
	if v, ok := sft.execCancels.Load(sessionID); ok {
		v.(context.CancelFunc)()
	}
}

func (sft *Manager) cancelAllRemoteOps() {
	sft.execCancels.Range(func(_, v any) bool {
		v.(context.CancelFunc)()
		return true
	})
}

// --- copy / move (clipboard paste backend) ---

// RemotePasteConflicts returns basenames under destDir that already
// exist for the given remote source paths.
func (sft *Manager) RemotePasteConflicts(sessionID, destDir string, srcPaths []string) ([]string, error) {
	if sessionID == "" || destDir == "" || len(srcPaths) == 0 {
		return nil, usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
	}
	ftpClient, err := sft.getSftpClient(sessionID)
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
		if _, statErr := ftpClient.Stat(dest); statErr == nil {
			conflicts = append(conflicts, name)
		} else if !os.IsNotExist(statErr) {
			return nil, statErr
		}
	}
	return conflicts, nil
}

// pasteDest computes where src lands inside destDir.
func pasteDest(destDir, src string) string {
	return joinRemotePath(destDir, path.Base(src))
}

// checkPasteGuard rejects copying/moving a path into itself or its
// own subtree (dest == src or dest inside src).
func checkPasteGuard(srcPaths []string, destDir string) error {
	for _, src := range srcPaths {
		srcClean := path.Clean("/" + strings.TrimPrefix(src, "/"))
		dest := pasteDest(destDir, srcClean)
		if dest == srcClean || strings.HasPrefix(dest, srcClean+"/") {
			return usererr.Coded("sftp.paste_into_self", "Cannot paste %s into itself", path.Base(srcClean))
		}
	}
	return nil
}

// srcIsDir reports whether remote path is a directory.
func srcIsDir(ftpClient *sftp.Client, remotePath string) (bool, error) {
	st, err := ftpClient.Stat(remotePath)
	if err != nil {
		return false, err
	}
	return st.IsDir(), nil
}

// resolveCopyDest fixes type conflicts before cp: a file landing on an
// existing directory (or vice versa) would be misplaced by cp, so the
// destination is removed first. Directory→directory merges as usual.
func (sft *Manager) resolveCopyDest(ftpClient *sftp.Client, destPath string, srcDir bool) error {
	st, err := ftpClient.Stat(destPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st.IsDir() == srcDir {
		return nil // same type: file overwrite / dir merge
	}
	return ftpClient.RemoveAll(destPath)
}

// resolveMoveDest removes any existing destination before mv: mv cannot
// overwrite a non-empty directory and would drop a file inside an
// existing same-named directory. Only called after overwrite confirm.
func (sft *Manager) resolveMoveDest(ftpClient *sftp.Client, destPath string) error {
	if _, err := ftpClient.Stat(destPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return ftpClient.RemoveAll(destPath)
}

func buildCopyCmd(srcPaths []string, destDir string) string {
	inner := "cp -rf"
	for _, src := range srcPaths {
		inner += " " + shellQuote(src)
	}
	inner += " " + shellQuote(strings.TrimSuffix(destDir, "/")+"/")
	return shCmd(inner)
}

func buildMoveCmd(srcPaths []string, destDir string) string {
	inner := "mv -f"
	for _, src := range srcPaths {
		inner += " " + shellQuote(src)
	}
	inner += " " + shellQuote(strings.TrimSuffix(destDir, "/")+"/")
	return shCmd(inner)
}

// CopyRemotePaths copies remote srcPaths into destDir server-side.
func (sft *Manager) CopyRemotePaths(sessionID string, srcPaths []string, destDir string) error {
	if sessionID == "" || destDir == "" || len(srcPaths) == 0 {
		return usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
	}
	if err := checkPasteGuard(srcPaths, destDir); err != nil {
		return err
	}
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	for _, src := range srcPaths {
		srcDir, err := srcIsDir(ftpClient, src)
		if err != nil {
			return usererr.Wrap(err, "sftp.remote_path", "Remote path %s: %s", src)
		}
		if err := sft.resolveCopyDest(ftpClient, pasteDest(destDir, src), srcDir); err != nil {
			return err
		}
	}
	_, err = sft.execRemote(sessionID, buildCopyCmd(srcPaths, destDir))
	return err
}

// MoveRemotePaths moves (cut) remote srcPaths into destDir server-side.
// Existing destinations are removed first; callers must have confirmed
// overwrite via RemotePasteConflicts.
func (sft *Manager) MoveRemotePaths(sessionID string, srcPaths []string, destDir string) error {
	if sessionID == "" || destDir == "" || len(srcPaths) == 0 {
		return usererr.Coded("sftp.invalid_paste", "Invalid paste parameters")
	}
	if err := checkPasteGuard(srcPaths, destDir); err != nil {
		return err
	}
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	for _, src := range srcPaths {
		if err := sft.resolveMoveDest(ftpClient, pasteDest(destDir, src)); err != nil {
			return err
		}
	}
	_, err = sft.execRemote(sessionID, buildMoveCmd(srcPaths, destDir))
	return err
}

// --- compress / extract ---

// detectArchiveFormat maps a remote filename to its archive format.
func detectArchiveFormat(name string) (string, bool) {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return ArchZip, true
	case strings.HasSuffix(lower, ".tar.zst"), strings.HasSuffix(lower, ".tzst"):
		return ArchTarZst, true
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return ArchTarXz, true
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return ArchTarBz2, true
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return ArchTarGz, true
	case strings.HasSuffix(lower, ".tar"):
		return ArchTar, true
	}
	return "", false
}

// IsArchiveName reports whether name looks like a supported archive.
func IsArchiveName(name string) bool {
	_, ok := detectArchiveFormat(name)
	return ok
}

// uniqueArchiveName picks dir/<base>.<format>, suffixing -1, -2… on clash.
func (sft *Manager) uniqueArchiveName(sessionID, dir, base, format string) (string, error) {
	ftpClient, err := sft.getSftpClient(sessionID)
	if err != nil {
		return "", err
	}
	for i := 0; ; i++ {
		name := base + "." + format
		if i > 0 {
			name = fmt.Sprintf("%s-%d.%s", base, i, format)
		}
		if _, err := ftpClient.Stat(joinRemotePath(dir, name)); err != nil {
			if os.IsNotExist(err) {
				return name, nil
			}
			return "", err
		}
	}
}

// buildCompressCmd builds `cd dir && <archiver> out './m1' './m2' …`.
// Members get a ./ prefix so names starting with '-' aren't parsed as
// options; quoting keeps spaces/quotes/UTF-8 safe.
func buildCompressCmd(dir, outName string, names []string, format string) (string, error) {
	var arch string
	switch format {
	case ArchZip:
		arch = "zip -r"
	case ArchTar:
		arch = "tar -cf"
	case ArchTarGz:
		arch = "tar -czf"
	case ArchTarXz:
		arch = "tar -cJf"
	case ArchTarZst:
		arch = "tar --zstd -cf"
	default:
		return "", usererr.Coded("sftp.unsupported_format", "Unsupported archive format: %s", format)
	}
	inner := "cd " + shellQuote(dir) + " && " + arch + " " + shellQuote("./"+outName)
	for _, n := range names {
		inner += " " + shellQuote("./"+n)
	}
	return shCmd(inner), nil
}

// CompressRemote archives remote entry names (basenames inside dir)
// into dir/<base>.<format> on the server.
func (sft *Manager) CompressRemote(sessionID, dir string, names []string, format string) (string, error) {
	if sessionID == "" || dir == "" || len(names) == 0 {
		return "", usererr.Coded("sftp.invalid_compress", "Invalid compress parameters")
	}
	if !isCompressFormat(format) {
		return "", usererr.Coded("sftp.unsupported_format", "Unsupported archive format: %s", format)
	}
	base := "archive"
	if len(names) == 1 {
		base = names[0]
	}
	outName, err := sft.uniqueArchiveName(sessionID, dir, base, format)
	if err != nil {
		return "", err
	}
	cmd, err := buildCompressCmd(dir, outName, names, format)
	if err != nil {
		return "", err
	}
	if _, err := sft.execRemote(sessionID, cmd); err != nil {
		return "", err
	}
	return joinRemotePath(dir, outName), nil
}

// tarExtractArgs maps a format to tar's extract arguments (leading dashes
// included so --zstd doesn't end up triple-dashed).
func tarExtractArgs(format string) (string, bool) {
	switch format {
	case ArchTar:
		return "-xf", true
	case ArchTarGz:
		return "-xzf", true
	case ArchTarBz2:
		return "-xjf", true
	case ArchTarXz:
		return "-xJf", true
	case ArchTarZst:
		return "--zstd -xf", true
	}
	return "", false
}

// buildExtractCmd builds the extract command for one archive.
// destDir is the archive's parent directory (absolute path).
func buildExtractCmd(remotePath, destDir, format string) (string, error) {
	if format == ArchZip {
		inner := "unzip -o " + shellQuote(remotePath) + " -d " + shellQuote(destDir)
		return shCmd(inner), nil
	}
	flags, ok := tarExtractArgs(format)
	if !ok {
		return "", usererr.Coded("sftp.unsupported_format", "Unsupported archive format: %s", format)
	}
	inner := "tar " + flags + " " + shellQuote(remotePath) + " -C " + shellQuote(destDir)
	return shCmd(inner), nil
}

// ExtractRemote decompresses each remote archive into its parent dir.
// Archives are processed sequentially; the first failure aborts.
func (sft *Manager) ExtractRemote(sessionID string, remotePaths []string) error {
	if sessionID == "" || len(remotePaths) == 0 {
		return usererr.Coded("sftp.invalid_extract", "Invalid extract parameters")
	}
	for _, p := range remotePaths {
		format, ok := detectArchiveFormat(p)
		if !ok {
			return usererr.Coded("sftp.unsupported_archive", "Unsupported archive: %s", path.Base(p))
		}
		destDir := path.Dir(p)
		if destDir == "." || destDir == "" {
			destDir = "/"
		}
		cmd, err := buildExtractCmd(p, destDir, format)
		if err != nil {
			return err
		}
		if _, err := sft.execRemote(sessionID, cmd); err != nil {
			return usererr.Wrap(err, "sftp.extract_failed", "Extract %s: %s", path.Base(p))
		}
	}
	return nil
}
