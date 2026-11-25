package settings

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

// Minimum column widths; all data columns share leftover width
// proportionally (key data has the largest minimum).
var knownHostsColMin = []float32{150, 90, 210, 300, 72}

func panelKnownHosts(w *gui.Window, d *Deps, st *State) gui.View {
	cols := resolveColWidths(w, knownHostsColMin, 0, 1, 2, 3)
	th := gui.CurrentTheme()
	rows := []gui.TableRowCfg{
		gui.TR([]gui.TableCellCfg{
			thW(i18n.T("settings.known_hosts.col_host"), cols[0]),
			thW(i18n.T("settings.known_hosts.col_type"), cols[1]),
			thW(i18n.T("settings.known_hosts.col_fingerprint"), cols[2]),
			thW(i18n.T("settings.known_hosts.col_key"), cols[3]),
			thW(i18n.T("settings.known_hosts.col_actions"), cols[4]),
		}),
	}
	if len(st.KnownHosts) == 0 {
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(i18n.T("settings.known_hosts.empty"), cols[0]),
			tdW("", cols[1]),
			tdW("", cols[2]),
			tdW("", cols[3]),
			tdW("", cols[4]),
		}))
	}
	for i := range st.KnownHosts {
		e := st.KnownHosts[i]
		host, keyType := e.Host, e.KeyType
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(e.Host, cols[0]),
			tdW(e.KeyType, cols[1]),
			tdW(e.Fingerprint, cols[2]),
			tdW(e.KeyType+" "+e.KeyData, cols[3]),
			actionCellW(gui.Button(gui.ButtonCfg{
				ID:      "settings-kh-del-" + host + "-" + keyType,
				Variant: gui.ButtonGhost,
				Label:   i18n.T("common.delete"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					deleteKnownHost(ctx.Window, d, host, keyType)
				},
			}), cols[4]),
		}))
	}

	hintRow := gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFit,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      i18n.T("settings.known_hosts.hint"),
				TextStyle: th.TextStyleSecondary,
				Sizing:    gui.FillFit,
			}),
			gui.Button(gui.ButtonCfg{
				ID:       "settings-kh-clear-all",
				Variant:  gui.ButtonGhost,
				Label:    i18n.T("settings.known_hosts.clear_all"),
				Disabled: len(st.KnownHosts) == 0,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					clearKnownHosts(ctx.Window, d)
				},
			}),
		},
	}))

	return panelColumn(i18n.T("settings.known_hosts.title"), []gui.View{
		hintRow,
		settingsTable("settings-known-hosts-table", rows),
	})
}

func clearKnownHosts(w *gui.Window, d *Deps) {
	dangerConfirmDialog(w,
		i18n.T("settings.known_hosts.clear_all_title"),
		i18n.T("settings.known_hosts.clear_all_body"),
		"",
		func(win *gui.Window) { confirmClearKnownHosts(win, d) })
}

func confirmClearKnownHosts(w *gui.Window, d *Deps) {
	go func() {
		err := d.SSH.ClearKnownHosts()
		w.QueueCommand(func(w2 *gui.Window) {
			if err != nil {
				toastErr(w2, err)
				return
			}
			refreshLists(gui.State[State](w2), d)
			toastOK(w2, i18n.T("settings.known_hosts.cleared"))
		})
	}()
}

func deleteKnownHost(w *gui.Window, d *Deps, host, keyType string) {
	dangerConfirmDialog(w,
		i18n.T("settings.known_hosts.delete_title"),
		i18n.T("settings.known_hosts.delete_body"),
		host+" ("+keyType+")",
		func(win *gui.Window) { confirmDeleteKnownHost(win, d, host, keyType) })
}

func confirmDeleteKnownHost(w *gui.Window, d *Deps, host, keyType string) {
	go func() {
		err := d.SSH.DeleteKnownHost(host, keyType)
		w.QueueCommand(func(w2 *gui.Window) {
			if err != nil {
				toastErr(w2, err)
				return
			}
			refreshLists(gui.State[State](w2), d)
			toastOK(w2, i18n.T("settings.known_hosts.deleted"))
		})
	}()
}
