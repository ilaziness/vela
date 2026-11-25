package main

import (
	"embed"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/buildinfo"
	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/internal/updater"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/fonts"
)

//go:embed assets/appicon.png
var appIconPNG []byte

//go:embed data/commands/*.json
var commandsFS embed.FS

func main() {
	if handled, err := updater.RunApplyMode(os.Args[1:]); handled {
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	rt, err := app.Bootstrap(command.LoaderFromFS(commandsFS, "data/commands"))
	if err != nil {
		log.Fatal(err)
	}
	updater.CleanupLeftovers(rt.Logger)

	// Non-release builds run go-gui's dev-mode frame audit: duplicate or
	// unscoped IDs, unknown focus IDs (DebugUnknownFocus), stale stamps.
	// Findings go to stderr once per finding per window.
	if !buildinfo.IsRelease() {
		gui.Debug(true)
		rt.Logger.Debug("go-gui debug audit enabled",
			zap.Bool("enabled", gui.DebugEnabled()))
	}

	guiApp := gui.NewApp()
	rt.App = guiApp
	fonts.Apply(rt.Store.Config)

	mainWin := newMainWindow(rt, newWindowDeps(rt, guiApp))
	backend.RunApp(guiApp, mainWin)
}
