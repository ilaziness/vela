package bookmarks

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

var (
	mu      sync.Mutex
	bmWin   *gui.Window
	opening bool
)

// Open shows the bookmark manager; creates one if needed (singleton).
func Open(d *Deps) {
	mu.Lock()
	if bmWin != nil {
		w := bmWin
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
		Title:     i18n.T("header.bookmarks"),
		Width:     1100,
		Height:    720,
		MinWidth:  800,
		MinHeight: 560,
		IconPNG:   d.IconPNG,
		State:     &State{},
		OnInit: func(w *gui.Window) {
			mu.Lock()
			bmWin = w
			opening = false
			mu.Unlock()
			st := gui.State[State](w)
			st.ensureLoaded()
			st.Lang = i18n.Language()
			reload(w, deps)
			w.SetView(func(win *gui.Window) gui.View {
				return Root(win, deps)
			})
		},
		OnCloseRequest: func(w *gui.Window) {
			st := gui.State[State](w)
			if st.Dirty {
				w.Dialog(gui.DialogCfg{
					DialogType: gui.DialogConfirm,
					Title:      i18n.T("bookmark.unsaved_title"),
					Body:       i18n.T("bookmark.unsaved_body"),
					OnOkYes: func(win *gui.Window) {
						closeWindow(win)
					},
				})
				return
			}
			closeWindow(w)
		},
	})
}

func closeWindow(w *gui.Window) {
	mu.Lock()
	if bmWin == w {
		bmWin = nil
	}
	opening = false
	mu.Unlock()
	w.Close()
}
