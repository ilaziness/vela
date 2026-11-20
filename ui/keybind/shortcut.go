// Package keybind converts between the settings page's logical shortcut
// strings and go-gui/go-term shortcut values.
package keybind

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// EventLogical renders a key-down event as the same logical string the
// settings page records (for example "CmdOrCtrl+Shift+C"). Modifier-only
// presses and Escape are ignored so they cannot be bound.
func EventLogical(e *gui.Event) (string, bool) {
	if e == nil {
		return "", false
	}
	switch e.KeyCode {
	case gui.KeyLeftControl, gui.KeyRightControl, gui.KeyLeftShift, gui.KeyRightShift,
		gui.KeyLeftAlt, gui.KeyRightAlt, gui.KeyLeftSuper, gui.KeyRightSuper,
		gui.KeyEscape:
		return "", false
	}
	key := keyCodeName(e.KeyCode)
	if key == "" {
		return "", false
	}
	var parts []string
	mods := e.Modifiers
	if mods.HasAny(gui.ModCtrl, gui.ModSuper) {
		parts = append(parts, "CmdOrCtrl")
	}
	if mods.Has(gui.ModShift) {
		parts = append(parts, "Shift")
	}
	if mods.Has(gui.ModAlt) {
		parts = append(parts, "Alt")
	}
	parts = append(parts, key)
	return strings.Join(parts, "+"), true
}

// ShortcutFor parses a logical shortcut string into a gui.Shortcut.
// CmdOrCtrl follows the platform convention used by the rest of the app:
// Command on macOS, Ctrl elsewhere.
func ShortcutFor(logical string) (gui.Shortcut, bool) {
	logical = strings.TrimSpace(logical)
	if logical == "" {
		return gui.Shortcut{}, false
	}
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return gui.Shortcut{}, false
	}
	keyName := strings.TrimSpace(parts[len(parts)-1])
	key, ok := keyCodeFor(keyName)
	if !ok {
		return gui.Shortcut{}, false
	}
	var mods gui.Modifier
	primary, shift, alt := false, false, false
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmdorctrl", "cmd", "command", "ctrl", "control":
			primary = true
		case "shift":
			shift = true
		case "alt", "option":
			alt = true
		default:
			return gui.Shortcut{}, false
		}
	}
	if primary {
		if runtime.GOOS == "darwin" {
			mods |= gui.ModSuper
		} else {
			mods |= gui.ModCtrl
		}
	}
	if shift {
		mods |= gui.ModShift
	}
	if alt {
		mods |= gui.ModAlt
	}
	return gui.Shortcut{Key: key, Modifiers: mods}, true
}

func keyCodeName(k gui.KeyCode) string {
	switch k {
	case gui.KeyComma:
		return ","
	case gui.KeySpace:
		return "Space"
	case gui.KeyEnter:
		return "Enter"
	case gui.KeyTab:
		return "Tab"
	case gui.KeyBackspace:
		return "Backspace"
	case gui.KeyDelete:
		return "Delete"
	case gui.KeyUp:
		return "Up"
	case gui.KeyDown:
		return "Down"
	case gui.KeyLeft:
		return "Left"
	case gui.KeyRight:
		return "Right"
	}
	if k >= gui.KeyF1 && k <= gui.KeyF25 {
		return "F" + strconv.Itoa(int(k-gui.KeyF1)+1)
	}
	// Printable ASCII keys carry their character code, so letters,
	// digits and punctuation all resolve through the same range.
	if k >= '!' && k <= '~' {
		return string(rune(k))
	}
	return ""
}

func keyCodeFor(name string) (gui.KeyCode, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case ",":
		return gui.KeyComma, true
	case "space":
		return gui.KeySpace, true
	case "enter":
		return gui.KeyEnter, true
	case "tab":
		return gui.KeyTab, true
	case "backspace":
		return gui.KeyBackspace, true
	case "delete":
		return gui.KeyDelete, true
	case "up":
		return gui.KeyUp, true
	case "down":
		return gui.KeyDown, true
	case "left":
		return gui.KeyLeft, true
	case "right":
		return gui.KeyRight, true
	}
	if len(name) >= 2 && (name[0] == 'f' || name[0] == 'F') {
		if n, err := strconv.Atoi(name[1:]); err == nil && n >= 1 && n <= 25 {
			return gui.KeyF1 + gui.KeyCode(n-1), true
		}
	}
	if len(name) == 1 {
		c := name[0]
		if c >= '!' && c <= '~' {
			return gui.KeyCode(c), true
		}
	}
	return gui.KeyInvalid, false
}
