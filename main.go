package main

import (
	_ "embed"
	"log"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"

	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/fonts"
	"github.com/ilaziness/vela/ui/shell"
)

//go:embed assets/appicon.png
var appIconPNG []byte

func main() {
	rt, err := app.Bootstrap()
	if err != nil {
		log.Fatal(err)
	}

	// UI: system sans; terminal/mono: system monospace; missing glyphs → system CJK.
	fonts.Apply(rt.Store.Config)

	win := gui.NewWindow(gui.WindowCfg{
		Title:   "Vela",
		Width:   1280,
		Height:  800,
		IconPNG: appIconPNG,
		State:   &app.State{},
		OnInit: func(w *gui.Window) {
			rt.BindWindow(w)
			w.SetView(func(w *gui.Window) gui.View {
				return shell.Root(w, rt.Ctrl)
			})
			w.OnEvent = func(e *gui.Event, w *gui.Window) {
				st := gui.State[app.State](w)
				rt.Ctrl.HandleWindowEvent(e, st)
			}
		},
		OnCloseRequest: func(w *gui.Window) {
			// OnCloseRequest vetoes the OS close unless Close() is called.
			rt.Ctrl.Shutdown()
			w.Close()
		},
	})
	backend.Run(win)
}
