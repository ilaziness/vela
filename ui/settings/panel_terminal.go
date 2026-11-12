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
	fontSize := strconv.Itoa(t.FontSize)
	scrollback := strconv.Itoa(t.Scrollback)
	lineHeight := strconv.FormatFloat(t.LineHeight, 'f', -1, 64)

	return panelColumn(i18n.T("settings.terminal.title"), []gui.View{
		formRow("settings-term-font", i18n.T("settings.terminal.font"), t.Font,
			i18n.T("settings.terminal.font_placeholder"), false, func(v string) { t.Font = v }),
		formRow("settings-term-font-size", i18n.T("settings.terminal.font_size"), fontSize,
			i18n.T("settings.general.font_size_placeholder"), false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					t.FontSize = n
				}
			}),
		formRow("settings-term-line-height", i18n.T("settings.terminal.line_height"), lineHeight,
			i18n.T("settings.terminal.line_height_placeholder"), false, func(v string) {
				if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					t.LineHeight = n
				}
			}),
		formRow("settings-term-scrollback", i18n.T("settings.terminal.scrollback"), scrollback,
			i18n.T("settings.terminal.scrollback_placeholder"), false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					t.Scrollback = n
				}
			}),
		switchRow("settings-term-bell", i18n.T("settings.terminal.bell_label"), t.Bell, func(gui.EventCtx) { t.Bell = !t.Bell }),
		switchRow("settings-term-copy", i18n.T("settings.terminal.copy_on_select_label"), t.CopyOnSelect, func(gui.EventCtx) {
			t.CopyOnSelect = !t.CopyOnSelect
		}),
		switchRow("settings-term-paste", i18n.T("settings.terminal.right_click_paste_label"), t.RightClickPaste, func(gui.EventCtx) {
			t.RightClickPaste = !t.RightClickPaste
		}),
		saveBtn("settings-term-save", i18n.T("common.save"), st.Busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			saveTerminal(ctx.Window, d)
		}),
	})
}

func saveTerminal(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	t := st.Terminal
	t.Normalize()
	if strings.TrimSpace(t.Font) == "" {
		w.Toast(gui.ToastCfg{Body: i18n.T("settings.terminal.font_required"), Severity: gui.ToastError})
		return
	}
	if t.FontSize < 1 || t.FontSize > 72 {
		w.Toast(gui.ToastCfg{
			Body:     i18n.Tf("settings.terminal.font_size_range", 1, 72),
			Severity: gui.ToastError,
		})
		return
	}
	if t.LineHeight <= 0 {
		w.Toast(gui.ToastCfg{Body: i18n.T("settings.terminal.line_height_positive"), Severity: gui.ToastError})
		return
	}
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
			d.Ctrl.SetTermDefaults(fonts.TermStyle(draft), draft.Scrollback)
			toastOK(win, i18n.T("settings.terminal.save_success"))
		})
	}()
}
