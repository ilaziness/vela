package keybind

import (
	"runtime"

	"github.com/ilaziness/vela/internal/i18n"
)

// Action IDs (stable config keys).
const (
	TerminalFind      = "terminal.find"
	TerminalCopy      = "terminal.copy"
	TerminalPaste     = "terminal.paste"
	TerminalToggleLog = "terminal.toggleLog"
	PanelAI           = "panel.ai"
	PanelQuickConnect = "panel.quickConnect"
	PanelCommand      = "panel.command"
)

const (
	categoryTerminal = "keybinding.category.terminal"
	categoryPanel    = "keybinding.category.panel"
)

// Info is one binding for settings UI.
type Info struct {
	Action      string
	Keys        string
	Logical     string
	DefaultKeys string
	Custom      bool
	Label       string
	Category    string
}

type def struct {
	action      string
	logical     string
	labelKey    string
	categoryKey string
}

func builtin() []def {
	// Copy/paste keep the platform's primary chord: macOS users expect
	// Cmd+C/Cmd+V, everywhere else the Shift forms avoid stealing
	// Ctrl+C/Ctrl+V from the child process.
	copyLogical, pasteLogical := "CmdOrCtrl+Shift+C", "CmdOrCtrl+Shift+V"
	findLogical := "CmdOrCtrl+Shift+F"
	if runtime.GOOS == "darwin" {
		// macOS keeps the bare Cmd chords; elsewhere the Shift forms
		// avoid stealing control characters (^F, ^C) from the child.
		copyLogical, pasteLogical = "CmdOrCtrl+C", "CmdOrCtrl+V"
		findLogical = "CmdOrCtrl+F"
	}
	return []def{
		{TerminalFind, findLogical, "keybinding.terminal.find", categoryTerminal},
		{TerminalCopy, copyLogical, "keybinding.terminal.copy", categoryTerminal},
		{TerminalPaste, pasteLogical, "keybinding.terminal.paste", categoryTerminal},
		{TerminalToggleLog, "CmdOrCtrl+Shift+Y", "keybinding.terminal.toggleLog", categoryTerminal},
		{PanelAI, "CmdOrCtrl+Shift+J", "keybinding.panel.ai", categoryPanel},
		{PanelQuickConnect, "CmdOrCtrl+B", "keybinding.panel.quickConnect", categoryPanel},
		{PanelCommand, "CmdOrCtrl+Shift+P", "keybinding.panel.command", categoryPanel},
	}
}

func defMap() map[string]def {
	out := make(map[string]def, 16)
	for _, d := range builtin() {
		out[d.action] = d
	}
	return out
}

func localized(d def, logical string) Info {
	custom := normalizeAccel(logical) != normalizeAccel(d.logical)
	return Info{
		Action:      d.action,
		Keys:        displayKeys(logical),
		Logical:     logical,
		DefaultKeys: displayKeys(d.logical),
		Custom:      custom,
		Label:       i18n.T(d.labelKey),
		Category:    i18n.T(d.categoryKey),
	}
}
