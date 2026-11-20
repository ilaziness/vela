package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/keybind"
	"github.com/ilaziness/vela/ui/app"
	keybindui "github.com/ilaziness/vela/ui/keybind"
	"github.com/ilaziness/vela/ui/shell"
)

// keybindCommandID prefixes every shortcut-driven command so reinstalling
// the table can unregister the previous generation by ID.
const keybindCommandID = "keybind:"

// installKeybindCommands (re)builds the main window's global shortcut
// commands from the current keybind table. Panel/app actions dispatch
// here because they must fire before focus routing; terminal find/copy/
// paste stay inside go-term via Controller.ApplyKeyBindings instead.
// Safe to call repeatedly: each action's old command is unregistered
// before the new one is registered.
func installKeybindCommands(w *gui.Window, rt *app.Runtime) {
	if rt == nil || rt.Keybind == nil || rt.Ctrl == nil {
		return
	}
	bindings := rt.Keybind.Bindings()
	labels := make(map[string]string, len(bindings))
	for _, info := range rt.Keybind.List() {
		labels[info.Action] = info.Label
	}
	specs := []struct {
		action string
		exec   func(*gui.Event, *gui.Window)
		can    func(*gui.Window) bool
	}{
		{
			action: keybind.TerminalToggleLog,
			exec:   func(_ *gui.Event, w *gui.Window) { rt.Ctrl.ToggleActiveRecording(w) },
			can:    hasActiveSession,
		},
		{
			action: keybind.PanelAI,
			exec:   func(_ *gui.Event, w *gui.Window) { rt.Ctrl.AgentToggle(w) },
		},
		{
			action: keybind.PanelQuickConnect,
			exec:   func(_ *gui.Event, w *gui.Window) { shell.ToggleQuickConnect(w, rt.Ctrl) },
		},
	}
	for _, s := range specs {
		id := keybindCommandID + s.action
		w.UnregisterCommand(id)
		cmd := gui.Command{
			ID:         id,
			Label:      labels[s.action],
			Execute:    s.exec,
			CanExecute: s.can,
			Global:     true,
		}
		if logical, ok := bindings[s.action]; ok {
			if sc, ok := keybindui.ShortcutFor(logical); ok {
				cmd.Shortcut = sc
			}
		}
		if err := w.RegisterCommand(cmd); err != nil {
			rt.Logger.Warn("keybind command register failed",
				zap.String("id", id), zap.Error(err))
		}
	}
}

// hasActiveSession gates terminal.toggleLog: recording needs a live
// session tab, and the command should not swallow the chord otherwise.
func hasActiveSession(w *gui.Window) bool {
	st := gui.State[app.State](w)
	t := st.Active()
	return t != nil && t.Kind == app.TabSession && t.SessID != ""
}

// watchKeybindChanges reapplies shortcuts whenever the settings page
// saves a new binding: global commands are rebuilt on the main window
// and every live terminal gets the new term.KeyMap.
func watchKeybindChanges(w *gui.Window, rt *app.Runtime) {
	if rt == nil || rt.Keybind == nil {
		return
	}
	rt.Keybind.SetOnChange(func() {
		w.QueueCommand(func(ww *gui.Window) {
			installKeybindCommands(ww, rt)
			rt.Ctrl.ApplyKeyBindings()
		})
	})
}
