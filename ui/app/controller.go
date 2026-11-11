package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-term/term"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/termstream"
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

func (c *Controller) runConnect(draft protocol.Draft, testOnly bool) {
	w := c.Window()
	if w == nil {
		return
	}
	entry, ok := c.registry.Get(draft.Protocol)
	if !ok || !entry.Enabled || entry.Connect == nil {
		c.toast(gui.ToastError, "", "协议不可用")
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

	go func() {
		ctx := context.Background()
		if testOnly {
			err := entry.Connect.Test(ctx, draft)
			w.QueueCommand(func(w *gui.Window) {
				st := gui.State[State](w)
				st.Busy = false
				st.BusyOp = ""
				if err != nil {
					w.Toast(gui.ToastCfg{Body: err.Error(), Severity: gui.ToastError, Duration: 4 * time.Second})
					return
				}
				w.Toast(gui.ToastCfg{Body: "测试连接成功", Severity: gui.ToastSuccess, Duration: 3 * time.Second})
			})
			return
		}
		sess, err := entry.Connect.Connect(ctx, draft)
		if err != nil {
			w.QueueCommand(func(w *gui.Window) {
				st := gui.State[State](w)
				st.Busy = false
				st.BusyOp = ""
				w.Toast(gui.ToastCfg{Body: err.Error(), Severity: gui.ToastError, Duration: 4 * time.Second})
			})
			return
		}
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			st.Busy = false
			st.BusyOp = ""
			if err := c.attachSession(w, st, sess); err != nil {
				w.Toast(gui.ToastCfg{Body: err.Error(), Severity: gui.ToastError, Duration: 4 * time.Second})
				_ = sess.Close()
				c.hub.Unregister(sess.ID())
				return
			}
			w.Toast(gui.ToastCfg{
				Body:     "已连接: " + sess.Label(),
				Severity: gui.ToastSuccess,
				Duration: 3 * time.Second,
			})
		})
	}()
}

func (c *Controller) attachSession(w *gui.Window, st *State, sess termio.Session) error {
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
				c.CloseSessionTab(w, id)
			})
		},
	}, stream)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.terms[sess.ID()] = tm
	c.mu.Unlock()

	// Reuse the active connect form tab so the terminal replaces the
	// form in place (no extra tab).
	if tab := st.Active(); tab != nil && tab.Kind == TabConnect {
		tab.Kind = TabSession
		tab.Title = sess.Label()
		tab.Protocol = sess.Protocol()
		tab.SessID = sess.ID()
		tab.Draft = protocol.Draft{}
	} else {
		st.tabSeq++
		tabID := "tab-sess-" + itoa(st.tabSeq)
		st.Tabs = append(st.Tabs, Tab{
			ID:       tabID,
			Title:    sess.Label(),
			Kind:     TabSession,
			Protocol: sess.Protocol(),
			SessID:   sess.ID(),
		})
		st.ActiveTab = tabID
	}
	c.focusActiveTerm(st)
	return nil
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

// CloseSessionTab finds a session tab by session id and closes it.
func (c *Controller) CloseSessionTab(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	for _, t := range st.Tabs {
		if t.SessID == sessID {
			c.CloseTab(w, t.ID)
			return
		}
	}
	c.tearDownSession(sessID)
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
