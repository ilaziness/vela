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
// slash (GNU tar, bsdtar and busybox tar all print names this way). GNU tar
// quotes control characters and backslashes in names (escape style) so every
// member stays on one line; TAR_OPTIONS pins that style explicitly so the
// listing is decoded with unescapeTarName. busybox tar lacks the option but
// also never quotes: the environment variable is ignored and its raw names
// pass through unchanged.
func listTarCommand(f archiveFormat, archivePath string) string {
	return fmt.Sprintf("TAR_OPTIONS=--quoting-style=escape tar t%sf %s", tarCompressFlag(f), shellQuote(archivePath))
}

// extractTarCommand builds the remote extraction invocation. strip == 1 drops
// one leading path component from every member (mirrors flattenEntries on the
// streaming zip path) so the archive's common root folder is not recreated;
// note that tar counts a leading "./" as that first component. With overwrite
// false, -k turns any member target that appeared after the conflict scan into
// an error instead of silently replacing it (GNU and busybox tar both support
// -k).
func extractTarCommand(f archiveFormat, archivePath, destDir string, strip int, overwrite bool) string {
	cmd := fmt.Sprintf("tar x%sf %s -C %s", tarCompressFlag(f), shellQuote(archivePath), shellQuote(destDir))
	if strip == 1 {
		cmd += " --strip-components=1"
	}
	if !overwrite {
		cmd += " -k"
	}
	return cmd
}

// extractSingleCommand builds the decompression of a bare compressed stream
// (file.gz and friends) on the server: the stream is decompressed into a
// unique staging sibling first and moved onto target only when complete, so a
// failed or truncated decompression never destroys an existing target. The
// final rename replaces a symlink entry at target instead of writing through
// it (rm -rf unlinks the link; the guarded branch refuses to touch an existing
// target). With overwrite false, a target that appears after the conflict scan
// is left untouched and the operation fails.
func extractSingleCommand(f archiveFormat, archivePath, staging, target string, overwrite bool) string {
	tool := "gzip"
	switch f {
	case formatBz2:
		tool = "bzip2"
	case formatXz:
		tool = "xz"
	}
	// A decompression failure removes the partial staging file and fails the
	// chain before anything touches target.
	head := fmt.Sprintf("%s -dc %s > %s", tool, shellQuote(archivePath), shellQuote(staging)) +
		" || { rm -f -- " + shellQuote(staging) + "; false; }"
	qt, qst := shellQuote(target), shellQuote(staging)
	if overwrite {
		return head + fmt.Sprintf(" && rm -rf -- %s && mv -f -- %s %s", qt, qst, qt)
	}
	return head + fmt.Sprintf(" && if [ -e %s ] || [ -L %s ]; then rm -f -- %s; false; else mv -f -- %s %s; fi",
		qt, qt, qst, qst, qt)
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

// unescapeTarName decodes GNU tar's "escape" listing quoting: backslash
// sequences such as \n, \t, \\ and \NNN octal bytes stand for the real
// characters, keeping every member on one output line. An unknown escape
// keeps both characters verbatim (raw names from non-quoting tar variants
// round-trip unchanged unless they literally contain escape sequences).
func unescapeTarName(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(s) {
			b.WriteByte('\\')
			break
		}
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case 'a':
			b.WriteByte('\a')
		case '\\':
			b.WriteByte('\\')
		case '0', '1', '2', '3', '4', '5', '6', '7':
			v := int(s[i] - '0')
			for n := 0; n < 2 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7'; n++ {
				i++
				v = v*8 + int(s[i]-'0')
			}
			b.WriteByte(byte(v))
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// archiveEntrySafety normalizes one tar member name for validation and reports
// whether the name is unsafe to extract via a remote tar. Unlike the streaming
// zip path — which can skip a single bad entry — the command path extracts the
// whole archive at once, so any unsafe member aborts the operation up front.
// Absolute names, drive letters and ".." segments are unsafe (zip-slip).
// "." components are preserved: tar counts a leading "./" as the first path
// component (--strip-components=1 on "./src/a" yields "src/a"), so keeping the
// raw structure lets flattenEntries and the extract command agree on the paths
// actually written. A backslash is an ordinary POSIX filename character, never
// a separator.
func archiveEntrySafety(raw string) (name string, unsafe bool) {
	if strings.HasPrefix(raw, "/") {
		return "", true
	}
	if len(raw) >= 2 && raw[1] == ':' {
		return "", true
	}
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
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
