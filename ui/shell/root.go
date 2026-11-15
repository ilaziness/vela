package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/app"
	chromeui "github.com/ilaziness/vela/ui/chrome"
	"github.com/ilaziness/vela/ui/session"
	sftpui "github.com/ilaziness/vela/ui/sftp"
	transferui "github.com/ilaziness/vela/ui/transfer"
	tunnelui "github.com/ilaziness/vela/ui/tunnel"
)

// Root builds the edge-to-edge shell: icon rail | [bookmarks] | main.
func Root(w *gui.Window, ctrl *app.Controller, chrome Chrome) gui.View {
	st := gui.State[app.State](w)
	st.EnsureDefaults(ctrl.NewSSHDraft)
	th := gui.CurrentTheme()

	cols := []gui.View{IconRail(w, ctrl, chrome), vRule()}
	if st.SideVisible && st.ActiveNav == navQuickConnect {
		cols = append(cols, SidePanel(w, ctrl), vRule())
	}
	cols = append(cols, mainPane(w, ctrl, st))
	if st.Frameless {
		// Bottom-right corner resize for the borderless frame; the
		// float child overlays the layout, so it needs no column.
		cols = append(cols, chromeui.ResizeGrip())
	}

	return gui.Row(flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   th.ColorBackground,
		Content: cols,
	}))
}

func mainPane(w *gui.Window, ctrl *app.Controller, st *app.State) gui.View {
	th := gui.CurrentTheme()
	tab := st.Active()
	var body gui.View
	switch {
	case tab == nil:
		body = gui.Label(i18n.T("shell.tab.open_hint"), th.TextStyleSecondary)
	case tab.Kind == app.TabSession:
		body = session.TerminalPane(w, ctrl, tab)
	default:
		body = session.FormHost(w, ctrl, tab)
	}

	// [terminal] [perf panel?] [transfer panel?] | [tool rail?] — the rail belongs to a
	// live SSH session tab and appears only once it has connected;
	// every tool it offers (sftp, perf, tunnel, transfers) is
	// SSH-only, so serial sessions get no rail. The SFTP panel stacks
	// over the whole stage; the rail and tab bar stay visible and the
	// terminal keeps running underneath.
	if tab != nil && tab.Kind == app.TabSession && tab.SessID != "" && tab.Protocol == termio.ProtocolSSH {
		stage := body
		if tab.PerfOpen {
			stage = gui.Row(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Content: []gui.View{stage, session.PerfPanel(ctrl, tab)},
			}))
		}
		if tab.TunnelOpen {
			stage = gui.Row(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Content: []gui.View{stage, tunnelui.TunnelPanel(ctrl, tab)},
			}))
		}
		if tab.TransferOpen {
			stage = gui.Row(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Content: []gui.View{stage, transferui.TransferPanel(ctrl, tab)},
			}))
		}
		if tab.SFTPOpen {
			stage = gui.Stack(flush(gui.ContainerCfg{
				Sizing: gui.FillFill,
				Content: []gui.View{
					stage,
					sftpui.View(w, ctrl, tab),
				},
			}))
		}
		body = gui.Row(flush(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Content: []gui.View{stage, vRule(), session.TabRail(ctrl, tab)},
		}))
	}

	return gui.Column(flush(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Color:  th.ColorBackground,
		Content: []gui.View{
			session.TabBar(w, ctrl),
			body,
		},
	}))
}
