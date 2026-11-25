package app

import (
	"context"
	"errors"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/errtext"
	"github.com/ilaziness/vela/ui/protocol"
)

// paneRef identifies one terminal slot: a tab plus a pane index.
// pane < 0 is the whole-tab target — the dial lands in the tab's
// first free slot, or a fresh tab when tabID is empty.
type paneRef struct {
	tabID string
	pane  int
}

// paneKey scopes the reconnect request of one pane.
func paneKey(tabID string, idx int) string {
	return tabID + "#" + itoa(idx)
}

// paneFillable reports whether a dial may occupy the slot: no live
// session and not mid-dial. A disconnected pane qualifies — a fresh
// dial simply replaces its reconnect state.
func paneFillable(p Pane) bool {
	return p.SessID == "" && p.Status != TabStatusConnecting
}

// slotBusy reports whether the slot holds state worth preserving
// across a layout change: a live session or a pending reconnect.
func slotBusy(p Pane) bool {
	return p.SessID != "" || p.Status == TabStatusDisconnected
}

// dialPaneTarget picks where a fresh connect or bookmark dial lands:
// the active connect form, or the active tab's active pane when that
// slot is free. Otherwise it returns the append-new-tab target.
func dialPaneTarget(st *State) paneRef {
	tab := st.Active()
	if tab == nil {
		return paneRef{pane: -1}
	}
	if tab.Kind == TabConnect {
		return paneRef{tabID: tab.ID, pane: -1}
	}
	tab.EnsurePanes()
	idx := tab.ActivePane
	if idx < 0 || idx >= len(tab.Panes) {
		idx = 0
	}
	if paneFillable(tab.Panes[idx]) {
		return paneRef{tabID: tab.ID, pane: idx}
	}
	return paneRef{pane: -1}
}

// markDialTarget flips the dial target to Connecting so the UI shows
// progress before the goroutine returns.
func markDialTarget(st *State, target paneRef) {
	if target.pane >= 0 {
		if tab := st.TabByID(target.tabID); tab != nil {
			tab.EnsurePanes()
			if target.pane < len(tab.Panes) {
				tab.Panes[target.pane].Status = TabStatusConnecting
				tab.Panes[target.pane].ConnErr = ""
				tab.SyncActive()
			}
		}
		return
	}
	if tab := st.TabByID(target.tabID); tab != nil {
		tab.Status = TabStatusConnecting
		tab.ConnErr = ""
	}
}

// restorePaneAfterDial puts a pane back after a failed or cancelled
// dial: Disconnected when a reconnect request still exists for retry,
// otherwise Idle so the slot is dialable again.
func (c *Controller) restorePaneAfterDial(tab *Tab, idx int, err error) {
	p := &tab.Panes[idx]
	c.mu.Lock()
	_, hasReq := c.reconnect[paneKey(tab.ID, idx)]
	c.mu.Unlock()
	p.ConnErr = ""
	if hasReq {
		p.Status = TabStatusDisconnected
		if err != nil {
			p.ConnErr = errtext.Text(err)
		}
		return
	}
	p.Status = TabStatusIdle
}

// resolvePaneTarget validates the dial target and returns the tab and
// pane index the session will occupy, creating a fresh session tab
// for the append case.
func (c *Controller) resolvePaneTarget(st *State, target paneRef) (*Tab, int, error) {
	if target.pane >= 0 {
		tab := st.TabByID(target.tabID)
		if tab == nil || tab.Kind != TabSession {
			return nil, 0, errors.New(i18n.T("pane.target_gone"))
		}
		tab.EnsurePanes()
		// SessID=="" is the real gate: Status may be Connecting because
		// markDialTarget flagged this very dial; Busy serializes dials.
		if target.pane >= len(tab.Panes) || tab.Panes[target.pane].SessID != "" {
			return nil, 0, errors.New(i18n.T("pane.target_gone"))
		}
		return tab, target.pane, nil
	}
	tab := st.TabByID(target.tabID)
	if tab == nil {
		st.tabSeq++
		st.Tabs = append(st.Tabs, Tab{ID: "tab-sess-" + itoa(st.tabSeq)})
		tab = &st.Tabs[len(st.Tabs)-1]
	}
	if tab.Kind != TabSession {
		tab.Kind = TabSession
	}
	tab.EnsurePanes()
	for i := range tab.Panes {
		if tab.Panes[i].SessID == "" {
			return tab, i, nil
		}
	}
	return nil, 0, errors.New(i18n.T("pane.target_gone"))
}

// ActivateTab selects tabID and syncs terminal focus/caret state to
// the tab's active pane. Every UI path that changes ActiveTab must go
// through here — a bare assignment leaves go-gui focus parked on the
// previous tab's (now unmounted) terminal, so the new tab's session
// never receives keystrokes.
func (c *Controller) ActivateTab(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	st.ActiveTab = tabID
	c.focusActiveTerm(st)
}

// SetActivePane makes pane idx the tab's active slot: Tab.SessID and
// the mirrored session fields switch to it, and terminal focus follows.
func (c *Controller) SetActivePane(w *gui.Window, tabID string, idx int) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.Kind != TabSession {
		return
	}
	tab.EnsurePanes()
	if idx < 0 || idx >= len(tab.Panes) || tab.ActivePane == idx {
		return
	}
	tab.ActivePane = idx
	tab.SyncActive()
	c.focusActiveTerm(st)
}

// activatePaneForSess marks the pane owning sessID active. Used by the
// terminal's click-to-focus hook so panels follow the clicked pane.
func (c *Controller) activatePaneForSess(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	tabID, idx, ok := st.PaneOf(sessID)
	if !ok {
		return
	}
	c.SetActivePane(w, tabID, idx)
}

// SetPaneLayout rearranges the tab's slots. Busy panes (live sessions
// and pending reconnects) keep their relative order and fill the new
// slots first; surplus live sessions each move out to a fresh tab,
// matching the single-pane collapse behavior.
func (c *Controller) SetPaneLayout(w *gui.Window, tabID string, layout PaneLayout) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.Kind != TabSession {
		return
	}
	tab.EnsurePanes()
	if tab.Layout == layout {
		return
	}
	n := paneSlotCount(layout)
	var occ []int
	for i := range tab.Panes {
		if slotBusy(tab.Panes[i]) {
			occ = append(occ, i)
		}
	}
	keepN := min(n, len(occ))
	newPanes := make([]Pane, n)
	for j := 0; j < keepN; j++ {
		newPanes[j] = tab.Panes[occ[j]]
	}
	after := tabID
	for j := keepN; j < len(occ); j++ {
		oldIdx := occ[j]
		if tab.Panes[oldIdx].SessID == "" {
			continue // disconnected surplus: its reconnect state is dropped
		}
		after = c.spawnPaneTab(st, after, paneKey(tabID, oldIdx), tab.Panes[oldIdx])
	}
	// InsertTabAfter may have reallocated st.Tabs; re-resolve.
	tab = st.TabByID(tabID)
	if tab == nil {
		return
	}
	oldCount := len(tab.Panes)
	activeSess := ""
	if tab.ActivePane >= 0 && tab.ActivePane < oldCount {
		activeSess = tab.Panes[tab.ActivePane].SessID
	}
	tab.Panes = newPanes
	tab.Layout = layout
	tab.RatioMain, tab.RatioSub = 0.5, 0.5
	// Re-home reconnect keys and the session index: slot indices change
	// under the new layout.
	c.mu.Lock()
	saved := make(map[int]reconnectReq, oldCount)
	for i := 0; i < oldCount; i++ {
		k := paneKey(tabID, i)
		if req, ok := c.reconnect[k]; ok {
			saved[i] = req
			delete(c.reconnect, k)
		}
	}
	tab.ActivePane = 0
	for j := 0; j < keepN; j++ {
		oldIdx := occ[j]
		if req, ok := saved[oldIdx]; ok {
			c.reconnect[paneKey(tabID, j)] = req
		}
		if s := newPanes[j].SessID; s != "" {
			c.paneLoc[s] = paneRef{tabID: tabID, pane: j}
			if s == activeSess {
				tab.ActivePane = j
			}
		}
	}
	c.mu.Unlock()
	tab.SyncActive()
}

// EvenPaneRatios resets both split ratios to 50/50.
func (c *Controller) EvenPaneRatios(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.RatioMain, tab.RatioSub = 0.5, 0.5
}

// CollapseToSingle keeps pane keepIdx and moves every other live
// session out to its own tab, then flattens the layout.
func (c *Controller) CollapseToSingle(w *gui.Window, tabID string, keepIdx int) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.Kind != TabSession {
		return
	}
	tab.EnsurePanes()
	if keepIdx < 0 || keepIdx >= len(tab.Panes) {
		return
	}
	keep := tab.Panes[keepIdx]
	oldCount := len(tab.Panes)
	after := tabID
	for i := range tab.Panes {
		if i == keepIdx || tab.Panes[i].SessID == "" {
			continue
		}
		after = c.spawnPaneTab(st, after, paneKey(tabID, i), tab.Panes[i])
	}
	if keep.SessID == "" {
		// 单屏 on an empty slot drops the slot instead of leaving a
		// blank single-pane tab; every live session already moved out
		// to its own tab above. Disconnected slots drop their
		// reconnect request along with the tab.
		c.mu.Lock()
		for i := 0; i < oldCount; i++ {
			delete(c.reconnect, paneKey(tabID, i))
		}
		c.mu.Unlock()
		st.RemoveTab(tabID)
		c.ensureConnectTab(st)
		c.focusActiveTerm(st)
		return
	}
	tab = st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.Panes = []Pane{keep}
	tab.Layout = LayoutSingle
	tab.ActivePane = 0
	tab.RatioMain, tab.RatioSub = 0.5, 0.5
	c.mu.Lock()
	saved := make(map[int]reconnectReq, oldCount)
	for i := 0; i < oldCount; i++ {
		k := paneKey(tabID, i)
		if req, ok := c.reconnect[k]; ok {
			saved[i] = req
			delete(c.reconnect, k)
		}
	}
	if req, ok := saved[keepIdx]; ok {
		c.reconnect[paneKey(tabID, 0)] = req
	}
	if keep.SessID != "" {
		c.paneLoc[keep.SessID] = paneRef{tabID: tabID, pane: 0}
	}
	c.mu.Unlock()
	tab.SyncActive()
	c.focusActiveTerm(st)
}

// spawnPaneTab moves one pane's session into a fresh single-pane tab
// inserted after afterID, carrying its reconnect request along.
// Returns the new tab id so callers can chain insertions.
func (c *Controller) spawnPaneTab(st *State, afterID, oldKey string, p Pane) string {
	st.tabSeq++
	id := "tab-sess-" + itoa(st.tabSeq)
	nt := Tab{
		ID:         id,
		Kind:       TabSession,
		Layout:     LayoutSingle,
		Panes:      []Pane{p},
		ActivePane: 0,
		RatioMain:  0.5,
		RatioSub:   0.5,
	}
	nt.SyncActive()
	st.InsertTabAfter(afterID, nt)
	c.mu.Lock()
	if req, ok := c.reconnect[oldKey]; ok {
		delete(c.reconnect, oldKey)
		c.reconnect[paneKey(id, 0)] = req
	}
	if p.SessID != "" {
		c.paneLoc[p.SessID] = paneRef{tabID: id, pane: 0}
	}
	c.mu.Unlock()
	return id
}

// MovePaneTo relocates the session in srcTabID/srcPane into the empty
// slot dstTabID/dstPane — the drop target of a tab-chip drag. The
// source slot becomes an empty pane; a source tab left without any
// session is removed.
func (c *Controller) MovePaneTo(st *State, srcTabID string, srcPane int, dstTabID string, dstPane int) {
	src := st.TabByID(srcTabID)
	dst := st.TabByID(dstTabID)
	if src == nil || dst == nil || src.Kind != TabSession || dst.Kind != TabSession {
		return
	}
	src.EnsurePanes()
	dst.EnsurePanes()
	if srcPane < 0 || srcPane >= len(src.Panes) || dstPane < 0 || dstPane >= len(dst.Panes) {
		return
	}
	if srcTabID == dstTabID && srcPane == dstPane {
		return
	}
	data := src.Panes[srcPane]
	if data.SessID == "" || !paneIsEmpty(dst.Panes[dstPane]) {
		return
	}
	src.Panes[srcPane] = Pane{Status: TabStatusIdle}
	dst.Panes[dstPane] = data
	dst.ActivePane = dstPane
	c.mu.Lock()
	oldKey := paneKey(srcTabID, srcPane)
	if req, ok := c.reconnect[oldKey]; ok {
		delete(c.reconnect, oldKey)
		c.reconnect[paneKey(dstTabID, dstPane)] = req
	}
	c.paneLoc[data.SessID] = paneRef{tabID: dstTabID, pane: dstPane}
	c.mu.Unlock()
	dst.SyncActive()
	src.SyncActive()
	hasSess := false
	for i := range src.Panes {
		if src.Panes[i].SessID != "" {
			hasSess = true
			break
		}
	}
	if !hasSess {
		c.removeEmptySessionTab(st, srcTabID)
	}
	c.focusActiveTerm(st)
}

// removeEmptySessionTab drops a session tab whose panes all went empty,
// releasing its leftover reconnect requests.
func (c *Controller) removeEmptySessionTab(st *State, tabID string) {
	tab := st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.EnsurePanes()
	c.mu.Lock()
	for i := range tab.Panes {
		delete(c.reconnect, paneKey(tabID, i))
	}
	c.mu.Unlock()
	st.RemoveTab(tabID)
	c.ensureConnectTab(st)
}

// ClosePane empties one slot: a live session is torn down (after the
// unsaved-edit check), a disconnected slot drops its reconnect
// request, and an idle slot is a no-op. The layout keeps its shape.
func (c *Controller) ClosePane(w *gui.Window, tabID string, idx int) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.Kind != TabSession {
		return
	}
	tab.EnsurePanes()
	if idx < 0 || idx >= len(tab.Panes) {
		return
	}
	p := tab.Panes[idx]
	switch {
	case p.SessID != "":
		if c.UnsavedEditCount(p.SessID) > 0 {
			c.requestCloseSession(w, p.SessID, func() {
				c.clearPaneNow(w, tabID, idx, p.SessID)
			})
			return
		}
		c.clearPaneNow(w, tabID, idx, p.SessID)
	case p.Status == TabStatusDisconnected:
		c.mu.Lock()
		delete(c.reconnect, paneKey(tabID, idx))
		c.mu.Unlock()
		tab.Panes[idx] = Pane{Status: TabStatusIdle}
		tab.SyncActive()
	default:
		return
	}
}

// clearPaneNow tears down sessID and empties the slot. Runs on the UI
// thread, possibly deferred by an edit-confirm dialog.
func (c *Controller) clearPaneNow(w *gui.Window, tabID string, idx int, sessID string) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.EnsurePanes()
	if idx < 0 || idx >= len(tab.Panes) || tab.Panes[idx].SessID != sessID {
		return // slot refilled or the tab changed mid-dialog
	}
	delete(st.SFTP, sessID)
	c.closeSFTPSession(sessID)
	c.mu.Lock()
	delete(c.paneLoc, sessID)
	c.mu.Unlock()
	c.tearDownSession(sessID)
	c.DropPerf(sessID)
	c.DropTransfers(sessID)
	wasActive := tab.ActivePane == idx
	tab.Panes[idx] = Pane{Status: TabStatusIdle}
	tab.SyncActive()
	if wasActive {
		w.ClearFocus()
	}
	c.focusActiveTerm(st)
}

// ensureConnectTab opens a connect form when the last tab went away.
func (c *Controller) ensureConnectTab(st *State) {
	if len(st.Tabs) != 0 {
		return
	}
	var newDraft func() protocol.Draft
	if e, ok := c.registry.Get(termio.ProtocolSSH); ok && e.NewDraft != nil {
		newDraft = e.NewDraft
	}
	st.OpenConnectTab(newDraft)
}

// ReconnectPane dials a dropped pane again. Runs on the UI thread;
// the dial itself happens on a goroutine.
func (c *Controller) ReconnectPane(w *gui.Window, tabID string, paneIdx int) {
	if w == nil {
		return
	}
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.EnsurePanes()
	if paneIdx < 0 || paneIdx >= len(tab.Panes) || st.Busy {
		return
	}
	if tab.Panes[paneIdx].SessID != "" || tab.Panes[paneIdx].Status == TabStatusConnecting {
		return
	}
	c.mu.Lock()
	req, ok := c.reconnect[paneKey(tabID, paneIdx)]
	c.mu.Unlock()
	if !ok {
		return
	}
	target := paneRef{tabID: tabID, pane: paneIdx}
	if req.bookmarkID != "" {
		// The bookmark path re-checks the vault and owns Busy itself.
		c.beginBookmarkConnect(w, target, req.bookmarkID)
		return
	}
	entry, ok := c.registry.Get(req.draft.Protocol)
	if !ok || !entry.Enabled || entry.Connect == nil {
		tab.Panes[paneIdx].Status = TabStatusDisconnected
		tab.Panes[paneIdx].ConnErr = i18n.T("shell.connect.protocol_unavailable")
		tab.SyncActive()
		return
	}
	st.Busy = true
	st.BusyOp = "connect"
	tab.Panes[paneIdx].Status = TabStatusConnecting
	tab.Panes[paneIdx].ConnErr = ""
	tab.SyncActive()

	go func() {
		sess, err := entry.Connect.Connect(context.Background(), req.draft)
		w.QueueCommand(func(w *gui.Window) {
			s := gui.State[State](w)
			s.Busy = false
			s.BusyOp = ""
			if err != nil {
				c.dialFailed(w, target, err)
				w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
				return
			}
			if s.TabByID(tabID) == nil {
				// The tab was closed mid-dial; drop the fresh session.
				_ = sess.Close()
				c.hub.Unregister(sess.ID())
				return
			}
			if err := c.attachSession(w, s, sess, target, req); err != nil {
				_ = sess.Close()
				c.hub.Unregister(sess.ID())
				c.dialFailed(w, target, err)
				w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
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
