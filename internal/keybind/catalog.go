package keybind

import "github.com/ilaziness/vela/internal/i18n"

// Action IDs (stable config keys).
const (
	TerminalFind         = "terminal.find"
	TerminalCopy         = "terminal.copy"
	TerminalPaste        = "terminal.paste"
	TerminalAddToChat    = "terminal.addToChat"
	TerminalToggleLog    = "terminal.toggleLog"
	TerminalClear        = "terminal.clear"
	PanelAI              = "panel.ai"
	PanelBookmarks       = "panel.bookmarks"
	PanelBookmarkConnect = "panel.bookmarkConnect"
	PanelCommand         = "panel.command"
	PanelTools           = "panel.tools"
	PanelSettings        = "panel.settings"
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
	return []def{
		{TerminalFind, "CmdOrCtrl+F", "keybinding.terminal.find", categoryTerminal},
		{TerminalCopy, "CmdOrCtrl+Shift+C", "keybinding.terminal.copy", categoryTerminal},
		{TerminalPaste, "CmdOrCtrl+Shift+V", "keybinding.terminal.paste", categoryTerminal},
		{TerminalAddToChat, "CmdOrCtrl+Shift+L", "keybinding.terminal.addToChat", categoryTerminal},
		{TerminalToggleLog, "CmdOrCtrl+Shift+Y", "keybinding.terminal.toggleLog", categoryTerminal},
		{TerminalClear, "CmdOrCtrl+Shift+K", "keybinding.terminal.clear", categoryTerminal},
		{PanelAI, "CmdOrCtrl+Shift+A", "keybinding.panel.ai", categoryPanel},
		{PanelBookmarkConnect, "CmdOrCtrl+B", "keybinding.panel.bookmarkConnect", categoryPanel},
		{PanelBookmarks, "CmdOrCtrl+Shift+B", "keybinding.panel.bookmarks", categoryPanel},
		{PanelCommand, "CmdOrCtrl+Shift+P", "keybinding.panel.command", categoryPanel},
		{PanelTools, "CmdOrCtrl+Shift+U", "keybinding.panel.tools", categoryPanel},
		{PanelSettings, "CmdOrCtrl+,", "keybinding.panel.settings", categoryPanel},
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
