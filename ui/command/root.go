package commandui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/chrome"
	"github.com/ilaziness/vela/ui/layout"
)

// Root is the command palette shell: command library | sessions + input.
func Root(w *gui.Window, d *Deps) gui.View {
	st := gui.State[State](w)
	st.ensureLoaded()
	th := gui.CurrentTheme()

	lang := i18n.Language()
	if lang != st.Lang || !st.dataLoaded {
		st.Lang = lang
		w.SetTitle(i18n.T("window.command"))
		st.reload(d)
	}

	body := gui.Row(layout.Flush(gui.ContainerCfg{
		ID:     "cmd-root",
		Sizing: gui.FillFill,
		Color:  th.ColorBackground,
		Content: []gui.View{
			libraryPanel(d, st),
			layout.VRule(),
			sendPanel(w, d, st),
		},
	}))
	if st.Frameless {
		return chrome.Frame(w, i18n.T("window.command"), &st.BarClicks, body)
	}
	return body
}
