package toolsui

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

var (
	mu       sync.Mutex
	toolsWin *gui.Window
	opening  bool
)

// Open shows the toolbox window; creates one if needed (singleton).
func Open(d *Deps) {
	mu.Lock()
	if toolsWin != nil {
		w := toolsWin
		mu.Unlock()
		w.Show()
		return
	}
	if opening {
		mu.Unlock()
		return
	}
	opening = true
	mu.Unlock()

	deps := d
	d.App.OpenWindow(gui.WindowCfg{
		Title:     i18n.T("window.tools"),
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		IconPNG:   d.IconPNG,
		State:     &State{},
		OnInit: func(w *gui.Window) {
			mu.Lock()
			toolsWin = w
			opening = false
			mu.Unlock()
			st := gui.State[State](w)
			st.ensureLoaded()
			st.Lang = i18n.Language()
			w.SetView(func(win *gui.Window) gui.View {
				return Root(win, deps)
			})
		},
		OnCloseRequest: func(w *gui.Window) {
			mu.Lock()
			if toolsWin == w {
				toolsWin = nil
			}
			opening = false
			mu.Unlock()
			w.Close()
		},
	})
}
