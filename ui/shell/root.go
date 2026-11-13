package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/session"
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

	// [terminal] [perf panel?] | [tool rail?] — the rail belongs to a
	// live session tab and appears only once it has connected.
	if tab != nil && tab.Kind == app.TabSession && tab.SessID != "" {
		if tab.PerfOpen {
			body = gui.Row(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Content: []gui.View{body, session.PerfPanel(ctrl, tab)},
			}))
		}
		body = gui.Row(flush(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Content: []gui.View{body, vRule(), session.TabRail(ctrl, tab)},
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
