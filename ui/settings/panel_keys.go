package settings

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/ui/layout"
)

// Minimum column widths; name (0), fingerprint (2) and comment (3)
// share leftover width proportionally.
var keysColMin = []float32{130, 88, 320, 72, 128, 300}

func panelKeys(w *gui.Window, d *Deps, st *State) gui.View {
	cols := resolveColWidths(w, keysColMin, 0, 2, 3)
	algoOpts := []gui.SelectOption{
		{Label: "Ed25519", Value: "ed25519"},
		{Label: "ECDSA P-256", Value: "ecdsa-p256"},
		{Label: "RSA 4096", Value: "rsa-4096"},
	}

	rows := []gui.TableRowCfg{
		gui.TR([]gui.TableCellCfg{
			thW(i18n.T("settings.keys.col_name"), cols[0]),
			thW(i18n.T("settings.keys.col_algorithm"), cols[1]),
			thW(i18n.T("settings.keys.col_fingerprint"), cols[2]),
			thW(i18n.T("settings.keys.col_comment"), cols[3]),
			thW(i18n.T("settings.keys.col_created"), cols[4]),
			thW(i18n.T("settings.keys.col_actions"), cols[5]),
		}),
	}
	if len(st.Keys) == 0 {
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(i18n.T("settings.keys.empty"), cols[0]),
			tdW("", cols[1]),
			tdW("", cols[2]),
			tdW("", cols[3]),
			tdW("", cols[4]),
			tdW("", cols[5]),
		}))
	}
	for i := range st.Keys {
		k := st.Keys[i]
		id, name, pub := k.ID, k.Name, k.PublicKey
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(k.Name, cols[0]),
			tdW(k.Algorithm, cols[1]),
			tdW(k.Fingerprint, cols[2]),
			tdW(k.Comment, cols[3]),
			tdW(k.CreatedAt, cols[4]),
			actionCellW(gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				Spacing: gui.SpacingTight,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "settings-key-copy-" + id,
						Variant: gui.ButtonGhost,
						Label:   i18n.T("settings.keys.copy_public"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctx.Window.SetClipboard(pub)
							toastOK(ctx.Window, i18n.T("settings.keys.public_key_copied"))
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "settings-key-edit-" + id,
						Variant: gui.ButtonGhost,
						Label:   i18n.T("common.edit"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							editKey(ctx.Window, d, k)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "settings-key-export-" + id,
						Variant: gui.ButtonGhost,
						Label:   i18n.T("settings.keys.export"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							exportKey(ctx.Window, d, k)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "settings-key-del-" + id,
						Variant: gui.ButtonGhost,
						Label:   i18n.T("common.delete"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							deleteKey(ctx.Window, d, id, name)
						},
					}),
				},
			})), cols[5]),
		}))
	}

	return panelColumn(i18n.T("settings.keys.title"), []gui.View{
		formRow("settings-key-name", i18n.T("settings.keys.name"), st.KeyGenName,
			i18n.T("settings.keys.name_placeholder"), false, func(v string) { st.KeyGenName = v }),
		gui.Select(gui.SelectCfg{
			ID:       "settings-key-algo",
			Label:    i18n.T("settings.keys.algorithm"),
			Selected: []string{st.KeyGenAlgo},
			Options:  algoOpts,
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).KeyGenAlgo = vals[0]
				}
			},
		}),
		formRow("settings-key-comment", i18n.T("settings.keys.comment"), st.KeyGenComment,
			i18n.T("settings.keys.comment_placeholder"), false, func(v string) { st.KeyGenComment = v }),
		gui.Button(gui.ButtonCfg{
			ID:       "settings-key-generate",
			Variant:  gui.ButtonPrimary,
			Label:    i18n.T("settings.keys.generate"),
			Disabled: st.Busy,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				generateKey(ctx.Window, d)
			},
		}),
		settingsTable("settings-keys-table", rows),
	})
}

func generateKey(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	name, algo, comment := st.KeyGenName, st.KeyGenAlgo, st.KeyGenComment
	withVault(w, d, i18n.T("password.encrypt_ssh_key"), func() {
		st.Busy = true
		go func() {
			_, err := d.Keys.GenerateAndSave(name, algo, comment)
			w.QueueCommand(func(win *gui.Window) {
				s := gui.State[State](win)
				s.Busy = false
				if err != nil {
					toastErr(win, err)
					return
				}
				s.KeyGenName = ""
				s.KeyGenComment = ""
				refreshLists(s, d)
				toastOK(win, i18n.T("settings.keys.saved_encrypted"))
			})
		}()
	})
}

// editKey shows a small dialog to rename a key and edit its comment.
func editKey(w *gui.Window, d *Deps, k sshkey.Info) {
	name, comment := k.Name, k.Comment
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("settings.keys.edit_title"),
		Width:      420,
		CustomView: func(*gui.Window) gui.View {
			return gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Content: []gui.View{
					gui.Input(gui.InputCfg{
						ID:          "settings-key-edit-name",
						Label:       i18n.T("settings.keys.name"),
						Text:        name,
						Placeholder: i18n.T("settings.keys.name_placeholder"),
						Sizing:      gui.FillFit,
						OnTextChanged: func(v string, ctx gui.EventCtx) {
							name = v
							ctx.Consume()
						},
					}),
					gui.Input(gui.InputCfg{
						ID:          "settings-key-edit-comment",
						Label:       i18n.T("settings.keys.comment"),
						Text:        comment,
						Placeholder: i18n.T("settings.keys.comment_placeholder"),
						Sizing:      gui.FillFit,
						OnTextChanged: func(v string, ctx gui.EventCtx) {
							comment = v
							ctx.Consume()
						},
					}),
					gui.Row(layout.Flush(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Spacing: gui.SpacingMedium,
						HAlign:  gui.HAlignEnd,
						Content: []gui.View{
							gui.Button(gui.ButtonCfg{
								ID:       "settings-key-edit-ok",
								Variant:  gui.ButtonPrimary,
								Label:    i18n.T("common.ok"),
								Disabled: strings.TrimSpace(name) == "",
								OnClick: func(ctx gui.EventCtx) {
									win := ctx.Window
									ctx.Consume()
									win.DialogDismiss()
									go func() {
										_, err := d.Keys.Update(k.ID, name, comment)
										win.QueueCommand(func(w2 *gui.Window) {
											if err != nil {
												toastErr(w2, err)
												return
											}
											refreshLists(gui.State[State](w2), d)
											toastOK(w2, i18n.T("settings.keys.updated"))
										})
									}()
								},
							}),
							gui.Button(gui.ButtonCfg{
								ID:      "settings-key-edit-cancel",
								Variant: gui.ButtonSecondary,
								Label:   i18n.T("common.cancel"),
								OnClick: func(ctx gui.EventCtx) {
									ctx.Consume()
									ctx.Window.DialogDismiss()
								},
							}),
						},
					})),
				},
			}))
		},
	})
}

func exportKey(w *gui.Window, d *Deps, k sshkey.Info) {
	withVault(w, d, i18n.T("password.decrypt_ssh_key"), func() {
		passphrase := ""
		w.Dialog(gui.DialogCfg{
			DialogType: gui.DialogCustom,
			Title:      i18n.T("settings.keys.export_private_title"),
			Width:      420,
			CustomView: func(*gui.Window) gui.View {
				th := gui.CurrentTheme()
				return gui.Column(layout.Flush(gui.ContainerCfg{
					Sizing:  gui.FillFit,
					Spacing: gui.SpacingMedium,
					Content: []gui.View{
						gui.Text(gui.TextCfg{
							Text:      i18n.T("settings.keys.export_passphrase_hint"),
							TextStyle: th.TextStyleSecondary,
							Mode:      gui.TextModeWrap,
							Sizing:    gui.FillFit,
						}),
						gui.Input(gui.InputCfg{
							ID:          "settings-key-export-passphrase",
							Placeholder: i18n.T("settings.keys.passphrase_placeholder"),
							IsPassword:  true,
							Sizing:      gui.FillFit,
							OnTextChanged: func(v string, ctx gui.EventCtx) {
								passphrase = v
								ctx.Consume()
							},
						}),
						gui.Row(layout.Flush(gui.ContainerCfg{
							Sizing:  gui.FillFit,
							Spacing: gui.SpacingMedium,
							HAlign:  gui.HAlignEnd,
							Content: []gui.View{
								gui.Button(gui.ButtonCfg{
									ID:      "settings-key-export-pass-ok",
									Variant: gui.ButtonPrimary,
									Label:   i18n.T("common.ok"),
									OnClick: func(ctx gui.EventCtx) {
										ctx.Consume()
										ctx.Window.DialogDismiss()
										pickExportPath(ctx.Window, d, k, passphrase)
									},
								}),
								gui.Button(gui.ButtonCfg{
									ID:      "settings-key-export-pass-cancel",
									Variant: gui.ButtonSecondary,
									Label:   i18n.T("common.cancel"),
									OnClick: func(ctx gui.EventCtx) {
										ctx.Consume()
										ctx.Window.DialogDismiss()
									},
								}),
							},
						})),
					},
				}))
			},
		})
	})
}

func pickExportPath(w *gui.Window, d *Deps, k sshkey.Info, passphrase string) {
	w.NativeSaveDialog(gui.NativeSaveDialogCfg{
		Title:            i18n.T("settings.keys.export_private_title"),
		DefaultName:      sshkey.DefaultFileName(k.Algorithm),
		ConfirmOverwrite: true,
		OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
			if res.Status != gui.DialogOK || len(res.Paths) == 0 {
				return
			}
			path := res.Paths[0].Path
			go func() {
				pem, err := d.Keys.PrivatePEM(k.ID)
				if err == nil {
					err = sshkey.WritePrivateKeyFile(path, pem, k.Comment, passphrase)
				}
				win.QueueCommand(func(w2 *gui.Window) {
					if err != nil {
						toastErr(w2, err)
						return
					}
					toastOK(w2, i18n.T("settings.keys.private_exported"))
				})
			}()
		},
	})
}

func deleteKey(w *gui.Window, d *Deps, id, name string) {
	dangerConfirmDialog(w,
		i18n.T("settings.keys.delete_title"),
		i18n.T("settings.keys.delete_body"),
		name,
		func(win *gui.Window) { confirmDeleteKey(win, d, id) })
}

func confirmDeleteKey(w *gui.Window, d *Deps, id string) {
	go func() {
		err := d.Keys.Delete(id)
		w.QueueCommand(func(w2 *gui.Window) {
			if err != nil {
				toastErr(w2, err)
				return
			}
			refreshLists(gui.State[State](w2), d)
			toastOK(w2, i18n.T("settings.keys.deleted"))
		})
	}()
}
