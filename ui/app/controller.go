package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-term/term"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/agent"
	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/keybind"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/termstream"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/tunnel"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/ui/fonts"
	"github.com/ilaziness/vela/ui/protocol"
	sshui "github.com/ilaziness/vela/ui/protocol/ssh"
)

// Controller owns session hub wiring and terminal widgets.
type Controller struct {
	mu       sync.Mutex
	win      *gui.Window
	hub      *termio.Hub
	registry *protocol.Registry
	logger   *zap.Logger
	terms    map[string]*term.Term
	termCfg  config.TerminalConfig
	// termTheme is the theme new sessions start with; termThemes and
	// termThemeItems are the lazily built go-term theme catalog for the
	// settings dropdown (Default first, then bundled themes).
	termTheme      term.NamedTheme
	termThemes     []term.NamedTheme
	termThemeItems []string
	onClose        func()

	// reconnect remembers how each session tab was dialed so the tab can
	// dial again after a drop. Keyed by tab id; secrets stay in memory
	// only and are dropped when the tab closes.
	reconnect map[string]reconnectReq
	// paneLoc maps a live sessID to its (tab, pane) slot so click-to-
	// focus and disconnect handling resolve panes in O(1).
	paneLoc map[string]paneRef
	// closing suppresses disconnect handling while the app tears down:
	// Term.Close fires OnExit like any other exit.
	closing bool
	// shutdownCh unblocks prompts still waiting on a dialog once the app
	// closes; Shutdown closes it exactly once.
	shutdownCh   chan struct{}
	shutdownOnce sync.Once

	// Bookmark dial / quick-connect.
	bookmarks   *bookmark.Service
	sshMgr      *ssh.Manager
	serialMgr   *serial.Manager
	globalProxy func() ssh.GlobalProxy
	vault       *secret.Vault
	store       *config.Store
	db          *database.Database
	agent       *agent.Service

	// perf holds per-session performance polling state, keyed by
	// sessID. Owned by perf.go; started/stopped from the tool rail.
	perf map[string]*perfMon

	// transfers holds per-session transfer progress, keyed by sessID.
	transfers map[string][]transfer.ProgressData

	// sftpMgr is the SFTP engine; edits tracks remote-edit watchers.
	// Both are wired by SetSFTP during bootstrap.
	sftpMgr *sftp.Manager
	edits   *sftpEditBook

	// keybindMgr is the user shortcut table; wired by SetKeybind during
	// bootstrap before any window exists.
	keybindMgr *keybind.Manager

	// tunnels is the port-forward engine (SetTunnels); tunnelRules
	// caches saved bookmark rules keyed by bookmark id.
	tunnels     *tunnel.Manager
	tunnelRules map[string]*tunnelRuleCache
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
		termCfg:    config.Default().Terminal,
		termTheme:  defaultTermTheme(),
		reconnect:  make(map[string]reconnectReq),
		paneLoc:    make(map[string]paneRef),
		transfers:  make(map[string][]transfer.ProgressData),
		edits:      newEditBook(),
		shutdownCh: make(chan struct{}),
	}
}

// SetTermConfig snapshots the terminal settings used to build every new
// session. It only affects terminals created afterwards — live terminals
// keep their construction-time configuration until restart.
func (c *Controller) SetTermConfig(cfg config.TerminalConfig) {
	cfg.Normalize()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.termCfg = cfg
}

// SetWindow binds the live window (for QueueCommand / dialogs).
func (c *Controller) SetWindow(w *gui.Window) {
	c.mu.Lock()
	c.win = w
	c.mu.Unlock()
}

// SetShutdown registers a process shutdown hook.
func (c *Controller) SetShutdown(fn func()) { c.onClose = fn }

// shutdownDone is closed by Shutdown, so a prompt blocking on a dialog
// returns instead of holding the window open. A Controller built by
// hand (tests) has a nil channel, which never fires.
func (c *Controller) shutdownDone() <-chan struct{} {
	return c.shutdownCh
}

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

// dialFailed restores the dial target after a failed dial or attach,
// and logs the cause. A pane that had a reconnect request goes back to
// red so its reconnect button works again; a fresh slot goes back to
// grey so it can be dialed again.
func (c *Controller) dialFailed(w *gui.Window, target paneRef, err error) {
	if err != nil {
		c.logger.Warn("dial failed", zap.String("target", paneKey(target.tabID, target.pane)), zap.Error(err))
	}
	st := gui.State[State](w)
	if target.pane >= 0 {
		tab := st.TabByID(target.tabID)
		if tab == nil {
			return
		}
		tab.EnsurePanes()
		if target.pane >= len(tab.Panes) {
			return
		}
		c.restorePaneAfterDial(tab, target.pane, err)
		tab.SyncActive()
		return
	}
	tab := st.TabByID(target.tabID)
	if tab == nil {
		return
	}
	if tab.Kind == TabSession {
		tab.EnsurePanes()
		for i := range tab.Panes {
			if tab.Panes[i].Status == TabStatusConnecting {
				tab.Panes[i].Status = TabStatusDisconnected
				tab.Panes[i].ConnErr = errText(err)
				break
			}
		}
		tab.SyncActive()
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

	// Capture the dial target now: by the time the dial returns the
	// user may have switched tabs, and the session must land where it
	// was started from.
	target := paneRef{pane: -1}
	if !testOnly {
		target = dialPaneTarget(st)
		markDialTarget(st, target)
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
				c.dialFailed(w, target, err)
				w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
			})
			return
		}
		req := reconnectReq{draft: draft}
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			st.Busy = false
			st.BusyOp = ""
			if err := c.attachSession(w, st, sess, target, req); err != nil {
				c.dialFailed(w, target, err)
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

// termRuntime snapshots the terminal settings a new Term widget needs, with
// the zero-value fallbacks required when no settings store is attached (tests).
func (c *Controller) termRuntime() (config.TerminalConfig, term.NamedTheme) {
	c.mu.Lock()
	cfg, theme := c.termCfg, c.termTheme
	c.mu.Unlock()
	if cfg.Scrollback <= 0 {
		cfg.Scrollback = 10000
	}
	if theme.Name == "" {
		theme = defaultTermTheme()
	}
	return cfg, theme
}

// attachSession wires sess into a go-term pane and binds it to the
// pane slot captured when the dial started; req is how the pane can
// dial again after a drop.
func (c *Controller) attachSession(w *gui.Window, st *State, sess termio.Session, target paneRef, req reconnectReq) error {
	tab, idx, err := c.resolvePaneTarget(st, target)
	if err != nil {
		return err
	}
	tabID := tab.ID
	termCfg, termTheme := c.termRuntime()

	stream := termstream.New(sess)
	var tm *term.Term
	tm, err = term.NewWithStream(w, term.Cfg{
		NoWindowHandler: true,
		ScrollbackRows:  termCfg.Scrollback,
		TextStyle:       fonts.TermStyle(termCfg),
		Themes:          []term.NamedTheme{termTheme},
		CursorStyle:     cursorStyleOf(termCfg.CursorStyle),
		CursorBlink:     termCfg.CursorBlink,
		CursorLocked:    termCfg.CursorLock,
		// Baked into the Term at construction: StartRecording reads
		// t.cfg.RecordInput, so the setting must arrive here to take
		// effect for manually started recordings too.
		RecordInput: termCfg.RecordInput,
		// The terminal draws on a canvas whose click handler consumes the
		// press, which stops go-gui's press-to-focus walk before it reaches
		// this pane's focusable container. Without this the pane never got
		// the keyboard back once another widget (the AI composer, a filter
		// field) owned focus: every keystroke kept landing there.
		// SetFocus takes an effective ID, and the pane sits under the
		// context-menu wrapper's scope, so resolve the leaf first.
		OnClickFocus: func() {
			if tm == nil {
				return
			}
			c.activatePaneForSess(w, sess.ID())
			if ids := w.ResolveID(tm.FocusID()); len(ids) > 0 {
				w.SetFocus(ids[0])
			}
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
	tm.SetKeyBindings(c.termKeyMap())

	// The tab slice may have shifted while the Term was built;
	// re-resolve before writing the pane.
	tab = st.TabByID(tabID)
	if tab == nil || idx >= len(tab.Panes) || tab.Panes[idx].SessID != "" {
		c.mu.Lock()
		delete(c.terms, sess.ID())
		c.mu.Unlock()
		_ = tm.Close()
		c.hub.Unregister(sess.ID())
		return errors.New(i18n.T("pane.target_gone"))
	}
	label := req.label
	if label == "" {
		label = sess.Label()
	}
	tab.Panes[idx] = Pane{
		SessID:     sess.ID(),
		Status:     TabStatusConnected,
		Label:      label,
		Protocol:   sess.Protocol(),
		BookmarkID: req.bookmarkID,
		OwnerKey:   c.sftpOwnerKey(req),
	}
	tab.Kind = TabSession
	tab.Draft = protocol.Draft{}
	tab.ActivePane = idx
	tab.SFTPOpen = false
	st.ActiveTab = tab.ID
	tab.SyncActive()

	c.mu.Lock()
	c.reconnect[paneKey(tabID, idx)] = req
	c.paneLoc[sess.ID()] = paneRef{tabID: tabID, pane: idx}
	c.mu.Unlock()

	c.focusActiveTerm(st)
	return nil
}

// markDisconnected detaches a dead session but keeps its pane slot, so
// it can offer a reconnect. Only the pane that died is affected; the
// tab's other panes keep running. Called on the UI thread.
func (c *Controller) markDisconnected(w *gui.Window, sessID string) {
	c.mu.Lock()
	closing := c.closing
	c.mu.Unlock()
	if closing {
		return
	}
	st := gui.State[State](w)
	tabID, idx, ok := st.PaneOf(sessID)
	if !ok {
		// The tab was closed by the user before the exit landed.
		return
	}
	tab := st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.EnsurePanes()
	if idx >= len(tab.Panes) || tab.Panes[idx].SessID != sessID {
		return
	}
	wasActive := tab.ActivePane == idx
	wasActiveTab := st.ActiveTab == tab.ID
	c.mu.Lock()
	delete(c.paneLoc, sessID)
	c.mu.Unlock()
	p := &tab.Panes[idx]
	p.SessID = ""
	p.Status = TabStatusDisconnected
	p.ConnErr = ""
	p.Protocol = ""
	p.BookmarkID = ""
	p.OwnerKey = ""
	// Label stays: it feeds the tab title while the pane is down.
	if wasActive {
		// A dead session has nothing to monitor; the user reopens the
		// panel after reconnecting.
		tab.PerfOpen = false
		tab.SFTPOpen = false
		tab.TransferOpen = false
		tab.TunnelOpen = false
	}
	delete(st.SFTP, sessID)
	// The SSH session is already dead: discard unsaved edits and
	// close the SFTP client without prompting.
	c.closeSFTPSession(sessID)
	c.tearDownSession(sessID)
	c.DropPerf(sessID)
	// The reconnect gets a fresh sessID, so this snapshot could never
	// be read again; retryable rows survive in the transfer queue.
	c.DropTransfers(sessID)
	if wasActive && wasActiveTab {
		// The focused Term is leaving the tree; clear focus first so
		// go-gui does not reassign it to the first remaining focusable.
		w.ClearFocus()
	}
	tab.SyncActive()
	if wasActive {
		w.ClearFocus()
	}
	c.focusActiveTerm(st)
}

// TermFor returns the terminal for a session.
func (c *Controller) TermFor(sessID string) *term.Term {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.terms[sessID]
}

// CloseTab closes a UI tab and every session its panes hold. When a
// session has unsaved remote edits the user is asked once per session
// before teardown.
func (c *Controller) CloseTab(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.EnsurePanes()
	var sessIDs []string
	for i := range tab.Panes {
		if tab.Panes[i].SessID != "" {
			sessIDs = append(sessIDs, tab.Panes[i].SessID)
		}
	}
	c.confirmCloseSessions(w, sessIDs, 0, func() {
		c.closeTabNow(w, tabID)
	})
}

// confirmCloseSessions chains the unsaved-edit prompt across every
// session the tab owns, then runs done on the UI thread.
func (c *Controller) confirmCloseSessions(w *gui.Window, sessIDs []string, i int, done func()) {
	if i >= len(sessIDs) {
		done()
		return
	}
	sessID := sessIDs[i]
	if c.UnsavedEditCount(sessID) > 0 {
		c.requestCloseSession(w, sessID, func() {
			c.confirmCloseSessions(w, sessIDs, i+1, done)
		})
		return
	}
	c.confirmCloseSessions(w, sessIDs, i+1, done)
}

// closeTabNow performs the tab removal and session teardown. Runs on
// the UI thread, possibly deferred by an edit-confirm dialog.
func (c *Controller) closeTabNow(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	// Drop focus before the close chip leaves the tree; otherwise go-gui
	// reassigns to the first remaining focusable (icon rail → blue ring).
	w.ClearFocus()
	tab := st.TabByID(tabID)
	var sessIDs []string
	paneCount := 0
	if tab != nil {
		tab.EnsurePanes()
		paneCount = len(tab.Panes)
		for i := range tab.Panes {
			if tab.Panes[i].SessID != "" {
				sessIDs = append(sessIDs, tab.Panes[i].SessID)
			}
		}
	}
	st.RemoveTab(tabID)
	// Forget how each pane dialed: the saved drafts may hold passwords.
	c.mu.Lock()
	for i := 0; i < paneCount; i++ {
		delete(c.reconnect, paneKey(tabID, i))
	}
	for _, sessID := range sessIDs {
		delete(c.paneLoc, sessID)
	}
	c.mu.Unlock()
	for _, sessID := range sessIDs {
		delete(st.SFTP, sessID)
		// Close the SFTP client too: without this the session's file
		// channel would leak after the tab is gone. Edit uploads were
		// already resolved by requestCloseSession when needed.
		c.closeSFTPSession(sessID)
		c.tearDownSession(sessID)
		c.DropPerf(sessID)
		c.DropTransfers(sessID)
	}
	c.ensureConnectTab(st)
	c.focusActiveTerm(st)
}

func (c *Controller) tearDownSession(sessID string) {
	c.mu.Lock()
	tm := c.terms[sessID]
	delete(c.terms, sessID)
	c.mu.Unlock()
	if tm != nil {
		_ = tm.Close()
	}
	closeSess := func() {
		if sess, ok := c.hub.Get(sessID); ok && sess != nil {
			_ = sess.Close()
		}
		c.hub.Unregister(sessID)
	}
	if mgr := c.TunnelManager(); mgr != nil {
		// Signal tunnel listeners first, then close the SSH session so
		// in-flight remote dials fail fast, then wait for tunnel
		// goroutines to drain.
		mgr.StopSessionAfter(sessID, closeSess)
		return
	}
	closeSess()
}

func (c *Controller) focusActiveTerm(st *State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	active := ""
	if t := st.Active(); t != nil && t.Kind == TabSession && !t.SFTPOpen {
		active = t.SessID
	}
	for id, tm := range c.terms {
		if tm == nil {
			continue
		}
		tm.SetFocused(id == active)
	}
	// SetFocused only re-asserts go-gui focus when its flag flips; when
	// the active term was already flagged (re-clicking the active tab,
	// or focus stolen by a sidebar field) resolve and set the effective
	// ID explicitly so keystrokes cannot stay parked elsewhere.
	if active != "" && c.win != nil {
		if tm := c.terms[active]; tm != nil {
			if ids := c.win.ResolveID(tm.FocusID()); len(ids) > 0 {
				c.win.SetFocus(ids[0])
			}
		}
	}
}

// HandleWindowEvent routes events to the focused terminal. While the
// SFTP panel covers the stage the terminal gets nothing: its PTY keeps
// running, it just does not receive input.
func (c *Controller) HandleWindowEvent(e *gui.Event, st *State) {
	t := st.Active()
	if t == nil || t.Kind != TabSession || t.SFTPOpen {
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
	c.paneLoc = make(map[string]paneRef)
	c.closing = true
	c.mu.Unlock()
	// Release prompts blocked on a dialog before their resources go away.
	c.shutdownOnce.Do(func() {
		if c.shutdownCh != nil {
			close(c.shutdownCh)
		}
	})
	// SFTP clients and edit watchers go down before the SSH sessions
	// they ride on; watchers are discarded, never prompted, here.
	c.CloseAllSFTP()
	for _, id := range ids {
		c.tearDownSession(id)
	}
	c.DropAllPerf()
	if mgr := c.TunnelManager(); mgr != nil {
		mgr.StopAll()
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

// sftpOwnerKey scopes the persisted transfer queue for a session:
// the bookmark id for saved entries, else user@host:port from the
// SSH form payload (port defaults to 22).
func (c *Controller) sftpOwnerKey(req reconnectReq) string {
	if req.bookmarkID != "" {
		return req.bookmarkID
	}
	var p sshui.Payload
	switch v := req.draft.Payload.(type) {
	case sshui.Payload:
		p = v
	case *sshui.Payload:
		if v == nil {
			return ""
		}
		p = *v
	default:
		return ""
	}
	port := p.Port
	if port <= 0 {
		port = 22
	}
	return fmt.Sprintf("%s@%s:%d", p.User, p.Host, port)
}

// TransferSnapshot returns a copy of the session's transfer list.
func (c *Controller) TransferSnapshot(sessID string) []transfer.ProgressData {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]transfer.ProgressData, len(c.transfers[sessID]))
	copy(out, c.transfers[sessID])
	return out
}

// DropTransfers discards a session's transfer state (tab closed, session dropped).
func (c *Controller) DropTransfers(sessID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.transfers, sessID)
}

// transferUpsert replaces or appends a progress row by ID.
func transferUpsert(list []transfer.ProgressData, p transfer.ProgressData) []transfer.ProgressData {
	for i := range list {
		if list[i].ID == p.ID {
			list[i] = p
			return list
		}
	}
	return append(list, p)
}

// transferRemove drops a row by ID.
func transferRemove(list []transfer.ProgressData, id string) []transfer.ProgressData {
	out := list[:0]
	for _, p := range list {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return out
}

// Statusf is a small helper for formatted toast messages.
func (c *Controller) Statusf(format string, args ...any) {
	c.SetStatus(fmt.Sprintf(format, args...))
}
