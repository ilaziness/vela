package settings

import (
	"os"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

// Minimum column widths; fingerprint (index 2) absorbs leftover width.
var keysColMin = []float32{100, 88, 320, 72, 128, 210}

func panelKeys(w *gui.Window, d *Deps, st *State) gui.View {
	cols := resolveColWidths(w, keysColMin, 2)
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
						ID:      "settings-key-export-" + id,
						Variant: gui.ButtonGhost,
						Label:   i18n.T("settings.keys.export"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							exportKey(ctx.Window, d, id, name)
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

func exportKey(w *gui.Window, d *Deps, id, name string) {
	withVault(w, d, i18n.T("password.decrypt_ssh_key"), func() {
		w.NativeSaveDialog(gui.NativeSaveDialogCfg{
			Title:            i18n.T("settings.keys.export_private_title"),
			DefaultName:      sanitizeFileName(name) + ".pem",
			ConfirmOverwrite: true,
			OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
				if res.Status != gui.DialogOK || len(res.Paths) == 0 {
					return
				}
				path := res.Paths[0].Path
				go func() {
					pem, err := d.Keys.PrivatePEM(id)
					if err == nil {
						err = os.WriteFile(path, []byte(pem), 0o600)
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
	})
}

func deleteKey(w *gui.Window, d *Deps, id, name string) {
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogConfirm,
		Title:      i18n.T("settings.keys.delete_title"),
		Body:       i18n.Tf("settings.keys.delete_body", name),
		OnOkYes: func(win *gui.Window) {
			go func() {
				err := d.Keys.Delete(id)
				win.QueueCommand(func(w2 *gui.Window) {
					if err != nil {
						toastErr(w2, err)
						return
					}
					refreshLists(gui.State[State](w2), d)
					toastOK(w2, i18n.T("settings.keys.deleted"))
				})
			}()
		},
	})
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "id_ed25519"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "key"
	}
	return b.String()
}
