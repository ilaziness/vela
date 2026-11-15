package settings

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/chrome"
)

var (
	mu          sync.Mutex
	settingsWin *gui.Window
	opening     bool
)

// Open shows the settings window; creates one if needed (singleton).
func Open(d *Deps) {
	mu.Lock()
	if settingsWin != nil {
		w := settingsWin
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
	frameless := deps.Store.Config.General.Frameless
	d.App.OpenWindow(gui.WindowCfg{
		Title:       i18n.T("header.settings"),
		Width:       960,
		Height:      720,
		IconPNG:     d.IconPNG,
		State:       &State{Frameless: frameless},
		Decorations: chrome.Decoration(frameless),
		OnInit: func(w *gui.Window) {
			mu.Lock()
			settingsWin = w
			opening = false
			mu.Unlock()
			st := gui.State[State](w)
			st.ensureLoaded(deps)
			if frameless {
				chrome.CenterOnScreen(w)
			}
			w.SetView(func(win *gui.Window) gui.View {
				return Root(win, deps)
			})
			w.OnEvent = func(e *gui.Event, win *gui.Window) {
				handleKeybindCapture(e, win, deps)
			}
		},
		OnCloseRequest: func(w *gui.Window) {
			mu.Lock()
			if settingsWin == w {
				settingsWin = nil
			}
			opening = false
			mu.Unlock()
			w.Close()
		},
	})
}

func refreshTitle(w *gui.Window) {
	w.SetTitle(i18n.T("header.settings"))
}
