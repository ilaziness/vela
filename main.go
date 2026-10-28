package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/services"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed data/commands/*.json
var commandsFS embed.FS

func main() {
	logService := services.NewLogService()
	cfgStore, err := config.Load(logService.Logger())
	if err != nil {
		log.Fatal(err)
	}
	app := application.New(application.Options{
		Name: "Vela",
		Services: []application.Service{
			application.NewService(logService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			WebviewUserDataPath: system.GetExecutableDir(),
		},
	})

	mainWindow := app.Window.NewWithOptions(mainWindowOptions(cfgStore.Config.General.Frameless))

	commandsLoader := command.LoaderFromFS(commandsFS, "data/commands")
	if err := services.RegisterServices(app, mainWindow, cfgStore, logService.Logger(), commandsLoader); err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func mainWindowOptions(frameless bool) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Title:                      "Vela",
		URL:                        "/",
		Width:                      1600,
		Height:                     900,
		Frameless:                  frameless,
		DefaultContextMenuDisabled: true,
		EnableFileDrop:             true,
	}
	if frameless {
		opts.Mac = application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		}
	}
	return opts
}
