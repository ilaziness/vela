package main

import (
	"context"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/buildinfo"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/tools"
	"github.com/ilaziness/vela/internal/vaultcheck"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/backup"
	"github.com/ilaziness/vela/ui/bookmarks"
	"github.com/ilaziness/vela/ui/chrome"
	commandui "github.com/ilaziness/vela/ui/command"
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
	Command   *commandui.Deps
	Backup    *backup.Deps
}

// windowStateSaver persists the main window's normal size and maximised
// flag. Saves are debounced so a resize drag writes once at the end.
type windowStateSaver struct {
	store  *config.Store
	logger *zap.Logger
	timer  *time.Timer
}

const windowStateDebounce = 400 * time.Millisecond

// schedule saves after the resize burst ends. Both callers run on the
// UI thread, so the timer field needs no lock.
func (s *windowStateSaver) schedule(w *gui.Window) {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(windowStateDebounce, func() {
		// WindowSize/Maximize getters belong to the UI thread.
		w.QueueCommand(func(win *gui.Window) { s.save(win) })
	})
}

// flush stops a pending save and writes the current geometry now.
func (s *windowStateSaver) flush(w *gui.Window) {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.save(w)
}

func (s *windowStateSaver) save(w *gui.Window) {
	st := config.WindowGeometry{Maximised: w.IsMaximized() || w.IsFullscreen()}
	if !st.Maximised {
		st.Width, st.Height = w.WindowSize()
	}
	if err := s.store.SaveWindow(st); err != nil {
		s.logger.Debug("save window state failed", zap.Error(err))
	}
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
			Agent:   rt.Agent,
			SSH:     rt.SSH,
			Vault:   rt.Vault,
			Keys:    rt.Keys,
			Keybind: rt.Keybind,
			DB:      rt.DB,
			IconPNG: appIconPNG,
			Update:  updateDeps,
			OnLanguageChanged: func(lang string) {
				if err := rt.Commands.ReloadBuiltin(lang); err != nil {
					rt.Logger.Warn("reload builtin commands failed",
						zap.String("language", lang), zap.Error(err))
				}
			},
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
		Command: &commandui.Deps{
			App:      guiApp,
			Commands: rt.Commands,
			Hub:      rt.Hub,
			Store:    rt.Store,
			IconPNG:  appIconPNG,
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
	saved := rt.Store.Window()
	width, height := saved.EffectiveSize()
	saver := &windowStateSaver{store: rt.Store, logger: rt.Logger}
	return gui.NewWindow(gui.WindowCfg{
		Title:       "Vela",
		Width:       width,
		Height:      height,
		IconPNG:     appIconPNG,
		State:       &app.State{},
		Decorations: chrome.Decoration(rt.Store.Config.General.Frameless),
		OnInit: func(w *gui.Window) {
			rt.BindWindow(w)
			installKeybindCommands(w, rt)
			watchKeybindChanges(w, rt)
			if saved.Maximised {
				w.Maximize()
			}
			frameless := rt.Store.Config.General.Frameless
			gui.State[app.State](w).Frameless = frameless
			w.OnEvent = func(e *gui.Event, w *gui.Window) {
				if e.Type == gui.EventResized {
					saver.schedule(w)
				}
				st := gui.State[app.State](w)
				if !buildinfo.IsRelease() && (e.Type == gui.EventChar || e.Type == gui.EventKeyDown) {
					fields := []zap.Field{
						zap.Int("eventType", int(e.Type)),
						zap.String("focus", w.FocusID()),
					}
					if t := st.Active(); t != nil && t.Kind == app.TabSession {
						if tm := rt.Ctrl.TermFor(t.SessID); tm != nil {
							fields = append(fields,
								zap.String("termLeaf", tm.FocusID()),
								zap.Strings("termResolved", w.ResolveID(tm.FocusID())),
							)
						}
					}
					rt.Logger.Debug("key event routing", fields...)
				}
				rt.Ctrl.HandleWindowEvent(e, st)
			}

			mountShell := func() {
				w.SetView(func(w *gui.Window) gui.View {
					return shell.Root(w, rt.Ctrl, shell.Chrome{
						OnBookmarks: func() { bookmarks.Open(d.Bookmarks) },
						OnSettings:  func() { settings.Open(d.Settings) },
						OnTools:     func() { toolsui.Open(d.Tools) },
						OnCommand:   func() { commandui.Toggle(d.Command) },
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
					saver.flush(win)
					rt.Ctrl.Shutdown()
					win.Close()
				},
			})
		},
		OnCloseRequest: func(w *gui.Window) {
			saver.flush(w)
			rt.Ctrl.Shutdown()
			w.Close()
		},
	})
}

func startUpdateCheck(w *gui.Window, rt *app.Runtime, d *update.Deps) {
	if d == nil || d.Manager == nil || !rt.Store.Config.General.AutoCheckUpdate {
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
