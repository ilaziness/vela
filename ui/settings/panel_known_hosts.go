package settings

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

// Minimum column widths; fingerprint (index 2) absorbs leftover width.
var knownHostsColMin = []float32{160, 100, 340, 72}

func panelKnownHosts(w *gui.Window, d *Deps, st *State) gui.View {
	cols := resolveColWidths(w, knownHostsColMin, 2)
	rows := []gui.TableRowCfg{
		gui.TR([]gui.TableCellCfg{
			thW(i18n.T("settings.known_hosts.col_host"), cols[0]),
			thW(i18n.T("settings.known_hosts.col_type"), cols[1]),
			thW(i18n.T("settings.known_hosts.col_fingerprint"), cols[2]),
			thW(i18n.T("settings.known_hosts.col_actions"), cols[3]),
		}),
	}
	if len(st.KnownHosts) == 0 {
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(i18n.T("settings.known_hosts.empty"), cols[0]),
			tdW("", cols[1]),
			tdW("", cols[2]),
			tdW("", cols[3]),
		}))
	}
	for i := range st.KnownHosts {
		e := st.KnownHosts[i]
		host, keyType := e.Host, e.KeyType
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(e.Host, cols[0]),
			tdW(e.KeyType, cols[1]),
			tdW(e.Fingerprint, cols[2]),
			actionCellW(gui.Button(gui.ButtonCfg{
				ID:      "settings-kh-del-" + host + "-" + keyType,
				Variant: gui.ButtonGhost,
				Label:   i18n.T("common.delete"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					deleteKnownHost(ctx.Window, d, host, keyType)
				},
			}), cols[3]),
		}))
	}

	return panelColumn(i18n.T("settings.known_hosts.title"), []gui.View{
		gui.Label(i18n.T("settings.known_hosts.hint"), gui.CurrentTheme().TextStyleSecondary),
		settingsTable("settings-known-hosts-table", rows),
	})
}

func deleteKnownHost(w *gui.Window, d *Deps, host, keyType string) {
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogConfirm,
		Title:      i18n.T("settings.known_hosts.delete_title"),
		Body:       i18n.T("settings.known_hosts.delete_body"),
		OnOkYes: func(win *gui.Window) {
			go func() {
				err := d.SSH.DeleteKnownHost(host, keyType)
				win.QueueCommand(func(w2 *gui.Window) {
					if err != nil {
						toastErr(w2, err)
						return
					}
					refreshLists(gui.State[State](w2), d)
					toastOK(w2, i18n.T("settings.known_hosts.deleted"))
				})
			}()
		},
	})
}
