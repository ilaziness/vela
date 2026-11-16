package main

import (
	_ "embed"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"

	"github.com/ilaziness/vela/internal/updater"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/fonts"
)

//go:embed assets/appicon.png
var appIconPNG []byte

func main() {
	if handled, err := updater.RunApplyMode(os.Args[1:]); handled {
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	rt, err := app.Bootstrap()
	if err != nil {
		log.Fatal(err)
	}
	updater.CleanupLeftovers(rt.Logger)

	guiApp := gui.NewApp()
	rt.App = guiApp
	fonts.Apply(rt.Store.Config)

	mainWin := newMainWindow(rt, newWindowDeps(rt, guiApp))
	backend.RunApp(guiApp, mainWin)
}
