package settings

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	keybindui "github.com/ilaziness/vela/ui/keybind"
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
	logical, ok := keybindui.EventLogical(e)
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
