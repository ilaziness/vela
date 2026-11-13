package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-term/term"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/termstream"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/ui/protocol"
)

// Controller owns session hub wiring and terminal widgets.
type Controller struct {
	mu         sync.Mutex
	win        *gui.Window
	hub        *termio.Hub
	registry   *protocol.Registry
	logger     *zap.Logger
	terms      map[string]*term.Term
	termStyle  gui.TextStyle
	scrollback int
	onClose    func()

	// reconnect remembers how each session tab was dialed so the tab can
	// dial again after a drop. Keyed by tab id; secrets stay in memory
	// only and are dropped when the tab closes.
	reconnect map[string]reconnectReq
	// closing suppresses disconnect handling while the app tears down:
	// Term.Close fires OnExit like any other exit.
	closing bool

	// Bookmark dial / quick-connect.
	bookmarks   *bookmark.Service
	sshMgr      *ssh.Manager
	serialMgr   *serial.Manager
	globalProxy func() ssh.GlobalProxy
	vault       *secret.Vault
	store       *config.Store
	db          *database.Database
}

// reconnectReq is how a session tab reached its session: either a
// protocol draft from the connect form, or a saved bookmark id. The
// draft is kept in memory only so a dropped tab can dial again; it may
// hold a typed password, so it must never be logged or persisted.
//
// label is the tab title to show; empty falls back to the session label
// (user@host:port). Bookmark sessions set it to name@user so the tab
// reads as the saved entry, not as the resolved endpoint.
type reconnectReq struct {
	draft      protocol.Draft
	bookmarkID string
	label      string
}

// NewController constructs the app controller.
func NewController(hub *termio.Hub, registry *protocol.Registry, logger *zap.Logger) *Controller {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Controller{
		hub:        hub,
		registry:   registry,
		logger:     logger,
		terms:      make(map[string]*term.Term),
		scrollback: 10000,
		reconnect:  make(map[string]reconnectReq),
	}
}

// SetTermDefaults configures go-term font face/size and scrollback for new sessions.
func (c *Controller) SetTermDefaults(style gui.TextStyle, scrollback int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.termStyle = style
	if scrollback > 0 {
		c.scrollback = scrollback
	}
}

// SetWindow binds the live window (for QueueCommand / dialogs).
func (c *Controller) SetWindow(w *gui.Window) {
	c.mu.Lock()
	c.win = w
	c.mu.Unlock()
}

// SetShutdown registers a process shutdown hook.
func (c *Controller) SetShutdown(fn func()) { c.onClose = fn }

// Window implements protocol.Bus.
func (c *Controller) Window() *gui.Window {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.win
}

// SetStatus implements protocol.Bus via an auto-dismiss toast.
func (c *Controller) SetStatus(msg string) {
	c.toast(gui.ToastInfo, "", msg)
}

func (c *Controller) toast(sev gui.ToastSeverity, title, body string) {
	w := c.Window()
	if w == nil || body == "" {
		return
	}
	w.QueueCommand(func(w *gui.Window) {
		w.Toast(gui.ToastCfg{
			Title:    title,
			Body:     body,
			Severity: sev,
			Duration: 3 * time.Second,
		})
	})
}

// errText formats errors for toast. usererr.Error() is JSON (Wails RPC
// contract); native UI shows the coded message in the current language.
func errText(err error) string {
	if err == nil {
		return ""
	}
	if ue, ok := usererr.As(err); ok {
		if ue.Code != "" {
			if len(ue.Params) == 0 {
				return i18n.T(ue.Code)
			}
			args := make([]any, len(ue.Params))
			for i, p := range ue.Params {
				args[i] = p
			}
			return i18n.Tf(ue.Code, args...)
		}
		return ue.Message
	}
	return err.Error()
}

// Connect implements protocol.Bus.
func (c *Controller) Connect(draft protocol.Draft) {
	c.runConnect(draft, false)
}

// Test implements protocol.Bus.
func (c *Controller) Test(draft protocol.Draft) {
	c.runConnect(draft, true)
}

// Busy implements protocol.Bus.
func (c *Controller) Busy() (bool, string) {
	w := c.Window()
	if w == nil {
		return false, ""
	}
	st := gui.State[State](w)
	return st.Busy, st.BusyOp
}

// dialFailed restores a tab's dot after a failed dial or attach, and logs
// the cause. A session tab goes back to red so its reconnect button works
// again; a connect form goes back to grey.
func (c *Controller) dialFailed(w *gui.Window, tabID string, err error) {
	if err != nil {
		c.logger.Warn("dial failed", zap.String("tab", tabID), zap.Error(err))
	}
	tab := gui.State[State](w).TabByID(tabID)
	if tab == nil {
		return
	}
	if tab.Kind == TabSession {
		tab.Status = TabStatusDisconnected
		tab.ConnErr = errText(err)
		return
	}
	tab.Status = TabStatusIdle
	tab.ConnErr = ""
}

func (c *Controller) runConnect(draft protocol.Draft, testOnly bool) {
	w := c.Window()
	if w == nil {
		return
	}
	entry, ok := c.registry.Get(draft.Protocol)
	if !ok || !entry.Enabled || entry.Connect == nil {
		c.toast(gui.ToastError, "", i18n.T("shell.connect.protocol_unavailable"))
		return
	}
	op := "connect"
	if testOnly {
		op = "test"
	}
	// OnClick is on the UI thread — set Busy here so the next frame
	// shows loading and a second click is ignored.
	st := gui.State[State](w)
	if st.Busy {
		return
	}
	st.Busy = true
	st.BusyOp = op

	// Capture the target tab now: by the time the dial returns the user
	// may have switched tabs, and the session must land where it was
	// started from.
	targetID := ""
	if !testOnly {
		if tab := st.Active(); tab != nil && tab.Kind == TabConnect {
			targetID = tab.ID
			tab.Status = TabStatusConnecting
			tab.ConnErr = ""
		}
	}

	go func() {
		ctx := context.Background()
		if testOnly {
			err := entry.Connect.Test(ctx, draft)
			w.QueueCommand(func(w *gui.Window) {
				st := gui.State[State](w)
				st.Busy = false
				st.BusyOp = ""
				if err != nil {
					w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
					return
				}
				w.Toast(gui.ToastCfg{
					Body:     i18n.T("shell.connect.test_ok"),
					Severity: gui.ToastSuccess,
					Duration: 3 * time.Second,
				})
			})
			return
		}
		sess, err := entry.Connect.Connect(ctx, draft)
		if err != nil {
			w.QueueCommand(func(w *gui.Window) {
				st := gui.State[State](w)
				st.Busy = false
				st.BusyOp = ""
				c.dialFailed(w, targetID, err)
				w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
			})
			return
		}
		req := reconnectReq{draft: draft}
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			st.Busy = false
			st.BusyOp = ""
			if err := c.attachSession(w, st, sess, targetID, req); err != nil {
				c.dialFailed(w, targetID, err)
				w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
				_ = sess.Close()
				c.hub.Unregister(sess.ID())
				return
			}
			w.Toast(gui.ToastCfg{
				Body:     i18n.Tf("shell.connect.connected", sess.Label()),
				Severity: gui.ToastSuccess,
				Duration: 3 * time.Second,
			})
		})
	}()
}

// attachSession wires sess into a go-term pane and binds it to a tab.
// targetTabID is the tab captured when the dial started ("" to append a
// new one); req is how the tab can dial again after a drop.
func (c *Controller) attachSession(w *gui.Window, st *State, sess termio.Session, targetTabID string, req reconnectReq) error {
	c.mu.Lock()
	termStyle := c.termStyle
	scrollback := c.scrollback
	c.mu.Unlock()
	if scrollback <= 0 {
		scrollback = 10000
	}

	stream := termstream.New(sess)
	tm, err := term.NewWithStream(w, term.Cfg{
		NoWindowHandler: true,
		ScrollbackRows:  scrollback,
		TextStyle:       termStyle,
		Themes: []term.NamedTheme{
			{Name: "Default", Theme: term.DefaultTheme},
		},
		OnExit: func() {
			id := sess.ID()
			w.QueueCommand(func(w *gui.Window) {
				if w.CloseRequested() {
					return
				}
				c.markDisconnected(w, id)
			})
		},
	}, stream)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.terms[sess.ID()] = tm
	c.mu.Unlock()

	tab := st.TabByID(targetTabID)
	if tab == nil {
		st.tabSeq++
		st.Tabs = append(st.Tabs, Tab{ID: "tab-sess-" + itoa(st.tabSeq)})
		tab = &st.Tabs[len(st.Tabs)-1]
	}
	// Replace the connect form in place, or revive a dropped session tab
	// that is dialing again.
	tab.Kind = TabSession
	tab.Title = req.label
	if tab.Title == "" {
		tab.Title = sess.Label()
	}
	tab.Protocol = sess.Protocol()
	tab.SessID = sess.ID()
	tab.Draft = protocol.Draft{}
	tab.Status = TabStatusConnected
	tab.ConnErr = ""
	st.ActiveTab = tab.ID

	c.mu.Lock()
	c.reconnect[tab.ID] = req
	c.mu.Unlock()

	c.focusActiveTerm(st)
	return nil
}

// markDisconnected detaches a dead session but keeps its tab, so the
// pane can offer a reconnect. Called on the UI thread via QueueCommand.
func (c *Controller) markDisconnected(w *gui.Window, sessID string) {
	c.mu.Lock()
	closing := c.closing
	c.mu.Unlock()
	if closing {
		return
	}
	st := gui.State[State](w)
	tab := st.TabBySessID(sessID)
	if tab == nil {
		// The tab was closed by the user before the exit landed.
		return
	}
	// Only the active tab owns the focus; a background drop must not
	// steal it from the terminal the user is typing in.
	if st.ActiveTab == tab.ID {
		// The focused Term is leaving the tree; clear focus first so
		// go-gui does not reassign it to the first remaining focusable.
		w.ClearFocus()
	}
	tab.SessID = ""
	tab.Status = TabStatusDisconnected
	tab.ConnErr = ""
	c.tearDownSession(sessID)
	c.focusActiveTerm(st)
}

// TermFor returns the terminal for a session.
func (c *Controller) TermFor(sessID string) *term.Term {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.terms[sessID]
}

// CloseTab closes a UI tab and its session if any.
func (c *Controller) CloseTab(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	var sessID string
	for _, t := range st.Tabs {
		if t.ID == tabID {
			sessID = t.SessID
			break
		}
	}
	// Drop focus before the close chip leaves the tree; otherwise go-gui
	// reassigns to the first remaining focusable (icon rail → blue ring).
	w.ClearFocus()
	st.RemoveTab(tabID)
	// Forget how the tab dialed: the saved draft may hold a password.
	c.mu.Lock()
	delete(c.reconnect, tabID)
	c.mu.Unlock()
	if sessID != "" {
		c.tearDownSession(sessID)
	}
	if len(st.Tabs) == 0 {
		var newDraft func() protocol.Draft
		if e, ok := c.registry.Get(termio.ProtocolSSH); ok && e.NewDraft != nil {
			newDraft = e.NewDraft
		}
		st.OpenConnectTab(newDraft)
	}
	c.focusActiveTerm(st)
}

// ReconnectTab dials a dropped session tab again. Runs on the UI thread;
// the dial itself happens on a goroutine.
func (c *Controller) ReconnectTab(w *gui.Window, tabID string) {
	if w == nil {
		return
	}
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.Status == TabStatusConnecting || st.Busy {
		return
	}
	c.mu.Lock()
	req, ok := c.reconnect[tabID]
	c.mu.Unlock()
	if !ok {
		return
	}
	if req.bookmarkID != "" {
		// The bookmark path re-checks the vault and owns Busy itself.
		c.beginBookmarkConnect(w, tabID, req.bookmarkID)
		return
	}
	entry, ok := c.registry.Get(req.draft.Protocol)
	if !ok || !entry.Enabled || entry.Connect == nil {
		tab.Status = TabStatusDisconnected
		tab.ConnErr = i18n.T("shell.connect.protocol_unavailable")
		return
	}
	st.Busy = true
	st.BusyOp = "connect"
	tab.Status = TabStatusConnecting
	tab.ConnErr = ""

	go func() {
		sess, err := entry.Connect.Connect(context.Background(), req.draft)
		w.QueueCommand(func(w *gui.Window) {
			s := gui.State[State](w)
			s.Busy = false
			s.BusyOp = ""
			if err != nil {
				c.dialFailed(w, tabID, err)
				w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
				return
			}
			if s.TabByID(tabID) == nil {
				// The tab was closed mid-dial; drop the fresh session.
				_ = sess.Close()
				c.hub.Unregister(sess.ID())
				return
			}
			if err := c.attachSession(w, s, sess, tabID, req); err != nil {
				_ = sess.Close()
				c.hub.Unregister(sess.ID())
				c.dialFailed(w, tabID, err)
				w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
				return
			}
			w.Toast(gui.ToastCfg{
				Body:     i18n.Tf("shell.connect.connected", sess.Label()),
				Severity: gui.ToastSuccess,
				Duration: 3 * time.Second,
			})
		})
	}()
}

func (c *Controller) tearDownSession(sessID string) {
	c.mu.Lock()
	tm := c.terms[sessID]
	delete(c.terms, sessID)
	c.mu.Unlock()
	if tm != nil {
		_ = tm.Close()
	}
	if sess, ok := c.hub.Get(sessID); ok && sess != nil {
		_ = sess.Close()
	}
	c.hub.Unregister(sessID)
}

func (c *Controller) focusActiveTerm(st *State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	active := ""
	if t := st.Active(); t != nil && t.Kind == TabSession {
		active = t.SessID
	}
	for id, tm := range c.terms {
		if tm == nil {
			continue
		}
		tm.SetFocused(id == active)
	}
}

// HandleWindowEvent routes events to the focused terminal.
func (c *Controller) HandleWindowEvent(e *gui.Event, st *State) {
	t := st.Active()
	if t == nil || t.Kind != TabSession {
		return
	}
	tm := c.TermFor(t.SessID)
	if tm != nil {
		tm.HandleWindowEvent(e)
	}
}

// Shutdown closes all terminals and sessions.
func (c *Controller) Shutdown() {
	c.mu.Lock()
	ids := make([]string, 0, len(c.terms))
	for id := range c.terms {
		ids = append(ids, id)
	}
	// Drop saved dial requests so no password outlives the window.
	c.reconnect = make(map[string]reconnectReq)
	c.closing = true
	c.mu.Unlock()
	for _, id := range ids {
		c.tearDownSession(id)
	}
	if c.onClose != nil {
		c.onClose()
	}
}

// Registry exposes the protocol registry.
func (c *Controller) Registry() *protocol.Registry { return c.registry }

// NewSSHDraft returns a fresh SSH draft if registered.
func (c *Controller) NewSSHDraft() protocol.Draft {
	if e, ok := c.registry.Get(termio.ProtocolSSH); ok && e.NewDraft != nil {
		return e.NewDraft()
	}
	return protocol.Draft{Protocol: termio.ProtocolSSH}
}

// Statusf is a small helper for formatted toast messages.
func (c *Controller) Statusf(format string, args ...any) {
	c.SetStatus(fmt.Sprintf(format, args...))
}
