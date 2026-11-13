package bookmarks

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

// Root is the bookmark manager shell: left sidebar + right editor.
func Root(w *gui.Window, d *Deps) gui.View {
	st := gui.State[State](w)
	st.ensureLoaded()
	if lang := i18n.Language(); lang != st.Lang {
		st.Lang = lang
		w.SetTitle(i18n.T("header.bookmarks"))
	}
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:     "bm-root",
		Sizing: gui.FillFill,
		Color:  gui.CurrentTheme().ColorBackground,
		Content: []gui.View{
			sidebar(w, d),
			layout.VRule(),
			editor(w, d),
		},
	}))
}
