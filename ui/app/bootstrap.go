package app

import (
	"path/filepath"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/applog"
	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/keybind"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/fonts"
	"github.com/ilaziness/vela/ui/protocol"
	serialui "github.com/ilaziness/vela/ui/protocol/serial"
	sshui "github.com/ilaziness/vela/ui/protocol/ssh"
)

// Runtime holds process-wide dependencies for the native UI.
type Runtime struct {
	App       *gui.App
	Logger    *zap.Logger
	Store     *config.Store
	Hub       *termio.Hub
	SSH       *ssh.Manager
	Serial    *serial.Manager
	Registry  *protocol.Registry
	Ctrl      *Controller
	HostKey   *sshui.HostKeyPrompter
	Keyboard  *sshui.KeyboardPrompter
	DB        *database.Database
	Vault     *secret.Vault
	Keys      *sshkey.Service
	Keybind   *keybind.Manager
	Bookmarks *bookmark.Service
}

// Bootstrap constructs engines and the protocol registry.
func Bootstrap() (*Runtime, error) {
	logger := applog.New()
	store, err := config.Load(logger)
	if err != nil {
		return nil, err
	}
	config.BootstrapLanguage(store, logger)
	ApplyGUILocale()

	db := database.NewDatabase(store.UserDataDir(), logger)
	if err := db.Initialize(); err != nil {
		return nil, err
	}
	vault := secret.NewVault()
	keys := sshkey.New(logger, db, func(reason string) (string, error) {
		return vault.Get()
	}, func() {
		vault.Clear()
	}, nil)
	kb := keybind.New(store)

	hub := termio.NewHub()
	hostKey := sshui.NewHostKeyPrompter(nil, nil)
	keyboard := sshui.NewKeyboardPrompter(nil, nil)
	sshMgr := ssh.NewManager(
		logger,
		filepath.Join(store.UserDataDir(), "known_hosts"),
		hostKey,
		keyboard,
	)
	hostKey.Bind(nil, sshMgr)
	keyboard.Bind(nil, sshMgr)
	sshMgr.SetOptions(ssh.Options{
		ServerAliveInterval: time.Duration(store.Config.SSH.ServerAliveInterval) * time.Second,
		DialTimeout:         time.Duration(store.Config.SSH.DialTimeoutSec) * time.Second,
	})

	globalProxy := func() ssh.GlobalProxy {
		c := store.Config.SSH
		return ssh.GlobalProxy{
			Type: c.ProxyType, Host: c.ProxyHost, Port: c.ProxyPort,
			User: c.ProxyUser, Password: c.ProxyPassword,
		}
	}
	serialMgr := serial.NewManager(logger)

	reg := protocol.NewRegistry()
	reg.Register(sshui.Entry(sshMgr, hub, logger, globalProxy))
	reg.Register(serialui.Entry(serialMgr, hub))

	bmSvc := bookmark.New(logger, db, func(reason string) (string, error) {
		return vault.Get()
	}, nil, func() {
		vault.Clear()
	}, func(id string) (string, error) {
		return keys.PrivatePEM(id)
	}, nil)

	ctrl := NewController(hub, reg, logger)
	ctrl.SetTermDefaults(fonts.TermStyle(store.Config.Terminal), store.Config.Terminal.Scrollback)
	ctrl.SetBookmarkDial(BookmarkDialDeps{
		Bookmarks:   bmSvc,
		SSH:         sshMgr,
		Serial:      serialMgr,
		GlobalProxy: globalProxy,
		Vault:       vault,
		Store:       store,
		DB:          db,
	})
	ctrl.SetShutdown(func() {
		sshMgr.CloseAll()
		serialMgr.CloseAll()
		hub.Clear()
		_ = db.Close()
	})

	return &Runtime{
		Logger:    logger,
		Store:     store,
		Hub:       hub,
		SSH:       sshMgr,
		Serial:    serialMgr,
		Registry:  reg,
		Ctrl:      ctrl,
		HostKey:   hostKey,
		Keyboard:  keyboard,
		DB:        db,
		Vault:     vault,
		Keys:      keys,
		Keybind:   kb,
		Bookmarks: bmSvc,
	}, nil
}

// BindWindow attaches the GUI window to prompters and controller.
func (r *Runtime) BindWindow(w *gui.Window) {
	r.Ctrl.SetWindow(w)
	r.HostKey.SetWindow(w)
	r.Keyboard.SetWindow(w)
}
