package bookmarks

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/chrome"
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
	frameless := deps.Store.Config.General.Frameless
	d.App.OpenWindow(gui.WindowCfg{
		Title:       i18n.T("header.bookmarks"),
		Width:       1100,
		Height:      720,
		MinWidth:    800,
		MinHeight:   560,
		IconPNG:     d.IconPNG,
		State:       &State{Frameless: frameless},
		Decorations: chrome.Decoration(frameless),
		OnInit: func(w *gui.Window) {
			mu.Lock()
			bmWin = w
			opening = false
			mu.Unlock()
			st := gui.State[State](w)
			st.ensureLoaded()
			st.Lang = i18n.Language()
			reload(w, deps)

			// Route SSH trust / keyboard dialogs to this window.
			st.ContextKey = deps.Ctrl.RegisterWindowPrompters(w)

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
						closeWindow(win, deps)
					},
				})
				return
			}
			closeWindow(w, deps)
		},
	})
}

func closeWindow(w *gui.Window, d *Deps) {
	mu.Lock()
	if bmWin == w {
		bmWin = nil
	}
	opening = false
	mu.Unlock()

	// Unregister prompters
	if st := gui.State[State](w); st.ContextKey != "" {
		d.Ctrl.UnregisterWindowPrompters(st.ContextKey)
	}

	w.Close()
}
