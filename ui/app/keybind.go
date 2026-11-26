package app

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-term/term"

	"github.com/ilaziness/vela/internal/keybind"
	keybindui "github.com/ilaziness/vela/ui/keybind"
)

// SetKeybind attaches the shortcut manager; called once during bootstrap
// before any window exists.
func (c *Controller) SetKeybind(kb *keybind.Manager) {
	c.mu.Lock()
	c.keybindMgr = kb
	c.mu.Unlock()
}

// termKeyMap translates the settings keybindings that go-term owns
// (find/copy/paste) into its KeyMap override table. Panel actions are
// dispatched as global gui.Commands instead.
func (c *Controller) termKeyMap() term.KeyMap {
	c.mu.Lock()
	kb := c.keybindMgr
	c.mu.Unlock()
	if kb == nil {
		return nil
	}
	bindings := kb.Bindings()
	km := make(term.KeyMap, 3)
	for action, logical := range bindings {
		var ta term.Action
		switch action {
		case keybind.TerminalFind:
			ta = term.ActionFind
		case keybind.TerminalCopy:
			ta = term.ActionCopy
		case keybind.TerminalPaste:
			ta = term.ActionPaste
		default:
			continue
		}
		sc, ok := keybindui.ShortcutFor(logical)
		if !ok {
			continue
		}
		km[ta] = sc
	}
	return km
}

// KeybindKeys returns the display shortcut for a keybind action (the same
// text the settings page shows), or "" when no manager is attached or the
// action is unknown.
func (c *Controller) KeybindKeys(action string) string {
	c.mu.Lock()
	kb := c.keybindMgr
	c.mu.Unlock()
	if kb == nil {
		return ""
	}
	return kb.Keys(action)
}

// ApplyKeyBindings pushes the current shortcut table into every live
// terminal. Main-thread only: Term.SetKeyBindings must run on the UI thread.
func (c *Controller) ApplyKeyBindings() {
	km := c.termKeyMap()
	c.mu.Lock()
	terms := make([]*term.Term, 0, len(c.terms))
	for _, tm := range c.terms {
		if tm != nil {
			terms = append(terms, tm)
		}
	}
	c.mu.Unlock()
	for _, tm := range terms {
		tm.SetKeyBindings(km)
	}
}

// ToggleActiveRecording starts/stops a session recording on the active
// session tab's terminal. Invoked by the global keybind command.
func (c *Controller) ToggleActiveRecording(w *gui.Window) {
	st := gui.State[State](w)
	t := st.Active()
	if t == nil || t.Kind != TabSession || t.SessID == "" {
		return
	}
	c.ToggleSessionRecording(w, t.SessID)
}
