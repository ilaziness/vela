package settings

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/fonts"
)

func panelGeneral(d *Deps, st *State) gui.View {
	theme := fonts.CanonicalThemeName(st.General.Theme)
	lang := strings.TrimSpace(st.General.Language)
	if lang == "" {
		lang = i18n.Language()
	}
	langOpts := []gui.SelectOption{
		{Label: i18n.T("settings.language.en"), Value: i18n.LangEn},
		{Label: i18n.T("settings.language.zh-CN"), Value: i18n.LangZhCN},
		{Label: i18n.T("settings.language.zh-TW"), Value: i18n.LangZhTW},
	}
	fontSize := ""
	if st.General.FontSize > 0 {
		fontSize = strconv.Itoa(st.General.FontSize)
	}

	return panelColumn(i18n.T("settings.general.title"), []gui.View{
		gui.Select(gui.SelectCfg{
			ID:       "settings-theme",
			Label:    i18n.T("settings.general.theme"),
			Selected: []string{theme},
			Items:    gui.ThemeRegisteredNames(),
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).General.Theme = vals[0]
				}
			},
		}),
		gui.Select(gui.SelectCfg{
			ID:       "settings-lang",
			Label:    i18n.T("settings.general.language"),
			Selected: []string{lang},
			Options:  langOpts,
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).General.Language = vals[0]
				}
			},
		}),
		formRow("settings-font", i18n.T("settings.general.font"), st.General.Font,
			i18n.T("settings.general.font_placeholder"), false, func(v string) {
				st.General.Font = v
			}),
		formRow("settings-font-size", i18n.T("settings.general.font_size"), fontSize,
			i18n.T("settings.general.font_size_placeholder"), false, func(v string) {
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err == nil {
					st.General.FontSize = n
				}
			}),
		pathBrowseRow(
			"settings-userdata",
			i18n.T("settings.general.user_data_dir"),
			st.General.UserDataDir,
			i18n.T("settings.general.user_data_dir_placeholder"),
			i18n.T("dialog.select_dir"),
			func(ctx gui.EventCtx) {
				ctx.Consume()
				start := gui.State[State](ctx.Window).General.UserDataDir
				if start == "" {
					start = d.Store.LastDialogDir()
				}
				ctx.Window.NativeFolderDialog(gui.NativeFolderDialogCfg{
					Title:    i18n.T("settings.general.user_data_dir"),
					StartDir: start,
					OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
						if res.Status != gui.DialogOK || len(res.Paths) == 0 {
							return
						}
						d.Store.RememberDialogPath(res.Paths[0].Path)
						gui.State[State](win).General.UserDataDir = res.Paths[0].Path
					},
				})
			},
		),
		switchRow("settings-frameless", i18n.T("settings.general.frameless"), st.General.Frameless, func(gui.EventCtx) {
			st.General.Frameless = !st.General.Frameless
		}),
		switchRow("settings-auto-check-update", i18n.T("settings.general.auto_check_update"), st.General.AutoCheckUpdate, func(gui.EventCtx) {
			st.General.AutoCheckUpdate = !st.General.AutoCheckUpdate
		}),
		saveBtn("settings-general-save", i18n.T("common.save"), st.Busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			saveGeneral(ctx.Window, d)
		}),
	})
}

func saveGeneral(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	g := st.General
	g.Normalize()
	if strings.TrimSpace(g.UserDataDir) == "" {
		w.Toast(gui.ToastCfg{Body: i18n.T("settings.general.user_data_dir_required"), Severity: gui.ToastError})
		return
	}
	if g.FontSize < 8 || g.FontSize > 72 {
		w.Toast(gui.ToastCfg{Body: i18n.T("settings.general.font_size_range"), Severity: gui.ToastError})
		return
	}
	g.Theme = fonts.CanonicalThemeName(g.Theme)
	st.Busy = true
	draft := g
	wasFrameless := d.Store.Config.General.Frameless
	go func() {
		err := persistGeneral(d, draft)
		w.QueueCommand(func(win *gui.Window) {
			s := gui.State[State](win)
			s.Busy = false
			if err != nil {
				toastErr(win, err)
				return
			}
			s.General = draft
			i18n.SetLanguage(draft.Language)
			app.ApplyGUILocale()
			fonts.Apply(d.Store.Config)
			refreshTitle(win)
			toastOK(win, i18n.T("settings.general.save_success"))
			if draft.Frameless != wasFrameless {
				// Decorations is fixed at window creation, so the
				// new frame style applies on next launch.
				win.Dialog(gui.DialogCfg{
					DialogType: gui.DialogMessage,
					Title:      i18n.T("settings.sync.restart_title"),
					Body:       i18n.T("dialog.frameless_restart"),
				})
			}
		})
	}()
}

func persistGeneral(d *Deps, draft config.GeneralConfig) error {
	if err := d.Store.ApplyUserDataDir(draft.UserDataDir); err != nil {
		return err
	}
	d.Store.Config.General = draft
	return d.Store.Save()
}
