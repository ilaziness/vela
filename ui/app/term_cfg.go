package app

import (
	"strings"

	"github.com/go-gui-org/go-term/term"
)

// cursorStyleOf maps the persisted cursor_style string onto go-term's
// CursorStyle. Unknown values fall back to the block cursor, matching
// config.Normalize.
func cursorStyleOf(style string) term.CursorStyle {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "underline":
		return term.CursorStyleUnderline
	case "bar":
		return term.CursorStyleBar
	default:
		return term.CursorStyleBlock
	}
}
