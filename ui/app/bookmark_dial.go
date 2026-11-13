package app

import (
	"errors"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/internal/vaultcheck"
	"github.com/ilaziness/vela/ui/vaultprompt"
)

// BookmarkDialDeps wires engines needed to test/connect bookmarks.
type BookmarkDialDeps struct {
	Bookmarks   *bookmark.Service
	SSH         *ssh.Manager
	Serial      *serial.Manager
	GlobalProxy func() ssh.GlobalProxy
	Vault       *secret.Vault
	Store       *config.Store
	DB          *database.Database
}

// SetBookmarkDial injects bookmark dial dependencies.
func (c *Controller) SetBookmarkDial(d BookmarkDialDeps) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bookmarks = d.Bookmarks
	c.sshMgr = d.SSH
	c.serialMgr = d.Serial
	c.globalProxy = d.GlobalProxy
	c.vault = d.Vault
	c.store = d.Store
	c.db = d.DB
	if c.globalProxy == nil {
		c.globalProxy = func() ssh.GlobalProxy { return ssh.GlobalProxy{} }
	}
}

// ReloadBookmarkGroups loads grouped bookmarks into window state.
// Call only from event handlers (open side panel / refresh), not from View build.
func (c *Controller) ReloadBookmarkGroups(w *gui.Window) {
	c.mu.Lock()
	svc := c.bookmarks
	logger := c.logger
	c.mu.Unlock()
	if svc == nil {
		return
	}
	groups, err := svc.ListGrouped()
	if err != nil {
		logger.Error("reload bookmark groups", zap.Error(err))
		w.Toast(gui.ToastCfg{
			Body:     i18n.Tf("bookmark.load_failed", errText(err)),
			Severity: gui.ToastError,
			Duration: 4 * time.Second,
		})
		return
	}
	gui.State[State](w).BookmarkGroups = groups
}

// TestBookmark dials using the form draft (PrepareTest restores masked secrets).
// It does not open a session tab. onDone runs on a background goroutine.
func (c *Controller) TestBookmark(b bookmark.Bookmark, onDone func(error)) {
	go func() {
		err := c.testBookmarkSync(b)
		if onDone != nil {
			onDone(err)
		}
	}()
}

// ConnectBookmark unlocks the vault if needed, dials the bookmark, and attaches a session tab.
// It reuses the active connect form tab when there is one.
func (c *Controller) ConnectBookmark(w *gui.Window, id string) {
	if id == "" {
		return
	}
	targetID := ""
	if tab := gui.State[State](w).Active(); tab != nil && tab.Kind == TabConnect {
		targetID = tab.ID
	}
	c.beginBookmarkConnect(w, targetID, id)
}

// beginBookmarkConnect is the shared vault gate + dial entry for bookmarks.
// targetTabID is the tab the session should land in ("" appends a new one).
func (c *Controller) beginBookmarkConnect(w *gui.Window, targetTabID, id string) {
	if id == "" {
		return
	}
	st := gui.State[State](w)
	if st.Busy {
		return
	}
	c.mu.Lock()
	vault := c.vault
	store := c.store
	db := c.db
	c.mu.Unlock()
	if vault == nil || store == nil {
		return
	}
	st.Busy = true
	st.BusyOp = "connect"
	if tab := st.TabByID(targetTabID); tab != nil {
		tab.Status = TabStatusConnecting
		tab.ConnErr = ""
	}

	clearBusy := func(win *gui.Window) {
		s := gui.State[State](win)
		s.Busy = false
		s.BusyOp = ""
	}

	cfg := vaultprompt.UnlockCfg{
		Vault:       vault,
		UserDataDir: store.UserDataDir(),
		Reason:      i18n.T("password.encrypt_decrypt_required"),
		OnUnlocked: func(win *gui.Window) {
			go c.runConnectBookmark(win, targetTabID, id)
		},
		OnFail: func(win *gui.Window, err error) {
			clearBusy(win)
			c.dialFailed(win, targetTabID, err)
			win.Toast(gui.ToastCfg{
				Body:     errText(err),
				Severity: gui.ToastError,
				Duration: 4 * time.Second,
			})
		},
		OnCancel: func(win *gui.Window) {
			clearBusy(win)
			c.dialFailed(win, targetTabID, nil)
		},
	}
	if vaultcheck.HasEncryptedSecrets(store, db) {
		cfg.Probe = func(pw string) bool {
			return vaultcheck.ProbePassword(store, db, pw)
		}
	}
	vaultprompt.Unlock(w, cfg)
}

func (c *Controller) runConnectBookmark(w *gui.Window, targetTabID, id string) {
	sess, label, err := c.connectBookmarkSync(id)
	w.QueueCommand(func(win *gui.Window) {
		st := gui.State[State](win)
		st.Busy = false
		st.BusyOp = ""
		if err != nil {
			c.dialFailed(win, targetTabID, err)
			win.Toast(gui.ToastCfg{
				Body:     errText(err),
				Severity: gui.ToastError,
				Duration: 4 * time.Second,
			})
			return
		}
		req := reconnectReq{bookmarkID: id, label: label}
		if err := c.attachSession(win, st, sess, targetTabID, req); err != nil {
			c.dialFailed(win, targetTabID, err)
			win.Toast(gui.ToastCfg{
				Body:     errText(err),
				Severity: gui.ToastError,
				Duration: 4 * time.Second,
			})
			_ = sess.Close()
			c.hub.Unregister(sess.ID())
			return
		}
		win.Toast(gui.ToastCfg{
			Body:     i18n.Tf("shell.connect.connected", sess.Label()),
			Severity: gui.ToastSuccess,
			Duration: 3 * time.Second,
		})
	})
}

// connectBookmarkSync dials a saved bookmark. The second result is the
// tab title for that bookmark (name@user), empty when it has no name.
func (c *Controller) connectBookmarkSync(id string) (termio.Session, string, error) {
	c.mu.Lock()
	svc := c.bookmarks
	sshMgr := c.sshMgr
	serialMgr := c.serialMgr
	globalProxy := c.globalProxy
	hub := c.hub
	c.mu.Unlock()
	if svc == nil {
		return nil, "", errors.New("bookmark dial is not configured")
	}
	b, err := svc.GetMasked(id)
	if err != nil {
		return nil, "", err
	}
	label := bookmarkTabLabel(b)
	proto, ok := termio.ParseProtocol(b.Protocol)
	if !ok {
		return nil, "", usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", b.Protocol)
	}
	switch proto {
	case termio.ProtocolSSH:
		if sshMgr == nil || globalProxy == nil {
			return nil, "", errors.New("bookmark dial is not configured")
		}
		if err := bookmark.ValidateProxy(*b); err != nil {
			return nil, "", err
		}
		prep, err := svc.PrepareTest(*b)
		if err != nil {
			return nil, "", err
		}
		hops, err := svc.ResolveHops(prep.Endpoint, prep.JumpID)
		if err != nil {
			return nil, "", err
		}
		proxy, err := ssh.ResolveDialProxy(b.ProxyMode, ssh.ProxyConfig{
			Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort,
			User: b.ProxyUser, Password: prep.ProxyPassword,
		}, globalProxy())
		if err != nil {
			return nil, "", ssh.MapUserError(err)
		}
		sessID, err := sshMgr.Connect(hops, proxy)
		if err != nil {
			return nil, "", ssh.MapUserError(err)
		}
		sess, err := sshMgr.GetSession(sessID)
		if err != nil {
			_ = sshMgr.CloseSession(sessID)
			return nil, "", ssh.MapUserError(err)
		}
		hs := ssh.AsTermio(sess)
		hub.Register(hs)
		return hs, label, nil
	case termio.ProtocolSerial:
		if serialMgr == nil {
			return nil, "", errors.New("bookmark dial is not configured")
		}
		sessID, err := serialMgr.Connect(serial.OpenConfig{
			Device:      b.SerialDevice,
			BaudRate:    b.BaudRate,
			DataBits:    b.DataBits,
			Parity:      b.Parity,
			StopBits:    b.StopBits,
			FlowControl: b.FlowControl,
			Charset:     b.Charset,
		})
		if err != nil {
			return nil, "", err
		}
		sess, err := serialMgr.GetSession(sessID)
		if err != nil {
			_ = serialMgr.CloseSession(sessID)
			return nil, "", err
		}
		hub.Register(sess)
		return sess, label, nil
	default:
		return nil, "", usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", b.Protocol)
	}
}

// bookmarkTabLabel is the tab title for a bookmark session: the saved
// name, plus the login user when the bookmark has one (serial entries
// have no user).
func bookmarkTabLabel(b *bookmark.Bookmark) string {
	if b.Title == "" {
		return ""
	}
	if b.User == "" {
		return b.Title
	}
	return b.Title + "@" + b.User
}

func (c *Controller) testBookmarkSync(b bookmark.Bookmark) error {
	c.mu.Lock()
	svc := c.bookmarks
	sshMgr := c.sshMgr
	serialMgr := c.serialMgr
	globalProxy := c.globalProxy
	c.mu.Unlock()
	if svc == nil {
		return errors.New("bookmark dial is not configured")
	}

	proto, ok := termio.ParseProtocol(b.Protocol)
	if !ok {
		return usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", b.Protocol)
	}
	switch proto {
	case termio.ProtocolSSH:
		if sshMgr == nil || globalProxy == nil {
			return errors.New("bookmark dial is not configured")
		}
		if err := bookmark.ValidateProxy(b); err != nil {
			return err
		}
		prep, err := svc.PrepareTest(b)
		if err != nil {
			return err
		}
		hops, err := svc.ResolveHops(prep.Endpoint, prep.JumpID)
		if err != nil {
			return err
		}
		proxy, err := ssh.ResolveDialProxy(b.ProxyMode, ssh.ProxyConfig{
			Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort,
			User: b.ProxyUser, Password: prep.ProxyPassword,
		}, globalProxy())
		if err != nil {
			return ssh.MapUserError(err)
		}
		return ssh.MapUserError(sshMgr.TestConnect(hops, proxy))
	case termio.ProtocolSerial:
		if serialMgr == nil {
			return errors.New("bookmark dial is not configured")
		}
		return serialMgr.TestConnect(serial.OpenConfig{
			Device:      b.SerialDevice,
			BaudRate:    b.BaudRate,
			DataBits:    b.DataBits,
			Parity:      b.Parity,
			StopBits:    b.StopBits,
			FlowControl: b.FlowControl,
			Charset:     b.Charset,
		})
	default:
		return usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", b.Protocol)
	}
}
