package main

import (
	_ "embed"
	"log"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"

	"github.com/ilaziness/vela/internal/tools"
	"github.com/ilaziness/vela/internal/vaultcheck"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/backup"
	"github.com/ilaziness/vela/ui/bookmarks"
	"github.com/ilaziness/vela/ui/fonts"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/settings"
	"github.com/ilaziness/vela/ui/shell"
	toolsui "github.com/ilaziness/vela/ui/tools"
	"github.com/ilaziness/vela/ui/vaultprompt"
)

//go:embed assets/appicon.png
var appIconPNG []byte

func main() {
	rt, err := app.Bootstrap()
	if err != nil {
		log.Fatal(err)
	}

	guiApp := gui.NewApp()
	rt.App = guiApp

	// UI: system sans; terminal/mono: system monospace; missing glyphs → system CJK.
	fonts.Apply(rt.Store.Config)

	settingsDeps := &settings.Deps{
		App:     guiApp,
		Store:   rt.Store,
		Ctrl:    rt.Ctrl,
		SSH:     rt.SSH,
		Vault:   rt.Vault,
		Keys:    rt.Keys,
		Keybind: rt.Keybind,
		DB:      rt.DB,
		IconPNG: appIconPNG,
	}
	bookmarksDeps := &bookmarks.Deps{
		App:       guiApp,
		Bookmarks: rt.Bookmarks,
		Keys:      rt.Keys,
		Ctrl:      rt.Ctrl,
		Store:     rt.Store,
		Vault:     rt.Vault,
		DB:        rt.DB,
		IconPNG:   appIconPNG,
	}
	toolsDeps := &toolsui.Deps{
		App:     guiApp,
		Core:    tools.New(),
		IconPNG: appIconPNG,
	}
	backupDeps := &backup.Deps{
		Store:  rt.Store,
		Vault:  rt.Vault,
		DB:     rt.DB,
		Logger: rt.Logger,
	}

	mainWin := gui.NewWindow(gui.WindowCfg{
		Title:   "Vela",
		Width:   1280,
		Height:  800,
		IconPNG: appIconPNG,
		State:   &app.State{},
		OnInit: func(w *gui.Window) {
			rt.BindWindow(w)
			w.OnEvent = func(e *gui.Event, w *gui.Window) {
				st := gui.State[app.State](w)
				rt.Ctrl.HandleWindowEvent(e, st)
			}

			mountShell := func() {
				w.SetView(func(w *gui.Window) gui.View {
					return shell.Root(w, rt.Ctrl, shell.Chrome{
						OnBookmarks: func() { bookmarks.Open(bookmarksDeps) },
						OnSettings:  func() { settings.Open(settingsDeps) },
						OnTools:     func() { toolsui.Open(toolsDeps) },
						OnBackup:    func() { backup.Start(w, backupDeps) },
					})
				})
			}

			if !vaultcheck.NeedsUnlock(rt.Vault, rt.Store.UserDataDir(), rt.Store, rt.DB) {
				mountShell()
				return
			}

			w.SetView(func(w *gui.Window) gui.View {
				th := gui.CurrentTheme()
				return gui.Column(layout.Flush(gui.ContainerCfg{
					Sizing: gui.FillFill,
					Color:  th.ColorBackground,
				}))
			})

			var probe func(string) bool
			if vaultcheck.HasEncryptedSecrets(rt.Store, rt.DB) {
				probe = func(pw string) bool {
					return vaultcheck.ProbePassword(rt.Store, rt.DB, pw)
				}
			}
			vaultprompt.StartupGate(w, vaultprompt.StartupGateCfg{
				Vault:       rt.Vault,
				UserDataDir: rt.Store.UserDataDir(),
				Probe:       probe,
				OnUnlocked:  func(*gui.Window) { mountShell() },
				OnQuit: func(win *gui.Window) {
					rt.Ctrl.Shutdown()
					win.Close()
				},
			})
		},
		OnCloseRequest: func(w *gui.Window) {
			// OnCloseRequest vetoes the OS close unless Close() is called.
			rt.Ctrl.Shutdown()
			w.Close()
		},
	})
	backend.RunApp(guiApp, mainWin)
}
