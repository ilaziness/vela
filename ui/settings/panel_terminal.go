package settings

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/fonts"
)

func panelTerminal(d *Deps, st *State) gui.View {
	t := &st.Terminal
	fontSize := ""
	if t.FontSize > 0 {
		fontSize = strconv.Itoa(t.FontSize)
	}
	scrollback := strconv.Itoa(t.Scrollback)

	body := []gui.View{
		gui.Combobox(gui.ComboboxCfg{
			ID:     "settings-term-theme",
			Label:  i18n.T("settings.terminal.theme"),
			Value:  d.Ctrl.TermThemeName(t.Theme),
			Items:  d.Ctrl.TermThemeItems(),
			Sizing: gui.FillFit,
			OnSelect: func(name string, ctx gui.EventCtx) {
				ctx.Consume()
				gui.State[State](ctx.Window).Terminal.Theme = name
			},
		}),
		formRow("settings-term-font", i18n.T("settings.terminal.font"), t.Font,
			i18n.T("settings.terminal.font_placeholder"), false, func(v string) { t.Font = v }),
		formRow("settings-term-font-size", i18n.T("settings.terminal.font_size"), fontSize,
			i18n.T("settings.terminal.font_size_placeholder"), false, func(v string) {
				v = strings.TrimSpace(v)
				if v == "" {
					t.FontSize = 0
					return
				}
				if n, err := strconv.Atoi(v); err == nil {
					t.FontSize = n
				}
			}),
		formRow("settings-term-scrollback", i18n.T("settings.terminal.scrollback"), scrollback,
			i18n.T("settings.terminal.scrollback_placeholder"), false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					t.Scrollback = n
				}
			}),
		gui.Select(gui.SelectCfg{
			ID:       "settings-term-cursor-style",
			Label:    i18n.T("settings.terminal.cursor_style"),
			Sizing:   gui.FillFit,
			Options:  cursorStyleOptions(),
			Selected: []string{cursorStyleValue(t.CursorStyle)},
			OnSelect: func(values []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(values) > 0 {
					gui.State[State](ctx.Window).Terminal.CursorStyle = values[0]
				}
			},
		}),
		switchRow("settings-term-cursor-blink", i18n.T("settings.terminal.cursor_blink"), t.CursorBlink, func(gui.EventCtx) {
			t.CursorBlink = !t.CursorBlink
		}),
		switchRow("settings-term-cursor-lock", i18n.T("settings.terminal.cursor_lock"), t.CursorLock, func(gui.EventCtx) {
			t.CursorLock = !t.CursorLock
		}),
		pathBrowseRow(
			"settings-term-record-dir",
			i18n.T("settings.terminal.record_dir"),
			t.RecordDir,
			i18n.T("settings.terminal.record_dir_placeholder"),
			i18n.T("dialog.select_dir"),
			func(ctx gui.EventCtx) {
				ctx.Consume()
				start := gui.State[State](ctx.Window).Terminal.RecordDir
				if start == "" {
					start = d.Store.LastDialogDir()
				}
				ctx.Window.NativeFolderDialog(gui.NativeFolderDialogCfg{
					Title:    i18n.T("settings.terminal.record_dir"),
					StartDir: start,
					OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
						if res.Status != gui.DialogOK || len(res.Paths) == 0 {
							return
						}
						d.Store.RememberDialogPath(res.Paths[0].Path)
						gui.State[State](win).Terminal.RecordDir = res.Paths[0].Path
					},
				})
			},
		),
		gui.Label(i18n.T("settings.terminal.record_dir_hint"), gui.CurrentTheme().TextStyleSecondary),
		switchRow("settings-term-record-input", i18n.T("settings.terminal.record_input"), t.RecordInput, func(gui.EventCtx) {
			t.RecordInput = !t.RecordInput
		}),
	}
	if t.RecordInput {
		body = append(body, gui.Label(i18n.T("settings.terminal.record_input_warning"), gui.CurrentTheme().TextStyleSecondary))
	}
	body = append(body, saveBtn("settings-term-save", i18n.T("common.save"), st.Busy, func(ctx gui.EventCtx) {
		ctx.Consume()
		saveTerminal(ctx.Window, d)
	}))

	return panelColumn(i18n.T("settings.terminal.title"), body)
}

// cursorStyleOptions lists the go-term cursor shapes; Values are the
// persisted strings, Labels are localized.
func cursorStyleOptions() []gui.SelectOption {
	return []gui.SelectOption{
		gui.NewSelectOption(i18n.T("settings.terminal.cursor_style_block"), "block"),
		gui.NewSelectOption(i18n.T("settings.terminal.cursor_style_underline"), "underline"),
		gui.NewSelectOption(i18n.T("settings.terminal.cursor_style_bar"), "bar"),
	}
}

// cursorStyleValue normalizes the stored value for the Select's Selected
// list so an empty/unknown config still shows the block cursor.
func cursorStyleValue(s string) string {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "underline", "bar":
		return v
	default:
		return "block"
	}
}

func saveTerminal(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	t := st.Terminal
	t.Normalize()
	t.Theme = d.Ctrl.TermThemeName(t.Theme)
	st.Busy = true
	draft := t
	go func() {
		d.Store.Config.Terminal = draft
		err := d.Store.Save()
		w.QueueCommand(func(win *gui.Window) {
			s := gui.State[State](win)
			s.Busy = false
			if err != nil {
				toastErr(win, err)
				return
			}
			s.Terminal = draft
			d.Ctrl.SetTermTheme(draft.Theme)
			d.Ctrl.SetTermConfig(draft)
			fonts.Apply(d.Store.Config)
			toastOK(win, i18n.T("settings.terminal.save_success"))
		})
	}()
}
