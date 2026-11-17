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

// extractTarCommand builds the remote extraction invocation for the overwrite
// path. strip == 1 drops one leading path component from every member (mirrors
// flattenEntries on the streaming zip path) so the archive's common root folder
// is not recreated; note that tar counts a leading "./" as that first component.
// tar runs with its normal last-member-wins semantics: an archive updated with
// tar -r stores several versions of one pathname and must restore the newest
// one. The no-overwrite path uses extractTarNoOverwriteCommand instead — a
// plain tar guard such as -k would also reject the later version of a repeated
// member because the earlier one already created the path.
func extractTarCommand(f archiveFormat, archivePath, destDir string, strip int) string {
	cmd := fmt.Sprintf("tar x%sf %s -C %s", tarCompressFlag(f), shellQuote(archivePath), shellQuote(destDir))
	if strip == 1 {
		cmd += " --strip-components=1"
	}
	return cmd
}

// extractTarNoOverwriteCommand builds the remote extraction for the
// no-overwrite path, separating archive materialization from destination
// conflict protection: the archive is extracted into an operation-owned
// staging directory first with normal last-member-wins semantics (repeated
// members — e.g. from tar -r — keep the newest content), and only the final
// extracted entries are promoted into destDir under per-entry guards:
//   - a target that exists (including as a symlink) fails the operation and is
//     left untouched;
//   - promotion never overwrites: files and symlinks are installed with
//     link(2)/symlink(2) semantics (creation fails on an existing name — the
//     atomic reservation the mv -f move lacked), directories are claimed with
//     mkdir(2), so a target created concurrently between check and promotion
//     is a reported conflict rather than a silent replacement;
//   - missing parent directories are created as real directories, refusing to
//     write through a symlink in the parent chain (extractParentsSafe parity)
//     and tolerating a parent another writer created concurrently;
//   - directories created by this operation keep the archive's mode, but it is
//     applied only AFTER all descendants are installed: a read-only archive
//     directory (0555) would otherwise block its own children. New directories
//     stay 0700 (owner-only) during population; pre-existing target directories
//     — including their mode — are untouched. The archived mode is carried by a
//     unique marker inside the staging tree and restored with find -depth, so
//     no newline-bearing member name can inject an extra chmod target;
//   - a hard link that fails with EXDEV (a nested destination mount lives on a
//     different filesystem) falls back to copying REGULAR files only into an
//     operation-owned file on the target filesystem and claiming the final
//     name with the same atomic no-overwrite link. Special entries (FIFOs,
//     devices, sockets) are rejected explicitly instead of being read by cp
//     (reading a FIFO would block forever with no writer);
//   - symlink targets are reproduced byte-for-byte (sentinel capture strips
//     only readlink's own output delimiter, keeping trailing newlines that
//     command substitution would otherwise remove);
//
// The staging tree lives INSIDE destDir (write access to destDir's parent must
// not be required) and is removed on every exit path. The promotion loop walks
// the staging tree itself instead of an inline name list: the command size
// stays bounded for archives with thousands of members (the whole command is
// one shell argument, capped at ~128KiB per argument on Linux).
//
// All paths are resolved to absolute forms against the initial working
// directory before the shell changes into a globally searchable directory:
// find(1) saves a descriptor of its startup cwd and cannot restore it when a
// non-root user inherits an unreadable cwd (e.g. /root via su), while relative
// destination/archive spellings must keep their original meaning.
func extractTarNoOverwriteCommand(f archiveFormat, archivePath, destDir string, strip int) string {
	st := stagingDirRemotePath(destDir)
	// Unique marker name for directories this operation creates. The marker
	// lives INSIDE the staging tree (never in the destination) and holds only
	// the archived mode digits, so restoring modes needs no text list of member
	// names: a name containing a newline can never inject a chmod target.
	mk := ".vela-mode-" + stagingToken()
	tarPart := fmt.Sprintf("tar x%sf \"$a\" -C \"$st\"", tarCompressFlag(f))
	if strip == 1 {
		tarPart += " --strip-components=1"
	}
	// The inner shell runs one invocation per find batch ("{} +", so the
	// argument count stays bounded too) and reports failures through a
	// marker file: find's own exit status is not relied on across
	// implementations. Paths are handled as "$1"-style arguments, so names
	// with spaces, newlines or glob characters are safe.
	promote := "rc=0; find \"$st\" -mindepth 1 -exec sh -c '" +
		"st=$1; fl=$2; d=$3; mk=$4; shift 4; " +
		"for s in \"$@\"; do " +
		"case ${s##*/} in \"$mk\") continue;; esac; " +
		"n=${s#\"$st\"/}; t=\"$d/$n\"; " +
		"[ -e \"$s\" ] || [ -L \"$s\" ] || continue; " +
		"if [ -d \"$s\" ] && [ ! -L \"$s\" ]; then " +
		"if [ -d \"$t\" ] && [ ! -L \"$t\" ]; then continue; fi; " +
		"if [ -e \"$t\" ] || [ -L \"$t\" ]; then touch -- \"$fl\"; continue; fi; " +
		"q=\"$d\"; r=\"$n\"; while [ \"$r\" != \"${r%/*}\" ]; do c=\"${r%%/*}\"; r=\"${r#*/}\"; q=\"$q/$c\"; " +
		"if [ -L \"$q\" ]; then touch -- \"$fl\"; continue 2; fi; " +
		"if [ ! -d \"$q\" ]; then mkdir -- \"$q\" 2>/dev/null || { [ -d \"$q\" ] && [ ! -L \"$q\" ] || { touch -- \"$fl\"; continue 2; }; }; fi; done; " +
		"if mkdir -- \"$t\" 2>/dev/null; then " +
		"m=$(stat -c %a -- \"$s\") && chmod -- 700 \"$t\" && chmod u+rwx -- \"$s\" && printf \"%s\" \"$m\" > \"$s/$mk\" || touch -- \"$fl\"; " +
		"elif [ ! -d \"$t\" ] || [ -L \"$t\" ]; then touch -- \"$fl\"; fi; " +
		"continue; fi; " +
		"if [ -e \"$t\" ] || [ -L \"$t\" ]; then touch -- \"$fl\"; continue; fi; " +
		"q=\"$d\"; r=\"$n\"; while [ \"$r\" != \"${r%/*}\" ]; do c=\"${r%%/*}\"; r=\"${r#*/}\"; q=\"$q/$c\"; " +
		"if [ -L \"$q\" ]; then touch -- \"$fl\"; continue 2; fi; " +
		"if [ ! -d \"$q\" ]; then mkdir -- \"$q\" 2>/dev/null || { [ -d \"$q\" ] && [ ! -L \"$q\" ] || { touch -- \"$fl\"; continue 2; }; }; fi; done; " +
		"if [ -L \"$s\" ]; then " +
		"g=$(readlink -- \"$s\" && printf x) && g=${g%x} && g=${g%?} && ln -s -- \"$g\" \"$t\" || touch -- \"$fl\"; " +
		"elif ln -- \"$s\" \"$t\" 2>/dev/null; then :; " +
		// Only regular files are byte-copied across filesystems: cp -p on a
		// FIFO would block forever waiting for a writer (no exec timeout).
		"elif [ -f \"$s\" ]; then c=$(mktemp \"${t%/*}/.vela-copy-XXXXXX\") && cp -p -- \"$s\" \"$c\" && ln -- \"$c\" \"$t\" && rm -f -- \"$c\" || { rm -f -- \"$c\"; touch -- \"$fl\"; }; " +
		"else touch -- \"$fl\"; fi; " +
		"done' sh \"$st\" \"$fl\" \"$d\" \"$mk\" {} + || touch -- \"$fl\"; " +
		// Restore archived directory modes bottom-up: find -depth visits a
		// child before its parent, and the markers live in the staging tree,
		// so no member name is ever parsed as a line of text.
		"find \"$st\" -depth -name \"$mk\" -exec sh -c 'st=$1; d=$2; fl=$3; shift 3; " +
		"for f; do s=${f%/*}; n=${s#\"$st\"/}; t=\"$d/$n\"; m=$(cat -- \"$f\"); rm -f -- \"$f\"; chmod -- \"$m\" \"$t\" || touch -- \"$fl\"; done' sh \"$st\" \"$d\" \"$fl\" {} + || touch -- \"$fl\"; " +
		"rc=0; [ -e \"$fl\" ] && rc=1; " +
		// Make read-only staged directories removable, then clean up.
		"find \"$st\" -type d -exec chmod u+rwx -- {} + 2>/dev/null; rm -rf -- \"$st\" \"$fl\"; exit \"$rc\"; }"
	// Resolve every path against the initial cwd, then start find from a
	// globally searchable directory (see the function comment).
	head := "o=$(pwd); st=" + shellQuote(st) + " fl=" + shellQuote(st+".fail") +
		" d=" + shellQuote(destDir) + " a=" + shellQuote(archivePath) + " mk=" + shellQuote(mk) +
		" && case \"$st\" in /*) ;; *) st=\"$o/$st\";; esac" +
		" && case \"$d\" in /*) ;; *) d=\"$o/$d\";; esac" +
		" && case \"$a\" in /*) ;; *) a=\"$o/$a\";; esac" +
		" && fl=\"$st.fail\" && cd /" +
		" && rm -rf -- \"$st\" \"$fl\" && mkdir -- \"$st\" && " + tarPart +
		" && { " + promote +
		" || { find \"$st\" -type d -exec chmod u+rwx -- {} + 2>/dev/null; rm -rf -- \"$st\" \"$fl\"; false; }"
	return head
}

// extractSingleCommand builds the decompression of a bare compressed stream
// (file.gz and friends) on the server: the stream is decompressed into a
// unique staging sibling first and moved onto target only when complete, so a
// failed or truncated decompression never destroys an existing target. The
// final rename replaces a symlink entry at target instead of writing through
// it (rm -rf unlinks the link; the guarded branch refuses to touch an existing
// target). With overwrite false, a target that appears after the conflict scan
// is left untouched and the operation fails. When overwriting an existing
// regular file, the target's permission bits are preserved: the staged file
// arrives with the shell's umask default (e.g. 0644) and must never broaden a
// private target (e.g. 0600).
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
		// Keep the replaced file's mode (fail closed when it cannot be read);
		// a symlink at target is replaced, never written through, so no mode
		// is read from it.
		preserve := fmt.Sprintf("if [ -f %s ] && [ ! -L %s ]; then m=$(stat -c %%a -- %s) && chmod -- \"$m\" %s || { rm -f -- %s; false; }; fi",
			qt, qt, qt, qst, qst)
		return head + " && " + preserve + fmt.Sprintf(" && rm -rf -- %s && mv -f -- %s %s", qt, qst, qt)
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
