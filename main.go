package main

import (
	"embed"
	_ "embed"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/services"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed data/commands.json
var commandsFile embed.FS

func main() {
	var commandsJSON []byte
	data, err := commandsFile.ReadFile("data/commands.json")
	if err != nil {
		fmt.Printf("读取 commands.json 失败：%v\n", err)
	} else {
		commandsJSON = data
	}

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

	if err := services.RegisterServices(app, mainWindow, cfgStore, logService.Logger(), commandsJSON); err != nil {
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
