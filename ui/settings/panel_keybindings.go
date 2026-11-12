package settings

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

// Minimum column widths; action label (index 0) absorbs leftover width.
var keybindColMin = []float32{300, 160, 80}

func panelKeybindings(w *gui.Window, d *Deps, st *State) gui.View {
	cols := resolveColWidths(w, keybindColMin, 0)
	rows := []gui.TableRowCfg{
		gui.TR([]gui.TableCellCfg{
			thW(i18n.T("settings.keybindings.col_action"), cols[0]),
			thW(i18n.T("settings.keybindings.col_shortcut"), cols[1]),
			thW(i18n.T("settings.keybindings.col_reset"), cols[2]),
		}),
	}
	for i := range st.Keybindings {
		kb := st.Keybindings[i]
		action := kb.Action
		custom := kb.Custom
		keysLabel := kb.Keys
		if st.RecordingAction == action {
			keysLabel = i18n.T("settings.keybindings.press_shortcut")
		} else if custom {
			keysLabel = keysLabel + " (" + i18n.T("settings.keybindings.customized") + ")"
		}
		actionLabel := kb.Label
		if kb.Category != "" {
			actionLabel = kb.Label + " · " + kb.Category
		}
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(actionLabel, cols[0]),
			clickCellW(keysLabel, cols[1], func(ctx gui.EventCtx) {
				ctx.Consume()
				gui.State[State](ctx.Window).RecordingAction = action
			}),
			actionCellW(gui.Button(gui.ButtonCfg{
				ID:       "settings-kb-reset-" + action,
				Variant:  gui.ButtonGhost,
				Label:    i18n.T("settings.keybindings.reset_one"),
				Disabled: !custom,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					if err := d.Keybind.Reset(action); err != nil {
						toastErr(ctx.Window, err)
						return
					}
					s := gui.State[State](ctx.Window)
					s.RecordingAction = ""
					refreshLists(s, d)
					toastOK(ctx.Window, i18n.T("settings.keybindings.reset_one_ok"))
				},
			}), cols[2]),
		}))
	}

	return panelColumn(i18n.T("settings.nav.keybindings"), []gui.View{
		gui.Label(i18n.T("settings.keybindings.hint"), gui.CurrentTheme().TextStyleSecondary),
		gui.Button(gui.ButtonCfg{
			ID:      "settings-kb-reset-all",
			Variant: gui.ButtonSecondary,
			Label:   i18n.T("settings.keybindings.reset_all"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				if err := d.Keybind.ResetAll(); err != nil {
					toastErr(ctx.Window, err)
					return
				}
				s := gui.State[State](ctx.Window)
				refreshLists(s, d)
				toastOK(ctx.Window, i18n.T("settings.keybindings.reset_all_ok"))
			},
		}),
		settingsTable("settings-keybindings-table", rows),
	})
}

func handleKeybindCapture(e *gui.Event, w *gui.Window, d *Deps) {
	if e.Type != gui.EventKeyDown {
		return
	}
	st := gui.State[State](w)
	action := st.RecordingAction
	if action == "" {
		return
	}
	if e.KeyCode == gui.KeyEscape {
		st.RecordingAction = ""
		return
	}
	logical, ok := eventToLogical(e)
	if !ok {
		return
	}
	st.RecordingAction = ""
	if err := d.Keybind.Set(action, logical); err != nil {
		toastErr(w, err)
		return
	}
	refreshLists(st, d)
	toastOK(w, i18n.T("settings.keybindings.updated"))
}

func eventToLogical(e *gui.Event) (string, bool) {
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
	case gui.KeyF1:
		return "F1"
	case gui.KeyF2:
		return "F2"
	case gui.KeyF3:
		return "F3"
	case gui.KeyF4:
		return "F4"
	case gui.KeyF5:
		return "F5"
	case gui.KeyF6:
		return "F6"
	case gui.KeyF7:
		return "F7"
	case gui.KeyF8:
		return "F8"
	case gui.KeyF9:
		return "F9"
	case gui.KeyF10:
		return "F10"
	case gui.KeyF11:
		return "F11"
	case gui.KeyF12:
		return "F12"
	}
	if k >= gui.KeyA && k <= gui.KeyZ {
		return string(rune('A' + (k - gui.KeyA)))
	}
	if k >= gui.Key0 && k <= gui.Key9 {
		return string(rune('0' + (k - gui.Key0)))
	}
	return ""
}
