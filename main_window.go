package main

import (
	"context"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/tools"
	"github.com/ilaziness/vela/internal/vaultcheck"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/backup"
	"github.com/ilaziness/vela/ui/bookmarks"
	"github.com/ilaziness/vela/ui/chrome"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/settings"
	"github.com/ilaziness/vela/ui/shell"
	toolsui "github.com/ilaziness/vela/ui/tools"
	"github.com/ilaziness/vela/ui/update"
	"github.com/ilaziness/vela/ui/vaultprompt"
)

type windowDeps struct {
	Update    *update.Deps
	Settings  *settings.Deps
	Bookmarks *bookmarks.Deps
	Tools     *toolsui.Deps
	Backup    *backup.Deps
}

func newWindowDeps(rt *app.Runtime, guiApp *gui.App) *windowDeps {
	updateDeps := &update.Deps{
		Manager: rt.Updater,
		OnInstalled: func() {
			rt.Ctrl.Shutdown()
			_ = rt.Logger.Sync()
			os.Exit(0)
		},
	}
	return &windowDeps{
		Update: updateDeps,
		Settings: &settings.Deps{
			App:     guiApp,
			Store:   rt.Store,
			Ctrl:    rt.Ctrl,
			SSH:     rt.SSH,
			Vault:   rt.Vault,
			Keys:    rt.Keys,
			Keybind: rt.Keybind,
			DB:      rt.DB,
			IconPNG: appIconPNG,
			Update:  updateDeps,
		},
		Bookmarks: &bookmarks.Deps{
			App:       guiApp,
			Bookmarks: rt.Bookmarks,
			Keys:      rt.Keys,
			Ctrl:      rt.Ctrl,
			Store:     rt.Store,
			Vault:     rt.Vault,
			DB:        rt.DB,
			IconPNG:   appIconPNG,
		},
		Tools: &toolsui.Deps{
			App:     guiApp,
			Core:    tools.New(),
			Store:   rt.Store,
			IconPNG: appIconPNG,
		},
		Backup: &backup.Deps{
			Store:  rt.Store,
			Vault:  rt.Vault,
			DB:     rt.DB,
			Logger: rt.Logger,
		},
	}
}

func newMainWindow(rt *app.Runtime, d *windowDeps) *gui.Window {
	return gui.NewWindow(gui.WindowCfg{
		Title:       "Vela",
		Width:       1280,
		Height:      800,
		IconPNG:     appIconPNG,
		State:       &app.State{},
		Decorations: chrome.Decoration(rt.Store.Config.General.Frameless),
		OnInit: func(w *gui.Window) {
			rt.BindWindow(w)
			frameless := rt.Store.Config.General.Frameless
			gui.State[app.State](w).Frameless = frameless
			if frameless {
				chrome.CenterOnScreen(w)
			}
			w.OnEvent = func(e *gui.Event, w *gui.Window) {
				st := gui.State[app.State](w)
				rt.Ctrl.HandleWindowEvent(e, st)
			}

			mountShell := func() {
				w.SetView(func(w *gui.Window) gui.View {
					return shell.Root(w, rt.Ctrl, shell.Chrome{
						OnBookmarks: func() { bookmarks.Open(d.Bookmarks) },
						OnSettings:  func() { settings.Open(d.Settings) },
						OnTools:     func() { toolsui.Open(d.Tools) },
						OnBackup:    func() { backup.Start(w, d.Backup) },
					})
				})
				startUpdateCheck(w, rt, d.Update)
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
			rt.Ctrl.Shutdown()
			w.Close()
		},
	})
}

func startUpdateCheck(w *gui.Window, rt *app.Runtime, d *update.Deps) {
	if d == nil || d.Manager == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				rt.Logger.Error("background update check panicked", zap.Any("panic", r))
			}
		}()
		time.Sleep(10 * time.Second)
		rel, err := d.Manager.Check(context.Background())
		if err != nil {
			rt.Logger.Debug("background update check failed", zap.Error(err))
			return
		}
		if rel == nil {
			return
		}
		w.QueueCommand(func(win *gui.Window) {
			st := gui.State[app.State](win)
			if d.Prompted.Load() || st.UpdatePrompted {
				return
			}
			st.UpdatePrompted = true
			update.NotifyAvailable(win, d)
		})
	}()
}
