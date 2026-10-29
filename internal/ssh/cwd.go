package ssh

import (
	"net/url"
	"strings"
)

const (
	vteVersionValue = "7603"
	cwdOSCMaxBuf    = 8192
)

const (
	cwdStateNormal = iota
	cwdStateESC    // saw ESC, want ']' for OSC
	cwdStateOSC    // inside OSC body after ESC ]
	cwdStateOSCESC // inside OSC, saw ESC, want '\' for ST
)

// cwdParser extracts shell CWD reports from PTY stdout (OSC 7 / 1337 / 633).
// Incomplete sequences are kept across Feed calls. Data is not modified.
type cwdParser struct {
	state   int
	partial []byte // OSC body after ']', excluding terminators
}

// Feed scans data for CWD OSC sequences. Returns the last valid absolute path
// found in this chunk (empty if none).
func (p *cwdParser) Feed(data []byte) string {
	var last string
	for _, b := range data {
		switch p.state {
		case cwdStateNormal:
			if b == 0x1b {
				p.state = cwdStateESC
			}
		case cwdStateESC:
			switch b {
			case ']':
				p.state = cwdStateOSC
				p.partial = p.partial[:0]
			case 0x1b:
				// stay in ESC (consecutive ESC)
			default:
				p.state = cwdStateNormal
			}
		case cwdStateOSC:
			switch b {
			case 0x07: // BEL
				if path := parseOSCCwd(string(p.partial)); path != "" {
					last = path
				}
				p.reset()
			case 0x1b:
				p.state = cwdStateOSCESC
			default:
				p.partial = append(p.partial, b)
				if len(p.partial) > cwdOSCMaxBuf {
					p.reset()
				}
			}
		case cwdStateOSCESC:
			switch b {
			case '\\': // ST
				if path := parseOSCCwd(string(p.partial)); path != "" {
					last = path
				}
				p.reset()
			case 0x1b:
				// ESC ESC — still waiting for ST '\'
			case ']':
				// Malformed OSC; treat as new OSC start
				p.state = cwdStateOSC
				p.partial = p.partial[:0]
			default:
				p.reset()
			}
		}
	}
	return last
}

func (p *cwdParser) reset() {
	p.state = cwdStateNormal
	p.partial = p.partial[:0]
}

// parseOSCCwd parses the OSC payload after "ESC ]" (without terminators).
func parseOSCCwd(payload string) string {
	switch {
	case strings.HasPrefix(payload, "7;"):
		return parseOSC7(payload[2:])
	case strings.HasPrefix(payload, "1337;"):
		return parseOSC1337(payload[5:])
	case strings.HasPrefix(payload, "633;"):
		return parseOSC633(payload[4:])
	default:
		return ""
	}
}

func parseOSC7(uri string) string {
	const prefix = "file://"
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	rest := uri[len(prefix):]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return ""
	}
	pathPart := rest[slash:]
	decoded, err := url.PathUnescape(pathPart)
	if err != nil {
		decoded = pathPart
	}
	return absUnixPath(decoded)
}

func parseOSC1337(body string) string {
	for _, part := range strings.Split(body, ";") {
		if strings.HasPrefix(part, "CurrentDir=") {
			return absUnixPath(part[len("CurrentDir="):])
		}
	}
	return ""
}

func parseOSC633(body string) string {
	if !strings.HasPrefix(body, "P;") {
		return ""
	}
	rest := body[2:]
	if !strings.HasPrefix(rest, "Cwd=") {
		return ""
	}
	val := rest[len("Cwd="):]
	// Semicolon in the path is \x3b; a raw ';' starts the next property.
	if i := strings.IndexByte(val, ';'); i >= 0 {
		val = val[:i]
	}
	return absUnixPath(unescapeVSCode(val))
}

func absUnixPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || !strings.HasPrefix(p, "/") {
		return ""
	}
	return p
}

// unescapeVSCode decodes VS Code OSC 633 path escapes: \\ and \xAB.
func unescapeVSCode(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		if s[i+1] == '\\' {
			b.WriteByte('\\')
			i++
			continue
		}
		if s[i+1] == 'x' && i+3 < len(s) {
			h1, ok1 := fromHex(s[i+2])
			h2, ok2 := fromHex(s[i+3])
			if ok1 && ok2 {
				b.WriteByte(h1<<4 | h2)
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// shellStartCommand starts a login shell with cwd reporting via env (not stdin).
// VTE_VERSION helps vte.sh/fish; PROMPT_COMMAND covers bash without vte.sh.
// SSH exec env does not need AcceptEnv; nothing is typed → not in history.
func shellStartCommand() string {
	report := `printf "\033]7;file://%s%s\007" "${HOSTNAME:-}" "$PWD"`
	return "env VTE_VERSION=" + vteVersionValue +
		" PROMPT_COMMAND=" + bashSingleQuote(report) +
		` "${SHELL:-bash}" -l`
}

func (sc *Session) getCwd() string {
	sc.cwdMu.Lock()
	defer sc.cwdMu.Unlock()
	return sc.cwd
}

func (sc *Session) feedCwd(data []byte) {
	path := sc.cwdParser.Feed(data)
	if path == "" {
		return
	}
	sc.cwdMu.Lock()
	if sc.cwd == path {
		sc.cwdMu.Unlock()
		return
	}
	sc.cwd = path
	sc.cwdMu.Unlock()
	if sc.manager.onCwd != nil {
		sc.manager.onCwd(sc.ID, path)
	}
}
