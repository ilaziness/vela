package keybind

import (
	"runtime"
	"strings"
	"unicode"

	"github.com/ilaziness/vela/internal/usererr"
)

func canonicalizeLogical(logical string) string {
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return logical
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	hasPrimary, hasShift, hasAlt := false, false, false
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmdorctrl", "cmd", "command", "ctrl", "control":
			hasPrimary = true
		case "shift":
			hasShift = true
		case "alt", "option":
			hasAlt = true
		}
	}
	named := map[string]string{
		",": ",", "f1": "F1", "f2": "F2", "f3": "F3", "f4": "F4",
		"f5": "F5", "f6": "F6", "f7": "F7", "f8": "F8", "f9": "F9",
		"f10": "F10", "f11": "F11", "f12": "F12",
		"escape": "Escape", "enter": "Enter", "space": "Space",
		"tab": "Tab", "backspace": "Backspace", "delete": "Delete",
		"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	}
	if v, ok := named[strings.ToLower(key)]; ok {
		key = v
	} else {
		key = strings.ToUpper(key)
	}
	var out []string
	if hasPrimary {
		out = append(out, "CmdOrCtrl")
	}
	if hasShift {
		out = append(out, "Shift")
	}
	if hasAlt {
		out = append(out, "Alt")
	}
	if len(out) == 0 {
		return key
	}
	return strings.Join(append(out, key), "+")
}

func isNamedKey(key string) bool {
	switch strings.ToLower(key) {
	case ",", "escape", "enter", "space", "tab", "backspace", "delete",
		"up", "down", "left", "right",
		"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
		return true
	}
	return false
}

func validateLogical(logical string) error {
	logical = strings.TrimSpace(logical)
	if logical == "" {
		return usererr.Coded("keybinding.empty", "Shortcut cannot be empty")
	}
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return usererr.Coded("keybinding.invalid", "Invalid shortcut")
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if key == "" {
		return usererr.Coded("keybinding.invalid", "Invalid shortcut")
	}
	hasPrimary, hasAlt := false, false
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmdorctrl", "cmd", "command", "ctrl", "control":
			hasPrimary = true
		case "alt", "option":
			hasAlt = true
		case "shift":
		default:
			return usererr.Coded("keybinding.unsupported_modifier", "Unsupported modifier: %s", p)
		}
	}
	isFunc := len(key) >= 2 && (key[0] == 'F' || key[0] == 'f')
	if isFunc {
		rest := key[1:]
		allDigit := rest != ""
		for _, r := range rest {
			if !unicode.IsDigit(r) {
				allDigit = false
				break
			}
		}
		if allDigit {
			return nil
		}
	}
	if isNamedKey(key) && strings.EqualFold(key, "Escape") {
		return usererr.Coded("keybinding.no_escape", "Cannot bind Escape")
	}
	if !hasPrimary && !hasAlt && !isFunc {
		return usererr.Coded("keybinding.need_modifier", "Shortcut must include Ctrl/Cmd or Alt, or be a function key")
	}
	if len(key) == 1 {
		r := rune(key[0])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || key == "," {
			return nil
		}
	}
	if isNamedKey(key) {
		return nil
	}
	if (hasPrimary || hasAlt) && len(key) >= 1 {
		return nil
	}
	return usererr.Coded("keybinding.unsupported_key", "Unsupported key: %s", key)
}

func normalizeAccel(logical string) string {
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return logical
	}
	key := parts[len(parts)-1]
	modSet := map[string]struct{}{}
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "cmd", "command":
			if runtime.GOOS == "darwin" {
				modSet["Cmd"] = struct{}{}
			} else {
				modSet["Ctrl"] = struct{}{}
			}
		case "ctrl", "control":
			modSet["Ctrl"] = struct{}{}
		case "shift":
			modSet["Shift"] = struct{}{}
		case "alt", "option":
			if runtime.GOOS == "darwin" {
				modSet["Option"] = struct{}{}
			} else {
				modSet["Alt"] = struct{}{}
			}
		}
	}
	order := []string{"Alt", "Cmd", "Ctrl", "Option", "Shift", "Win"}
	var mods []string
	for _, m := range order {
		if _, ok := modSet[m]; ok {
			mods = append(mods, m)
		}
	}
	key = strings.ToUpper(key)
	if len(mods) == 0 {
		return key
	}
	return strings.Join(append(mods, key), "+")
}

func displayKeys(logical string) string {
	isMac := runtime.GOOS == "darwin"
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return logical
	}
	key := parts[len(parts)-1]
	var out []string
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "cmd", "command":
			if isMac {
				out = append(out, "⌘")
			} else {
				out = append(out, "Ctrl")
			}
		case "ctrl", "control":
			if isMac {
				out = append(out, "⌃")
			} else {
				out = append(out, "Ctrl")
			}
		case "shift":
			if isMac {
				out = append(out, "⇧")
			} else {
				out = append(out, "Shift")
			}
		case "alt", "option":
			if isMac {
				out = append(out, "⌥")
			} else {
				out = append(out, "Alt")
			}
		default:
			out = append(out, p)
		}
	}
	if isMac {
		return strings.Join(append(out, strings.ToUpper(key)), "")
	}
	return strings.Join(append(out, strings.ToUpper(key)), "+")
}
