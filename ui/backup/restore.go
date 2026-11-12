package backup

import (
	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/ui/layout"
)

type restoreState struct {
	items    []sync.BackupInfo
	selected string
	page     int
	hasMore  bool
	loading  bool
	loadErr  string
}

func openRestoreList(w *gui.Window, d *Deps) {
	st := &restoreState{hasMore: true}
	withVault(w, d, i18n.T("password.decrypt_sync"), func() {
		showRestoreList(w, d, st)
		loadRestorePage(w, d, st, 0)
	})
}

func showRestoreList(w *gui.Window, d *Deps, st *restoreState) {
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("settings.sync.restore_dialog_title"),
		Width:      520,
		Height:     480,
		CustomView: func(*gui.Window) gui.View {
			return restoreListView(d, st)
		},
	})
}

func restoreListView(d *Deps, st *restoreState) gui.View {
	th := gui.CurrentTheme()
	body := []gui.View{
		gui.Text(gui.TextCfg{
			Text:      i18n.T("settings.sync.restore_pick_backup"),
			TextStyle: th.TextStyleBody,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}),
	}

	switch {
	case st.loadErr != "" && len(st.items) == 0:
		body = append(body, gui.Text(gui.TextCfg{
			Text:      st.loadErr,
			TextStyle: th.TextStyleBody,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}))
	case len(st.items) == 0 && !st.loading:
		body = append(body, gui.Text(gui.TextCfg{
			Text:      i18n.T("settings.sync.no_backups"),
			TextStyle: th.TextStyleSecondary,
		}))
	default:
		if st.loadErr != "" {
			body = append(body, gui.Text(gui.TextCfg{
				Text:      st.loadErr,
				TextStyle: th.TextStyleBody,
				Mode:      gui.TextModeWrap,
				Sizing:    gui.FillFit,
			}))
		}
		rows := make([]gui.View, 0, len(st.items)+1)
		for _, b := range st.items {
			itemName := b.Name
			secondary := formatFileSize(b.FileSize) + " · " + formatDateTime(b.CreatedAt)
			bg := gui.Color{}
			if st.selected == itemName {
				bg = th.ColorActive
			}
			rows = append(rows, gui.Button(gui.ButtonCfg{
				ID:      "backup-item-" + itemName,
				Variant: gui.ButtonGhost,
				Sizing:  gui.FillFit,
				Color:   bg,
				HAlign:  gui.Some(gui.HAlignStart),
				Content: []gui.View{
					gui.Column(layout.Flush(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Spacing: gui.SpacingTight,
						HAlign:  gui.HAlignStart,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: itemName, TextStyle: th.TextStyleBody}),
							gui.Text(gui.TextCfg{Text: secondary, TextStyle: th.TextStyleSecondary}),
						},
					})),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					st.selected = itemName
				},
			}))
		}
		if st.loading {
			rows = append(rows, gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing: gui.FillFit,
				HAlign: gui.HAlignCenter,
				Content: []gui.View{
					gui.SvgSpinner(gui.SvgSpinnerCfg{
						Kind:   gui.SvgSpinner90Ring,
						Width:  24,
						Height: 24,
					}),
				},
			})))
		} else if st.hasMore && len(st.items) > 0 {
			rows = append(rows, gui.Button(gui.ButtonCfg{
				ID:      "backup-restore-more",
				Variant: gui.ButtonSecondary,
				Label:   i18n.T("settings.sync.load_more"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					loadRestorePage(ctx.Window, d, st, st.page+1)
				},
			}))
		} else if !st.hasMore && len(st.items) > 0 {
			rows = append(rows, gui.Text(gui.TextCfg{
				Text:      i18n.T("settings.sync.all_backups_loaded"),
				TextStyle: th.TextStyleSecondary,
			}))
		}
		body = append(body, gui.Column(layout.Flush(gui.ContainerCfg{
			ID:         "backup-restore-scroll",
			Sizing:     gui.FillFill,
			Scrollable: true,
			MaxHeight:  300,
			Spacing:    gui.SpacingTight,
			Content:    rows,
		})))
	}

	body = append(body, dialogButtons(
		gui.Button(gui.ButtonCfg{
			ID:      "backup-restore-cancel",
			Variant: gui.ButtonSecondary,
			Label:   i18n.T("common.cancel"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				ctx.Window.DialogDismiss()
				showChoose(ctx.Window, d)
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID:       "backup-restore-go",
			Variant:  gui.ButtonPrimary,
			Label:    i18n.T("settings.sync.restore"),
			Disabled: st.selected == "",
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				ctx.Window.DialogDismiss()
				showConfirmRestore(ctx.Window, d, st, st.selected)
			},
		}),
	))

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingMedium,
		Content: body,
	}))
}

func loadRestorePage(w *gui.Window, d *Deps, st *restoreState, page int) {
	if page > 0 && st.loading {
		return
	}
	st.loading = true
	st.loadErr = ""

	go func() {
		result, err := listBackups(d, backupPageSize, page*backupPageSize)
		w.QueueCommand(func(*gui.Window) {
			st.loading = false
			if err != nil {
				d.Logger.Error("failed to load backups", zap.Error(err))
				st.loadErr = i18n.Tf("settings.sync.load_backups_failed", errText(err))
				return
			}
			if page == 0 {
				st.items = result
			} else {
				st.items = append(st.items, result...)
			}
			st.hasMore = len(result) == backupPageSize
			st.page = page
		})
	}()
}

func showConfirmRestore(w *gui.Window, d *Deps, st *restoreState, name string) {
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("header.confirm_restore_title"),
		Width:      420,
		CustomView: func(*gui.Window) gui.View {
			th := gui.CurrentTheme()
			return gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      i18n.T("header.confirm_restore_message"),
						TextStyle: th.TextStyleBody,
						Mode:      gui.TextModeWrap,
						Sizing:    gui.FillFit,
					}),
					gui.Text(gui.TextCfg{
						Text:      name,
						TextStyle: th.TextStyleSecondary,
						Mode:      gui.TextModeWrap,
						Sizing:    gui.FillFit,
					}),
					dialogButtons(
						gui.Button(gui.ButtonCfg{
							ID:      "backup-confirm-restore-cancel",
							Variant: gui.ButtonSecondary,
							Label:   i18n.T("common.cancel"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								ctx.Window.DialogDismiss()
								showRestoreList(ctx.Window, d, st)
							},
						}),
						gui.Button(gui.ButtonCfg{
							ID:      "backup-confirm-restore-ok",
							Variant: gui.ButtonDanger,
							Label:   i18n.T("settings.sync.restore"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								ctx.Window.DialogDismiss()
								runRestore(ctx.Window, d, name)
							},
						}),
					),
				},
			}))
		},
	})
}

func runRestore(w *gui.Window, d *Deps, name string) {
	withVault(w, d, i18n.T("password.encrypt_decrypt_required"), func() {
		showBusy(w, i18n.T("header.restoring_backup"))
		go func() {
			err := downloadSync(d, name)
			w.QueueCommand(func(win *gui.Window) {
				win.DialogDismiss()
				if err != nil {
					d.Logger.Error("backup restore failed", zap.Error(err))
					toastErrMsg(win, i18n.Tf("settings.sync.download_failed", errText(err)))
					return
				}
				showRestart(win)
			})
		}()
	})
}

func showRestart(w *gui.Window) {
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogMessage,
		Title:      i18n.T("settings.sync.restart_title"),
		Body:       i18n.T("settings.sync.restart_body"),
	})
}
