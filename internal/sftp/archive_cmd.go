package sftp

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/usererr"
)

// archiveFormat identifies the supported archive formats. Detection is by file
// name extension (case-insensitive).
type archiveFormat int

const (
	formatUnknown archiveFormat = iota
	formatZip
	formatTar
	formatTarGz
	formatTarBz2
	formatTarXz
	formatGz // bare single-file compressed streams, not archives
	formatBz2
	formatXz
)

// detectArchiveFormat maps an archive file name to its format. Longest
// extensions are matched first (.tar.gz before .gz).
func detectArchiveFormat(name string) archiveFormat {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"), strings.HasSuffix(lower, ".taz"):
		return formatTarGz
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"), strings.HasSuffix(lower, ".tbz"):
		return formatTarBz2
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return formatTarXz
	case strings.HasSuffix(lower, ".tar"):
		return formatTar
	case strings.HasSuffix(lower, ".zip"):
		return formatZip
	case strings.HasSuffix(lower, ".gz"):
		return formatGz
	case strings.HasSuffix(lower, ".bz2"):
		return formatBz2
	case strings.HasSuffix(lower, ".xz"):
		return formatXz
	}
	return formatUnknown
}

// isTarFamily reports whether f is a tar stream, optionally compressed.
func isTarFamily(f archiveFormat) bool {
	return f == formatTar || f == formatTarGz || f == formatTarBz2 || f == formatTarXz
}

// isSingleStream reports whether f is a bare compressed single-file stream.
func isSingleStream(f archiveFormat) bool {
	return f == formatGz || f == formatBz2 || f == formatXz
}

// tarCompressFlag returns the tar compression flag shared by c/x/t modes.
// Empty for plain tar.
func tarCompressFlag(f archiveFormat) string {
	switch f {
	case formatTarGz:
		return "z"
	case formatTarBz2:
		return "j"
	case formatTarXz:
		return "J"
	}
	return ""
}

// shellQuote wraps s in POSIX single quotes so the remote shell receives it as
// one literal word, whatever spaces, quotes, $, backticks or newlines it holds.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// archiveListMaxOut bounds remote listing output (32 MB of names ≈ 800k
// entries); extraction and compression failures only return diagnostic tails.
const archiveListMaxOut = 32 << 20

// compressCommand builds the remote tar invocation archiving srcPaths into
// archivePath. Compression is fixed to tar.gz — no format dispatch. Every
// top-level entry is stored under its basename (a single directory keeps
// itself as the archive root, Windows explorer behavior); each source gets
// its own -C group so sources may come from different directories. A name
// starting with "-" is stored as "./-name" so it can never be parsed as a tar
// option.
func compressCommand(archivePath string, srcPaths []string) (string, error) {
	if len(srcPaths) == 0 {
		return "", usererr.Coded("sftp.invalid_compress", "Invalid compress parameters")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "tar czf %s", shellQuote(archivePath))
	for _, src := range srcPaths {
		clean := strings.TrimSuffix(src, "/")
		dir, base := path.Split(clean)
		dir = strings.TrimSuffix(dir, "/")
		if dir == "" {
			dir = "/"
		}
		if base == "" || base == "." || base == ".." {
			return "", usererr.Coded("sftp.invalid_compress", "Invalid compress parameters")
		}
		member := base
		if strings.HasPrefix(base, "-") {
			member = "./" + base
		}
		fmt.Fprintf(&b, " -C %s %s", shellQuote(dir), shellQuote(member))
	}
	return b.String(), nil
}

// listTarCommand builds the remote listing invocation for a tar-family
// archive. Output is one member name per line, directories with a trailing
// slash (GNU tar, bsdtar and busybox tar all print names this way).
func listTarCommand(f archiveFormat, archivePath string) string {
	return fmt.Sprintf("tar t%sf %s", tarCompressFlag(f), shellQuote(archivePath))
}

// extractTarCommand builds the remote extraction invocation. strip == 1 drops
// one leading path component from every member (mirrors flattenEntries on the
// streaming zip path) so the archive's common root folder is not recreated.
func extractTarCommand(f archiveFormat, archivePath, destDir string, strip int) string {
	cmd := fmt.Sprintf("tar x%sf %s -C %s", tarCompressFlag(f), shellQuote(archivePath), shellQuote(destDir))
	if strip == 1 {
		cmd += " --strip-components=1"
	}
	return cmd
}

// extractSingleCommand builds the decompression of a bare compressed stream
// (file.gz and friends) onto target, i.e. gunzip semantics on the server.
func extractSingleCommand(f archiveFormat, archivePath, target string) string {
	tool := "gzip"
	switch f {
	case formatBz2:
		tool = "bzip2"
	case formatXz:
		tool = "xz"
	}
	return fmt.Sprintf("%s -dc %s > %s", tool, shellQuote(archivePath), shellQuote(target))
}

// singleStreamName returns the extracted file name for a bare compressed
// stream: "dir/file.txt.gz" extracts as "file.txt". Empty when nothing
// remains after stripping the compression extension.
func singleStreamName(archivePath string) string {
	base := path.Base(archivePath)
	for _, ext := range []string{".gz", ".bz2", ".xz"} {
		if len(base) > len(ext) && strings.HasSuffix(strings.ToLower(base), ext) {
			return base[:len(base)-len(ext)]
		}
	}
	return ""
}

// archiveEntrySafety normalizes one tar member name for validation and reports
// whether the name is unsafe to extract via a remote tar. Unlike the streaming
// zip path — which can skip a single bad entry — the command path extracts the
// whole archive at once, so any unsafe member aborts the operation up front.
// Absolute names, drive letters and ".." segments are unsafe (zip-slip).
func archiveEntrySafety(raw string) (name string, unsafe bool) {
	raw = strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(raw, "/") {
		return "", true
	}
	if len(raw) >= 2 && raw[1] == ':' {
		return "", true
	}
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", true
		}
		out = append(out, part)
	}
	return strings.Join(out, "/"), false
}

// execUnavailable and execFailed classify a remote command outcome.
const (
	execOK = iota
	execUnavailable
	execFailed
)

// classifyExec maps a remote command outcome to a class: execUnavailable when
// the command never really ran (no exec channel, or the shell could not find /
// execute the tool — exit 126/127), execFailed when the tool ran and reported
// a real error (corrupt archive, missing source…).
func classifyExec(hasStatus bool, status int, err error) int {
	if err == nil {
		return execOK
	}
	if !hasStatus {
		return execUnavailable
	}
	if status == 126 || status == 127 {
		return execUnavailable
	}
	return execFailed
}

// commandFailureDetail renders a diagnostic tail for a failed remote command.
func commandFailureDetail(out string, err error) string {
	detail := strings.TrimSpace(out)
	if detail == "" && err != nil {
		detail = err.Error()
	}
	const maxDetail = 600
	if r := []rune(detail); len(r) > maxDetail {
		detail = "…" + string(r[len(r)-maxDetail:])
	}
	return detail
}

// runRemoteCommand executes cmd on a fresh, non-interactive SSH session of
// sessionID's connection — never on the interactive terminal session — and
// classifies failures into user-facing coded errors. timeout <= 0 means no
// time limit.
func (sft *Manager) runRemoteCommand(ctx context.Context, sessionID, cmd string, timeout time.Duration, maxOut int) (string, error) {
	if sft.remoteExec == nil {
		return "", usererr.Coded("sftp.remote_exec_unavailable", "Remote command execution is unavailable on this server")
	}
	if timeout <= 0 {
		timeout = 0
	}
	out, err := sft.remoteExec(ctx, sessionID, cmd, timeout, maxOut)
	status, has := ssh.ExitStatus(err)
	switch classifyExec(has, status, err) {
	case execOK:
		return out, nil
	case execUnavailable:
		return out, usererr.Coded("sftp.remote_exec_unavailable", "Remote command execution is unavailable on this server")
	default:
		return out, usererr.Coded("sftp.remote_command_failed", "Remote command failed: %s", commandFailureDetail(out, err))
	}
}
