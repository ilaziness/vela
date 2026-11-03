package command

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var (
	ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)|\x1b.`)
	// Match last-line prompts like "Password:", "[sudo] password for user:", "请输入密码："
	passwordPromptRe = regexp.MustCompile(`(?i)(password|passphrase|密码|口令)`)
)

// LineCapture assembles submitted shell lines from a raw PTY stdin byte stream
// and skips lines typed after a password-like prompt in recent PTY output.
// Safe for concurrent FeedInput (WS read) and FeedOutput (WS write).
type LineCapture struct {
	mu            sync.Mutex
	buf           []byte
	esc           bool
	csi           bool // ESC [ ... final (stdin)
	osc           bool // ESC ] ... BEL/ST (stdin)
	secretPending bool
	lastOutLine   string // last evaluated prompt line; only re-arm secret when it changes
	recentOut     []byte
	altScreen     bool
	// Incomplete DEC private mode / CSI from prior FeedOutput chunk.
	outCSIPend []byte
}

const maxRecentOut = 4096
const maxOutCSIPend = 64

// FeedOutput records recent PTY output and updates password-prompt / alt-screen detection.
func (c *LineCapture) FeedOutput(data []byte) {
	if len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scanAltScreen(data)
	c.recentOut = append(c.recentOut, data...)
	if len(c.recentOut) > maxRecentOut {
		c.recentOut = append([]byte(nil), c.recentOut[len(c.recentOut)-maxRecentOut:]...)
	}
	// TUI redraws must not arm password skipping (e.g. file content containing "password").
	if c.altScreen {
		return
	}
	plain := stripANSI(string(c.recentOut))
	plain = strings.ReplaceAll(plain, "\r", "\n")
	last := plain
	if i := strings.LastIndex(plain, "\n"); i >= 0 {
		last = plain[i+1:]
	}
	last = strings.TrimRight(last, " \t")
	if last == c.lastOutLine {
		return
	}
	c.lastOutLine = last
	// New last line: arm when it looks like a password prompt, else clear sticky flag.
	c.secretPending = passwordPromptRe.MatchString(last)
}

// scanAltScreen updates altScreen from DEC private mode sequences (47/1047/1049).
// Handles CSI split across FeedOutput calls via outCSIPend.
func (c *LineCapture) scanAltScreen(data []byte) {
	buf := data
	if len(c.outCSIPend) > 0 {
		combined := make([]byte, 0, len(c.outCSIPend)+len(data))
		combined = append(combined, c.outCSIPend...)
		combined = append(combined, data...)
		buf = combined
		c.outCSIPend = nil
	}
	i := 0
	for i < len(buf) {
		if buf[i] != 0x1b {
			i++
			continue
		}
		if i+1 >= len(buf) {
			c.outCSIPend = append([]byte(nil), buf[i:]...)
			return
		}
		if buf[i+1] != '[' {
			i++
			continue
		}
		j := i + 2
		for j < len(buf) {
			b := buf[j]
			if b >= 0x40 && b <= 0x7e {
				c.applyDECPrivate(buf[i : j+1])
				i = j + 1
				break
			}
			if b < 0x20 && b != 0x1b {
				i = j
				break
			}
			j++
		}
		if j >= len(buf) {
			pend := buf[i:]
			if len(pend) > maxOutCSIPend {
				pend = pend[len(pend)-maxOutCSIPend:]
			}
			c.outCSIPend = append([]byte(nil), pend...)
			return
		}
	}
}

func (c *LineCapture) applyDECPrivate(seq []byte) {
	// Expect ESC [ ? <params> h|l
	if len(seq) < 5 || seq[0] != 0x1b || seq[1] != '[' {
		return
	}
	final := seq[len(seq)-1]
	if final != 'h' && final != 'l' {
		return
	}
	body := seq[2 : len(seq)-1]
	if len(body) == 0 || body[0] != '?' {
		return
	}
	enter := final == 'h'
	changed := false
	for _, p := range strings.Split(string(body[1:]), ";") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		switch n {
		case 47, 1047, 1049:
			if c.altScreen != enter {
				c.altScreen = enter
				changed = true
			}
		}
	}
	if changed {
		c.buf = c.buf[:0]
		c.secretPending = false
	}
}

// FeedInput consumes stdin bytes and returns completed non-secret command lines.
func (c *LineCapture) FeedInput(data []byte) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for i := 0; i < len(data); {
		b := data[i]

		if c.osc {
			if b == 0x07 { // BEL
				c.osc = false
				c.esc = false
			} else if b == '\\' && i > 0 && data[i-1] == 0x1b {
				c.osc = false
				c.esc = false
			}
			i++
			continue
		}
		if c.csi {
			if b >= 0x40 && b <= 0x7e {
				c.csi = false
				c.esc = false
			}
			i++
			continue
		}
		if c.esc {
			if b == '[' {
				c.csi = true
				i++
				continue
			}
			if b == ']' {
				c.osc = true
				i++
				continue
			}
			// Other ESC sequences: consume one following byte.
			c.esc = false
			i++
			continue
		}

		if b == 0x1b {
			c.esc = true
			i++
			continue
		}

		// TUI alternate screen: keep escape state machine, do not record.
		if c.altScreen {
			i++
			continue
		}

		switch b {
		case '\r', '\n':
			line := strings.TrimSpace(string(c.buf))
			c.buf = c.buf[:0]
			if c.secretPending {
				c.secretPending = false
			} else if line != "" {
				out = append(out, line)
			}
			i++
			// Treat CRLF as one line end.
			if b == '\r' && i < len(data) && data[i] == '\n' {
				i++
			}
		case 0x08, 0x7f: // BS / DEL
			c.buf = deleteLastRune(c.buf)
			i++
		case 0x15: // Ctrl+U
			c.buf = c.buf[:0]
			i++
		case 0x17: // Ctrl+W
			c.buf = deleteLastWord(c.buf)
			i++
		case 0x03: // Ctrl+C
			c.buf = c.buf[:0]
			c.secretPending = false
			i++
		default:
			if b < 0x20 {
				i++
				continue
			}
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 {
				i++
				continue
			}
			if unicode.IsPrint(r) || r == '\t' {
				c.buf = append(c.buf, data[i:i+size]...)
			}
			i += size
		}
	}
	return out
}

func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

func deleteLastRune(buf []byte) []byte {
	if len(buf) == 0 {
		return buf
	}
	_, size := utf8.DecodeLastRune(buf)
	return buf[:len(buf)-size]
}

func deleteLastWord(buf []byte) []byte {
	s := string(buf)
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	i := len(s)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if unicode.IsSpace(r) {
			break
		}
		i -= size
	}
	return []byte(s[:i])
}
