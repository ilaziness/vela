package commandui

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/chrome"
)

var (
	mu      sync.Mutex
	cmdWin  *gui.Window
	opening bool
)

// Open shows the command palette window; creates one if needed (singleton).
func Open(d *Deps) {
	mu.Lock()
	if cmdWin != nil {
		w := cmdWin
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

	frameless := d.Store.Config.General.Frameless
	d.App.OpenWindow(gui.WindowCfg{
		Title:       i18n.T("window.command"),
		Width:       980,
		Height:      640,
		MinWidth:    720,
		MinHeight:   480,
		IconPNG:     d.IconPNG,
		State:       &State{Frameless: frameless},
		Decorations: chrome.Decoration(frameless),
		OnInit: func(w *gui.Window) {
			mu.Lock()
			cmdWin = w
			opening = false
			mu.Unlock()
			st := gui.State[State](w)
			st.ensureLoaded()
			st.Lang = i18n.Language()
			w.SetView(func(win *gui.Window) gui.View {
				return Root(win, d)
			})
		},
		OnCloseRequest: func(w *gui.Window) {
			mu.Lock()
			if cmdWin == w {
				cmdWin = nil
			}
			opening = false
			mu.Unlock()
			w.Close()
		},
	})
}

// Toggle shows the command palette, or hides it when already visible.
// Hiding (not closing) keeps drafts and selections for the next open.
func Toggle(d *Deps) {
	mu.Lock()
	w := cmdWin
	mu.Unlock()
	if w != nil {
		if w.IsVisible() {
			w.Hide()
			return
		}
		w.Show()
		return
	}
	Open(d)
}
