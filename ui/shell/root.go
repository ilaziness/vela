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

	cols := []gui.View{IconRail(w, chrome), vRule()}
	if st.SideVisible && st.ActiveNav == navQuickConnect {
		cols = append(cols, SidePanel(ctrl), vRule())
	}
	cols = append(cols, mainPane(w, ctrl, st))

	return gui.Row(flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   th.ColorBackground,
		Content: cols,
	}))
}

func mainPane(w *gui.Window, ctrl *app.Controller, st *app.State) gui.View {
	var body gui.View
	if tab := st.Active(); tab != nil {
		switch tab.Kind {
		case app.TabSession:
			body = session.TerminalPane(w, ctrl, tab)
		default:
			body = session.FormHost(w, ctrl, tab)
		}
	} else {
		body = gui.Label(i18n.T("shell.tab.open_hint"), gui.CurrentTheme().TextStyleSecondary)
	}
	th := gui.CurrentTheme()
	return gui.Column(flush(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Color:  th.ColorBackground,
		Content: []gui.View{
			session.TabBar(w, ctrl),
			body,
		},
	}))
}
