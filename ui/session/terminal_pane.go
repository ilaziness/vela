package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// TerminalPane embeds the go-term view flush in the main pane.
func TerminalPane(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	if tab == nil || tab.SessID == "" {
		return gui.Label("无会话", gui.CurrentTheme().TextStyleBody)
	}
	tm := ctrl.TermFor(tab.SessID)
	if tm == nil {
		return gui.Label("终端未就绪", gui.CurrentTheme().TextStyleBody)
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Content: []gui.View{tm.View(w)},
	}))
}
