package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// TerminalPane embeds the go-term view flush in the main pane, or the
// reconnect pane when the session behind the tab has dropped.
func TerminalPane(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	if tab == nil || tab.Kind != app.TabSession {
		return gui.Label(i18n.T("shell.session.none"), gui.CurrentTheme().TextStyleBody)
	}
	if tab.SessID == "" {
		// The session ended but the tab was kept: offer a reconnect.
		return ReconnectPane(w, ctrl, tab)
	}
	tm := ctrl.TermFor(tab.SessID)
	if tm == nil {
		// SessID without a term is a broken invariant, not a disconnect.
		return gui.Label(i18n.T("shell.session.term_not_ready"), gui.CurrentTheme().TextStyleBody)
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Content: []gui.View{tm.View(w)},
	}))
}
