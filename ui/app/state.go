package app

import (
	"github.com/ilaziness/vela/internal/agent"
	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/termio"
	agentmodel "github.com/ilaziness/vela/ui/agent/model"
	"github.com/ilaziness/vela/ui/chrome"
	"github.com/ilaziness/vela/ui/protocol"
)

// TabKind distinguishes connect form tabs from live sessions.
type TabKind int

const (
	TabConnect TabKind = iota
	TabSession
)

// TabStatus is the connection state shown as the tab's status dot.
type TabStatus int

const (
	// TabStatusIdle is the zero value: a connect form, or a tab that has
	// never started dialing. Rendered grey.
	TabStatusIdle TabStatus = iota
	TabStatusConnecting
	TabStatusConnected
	TabStatusDisconnected
)

// PaneLayout is the split arrangement of a session tab's panes.
type PaneLayout int

const (
	// LayoutSingle is one pane filling the stage.
	LayoutSingle PaneLayout = iota
	// LayoutSplitH is a left|right pair.
	LayoutSplitH
	// LayoutSplitV is a top/bottom pair.
	LayoutSplitV
	// LayoutGrid2x2 is four quadrants: top-left, top-right,
	// bottom-left, bottom-right in pane index order 0..3.
	LayoutGrid2x2
)

// Pane is one terminal slot inside a session tab. A pane may be empty
// (Status Idle, no session) and act as a dial target, disconnected
// (Status Disconnected, awaiting ReconnectPane), or live.
type Pane struct {
	SessID     string
	Status     TabStatus
	ConnErr    string // last connect/reconnect error for this pane, localized
	Label      string // session label; feeds the tab title fallback
	Protocol   termio.Protocol
	BookmarkID string
	OwnerKey   string
}

// paneIsEmpty reports whether the slot holds no session and no
// reconnect state — the only state a drop or dial may fill.
func paneIsEmpty(p Pane) bool {
	return p.SessID == "" && p.Status == TabStatusIdle
}

// Tab is one main-area tab.
type Tab struct {
	ID       string
	Title    string
	Kind     TabKind
	Status   TabStatus
	ConnErr  string // last connect/reconnect error, already localized
	Protocol termio.Protocol
	Draft    protocol.Draft
	SessID   string
	// PerfOpen remembers whether this tab's performance panel is shown
	// on the right-side tool rail.
	PerfOpen bool
	// SFTPOpen remembers whether this tab's SFTP panel covers the
	// terminal stage. The terminal keeps running underneath.
	SFTPOpen bool
	// TransferOpen remembers whether this tab's transfer panel is shown
	// on the right-side tool rail.
	TransferOpen bool
	// TunnelOpen remembers whether this tab's tunnel panel is shown
	// on the right-side tool rail.
	TunnelOpen bool
	// BookmarkID scopes persisted tunnel rules for this session: set
	// when the tab was dialed from a saved bookmark, empty for quick
	// connects (whose tunnels stay temporary).
	BookmarkID string
	// OwnerKey scopes persisted transfer-queue rows for this session
	// (bookmark id, or user@host:port). Empty until the session binds.
	OwnerKey string
	// Panes holds the tab's terminal slots. Session tabs always keep at
	// least one pane; connect-form tabs leave it nil. Tab.SessID and the
	// per-session fields above mirror Panes[ActivePane] so existing
	// single-session readers keep working.
	Panes []Pane
	// Layout is the pane arrangement; meaningful only for TabSession.
	Layout PaneLayout
	// ActivePane indexes Panes; the pane owning Tab.SessID.
	ActivePane int
	// RatioMain splits the outer divider (left|right for SplitH/Grid,
	// top|bottom for SplitV); RatioSub splits each half of Grid2x2.
	RatioMain float32
	RatioSub  float32
}

// PaneRect is a pane cell's absolute on-window geometry, refreshed
// every frame via AmendLayout. The tab-chip drag reads it on MouseUp
// to hit-test drop targets.
type PaneRect struct {
	X, Y, W, H float32
}

// DragPane tracks an in-flight tab-chip drag toward a pane slot;
// SrcTabID "" means no drag.
type DragPane struct {
	SrcTabID string
	SrcPane  int
	// HoverTabID/HoverPane mark the empty slot currently under the
	// pointer during a chip drag, for drop-target highlighting.
	HoverTabID string
	HoverPane  int
}

// State is the per-window gui.State root (immediate-mode data only).
type State struct {
	ActiveNav      string
	SideVisible    bool
	Tabs           []Tab
	ActiveTab      string
	Busy           bool
	BusyOp         string // "connect" | "test" while Busy
	SearchQuery    string
	BookmarkGroups []*bookmark.Group
	// AgentOpen shows the AI assistant side panel docked on the right edge.
	AgentOpen bool
	// AgentRatio is the terminal share of the stage/agent splitter (0..1);
	// the agent pane is clamped to at most 60% of the stage width.
	AgentRatio float32
	// AgentSessionID selects the active AI chat session.
	AgentSessionID string
	// AgentSessions feeds the session picker list.
	AgentSessions []*agent.AISession
	// AgentChat is the live transcript model for AgentSessionID.
	AgentChat *agentmodel.Chat
	// AgentHistoryOpen shows the session-history sub-panel inside the
	// assistant sidebar.
	AgentHistoryOpen bool
	// AgentDraft is the composer text.
	AgentDraft string
	// AgentAttachments are files queued in the composer. Each path is the
	// original absolute path; nothing is copied into the user data dir.
	AgentAttachments []agentmodel.Attachment
	// AgentSeenRev is the last transcript Rev the view auto-scrolled for.
	AgentSeenRev int
	// AgentScrollChat is the transcript the view has already positioned
	// at its end; a different chat starts at the bottom on first paint.
	AgentScrollChat *agentmodel.Chat
	// AgentScrollPushed reports that the bottom pull for AgentScrollChat
	// was issued at least once. A pull reads the height laid out on the
	// previous frame, so the first one is not enough to confirm the
	// position when a longer chat replaces a shorter one.
	AgentScrollPushed bool
	// AgentExpanded tracks open ExpandPanel sections (reasoning, tool IO)
	// keyed by "reasoning-<msgID>-<blockIdx>" / "tool-<callID>-in|out".
	AgentExpanded map[string]bool

	// SFTP holds per-session SFTP panel state, keyed by session id.
	// Entries are created lazily when a panel opens and survive tab
	// switches; they are dropped when the session dies.
	SFTP map[string]*SFTPState

	// Frameless and BarClicks are window chrome captured at creation:
	// WindowCfg.Decorations is fixed when the window is made, so
	// Frameless reflects the config the window was built with, not
	// later settings drafts. BarClicks feeds the tab bar's
	// double-click-to-maximize detection.
	Frameless bool
	BarClicks chrome.ClickTracker

	// UpdatePrompted ensures the background update check pops the dialog
	// at most once per process.
	UpdatePrompted bool

	// DragPane is non-nil-equivalent state for an in-flight tab-chip
	// drag (SrcTabID "" means no drag).
	DragPane DragPane

	// PaneRects maps "tabID#paneIdx" to the cell's last-known geometry.
	PaneRects map[string]PaneRect

	tabSeq     int
	defaultsOK bool
}

// EnsureDefaults seeds first-run state.
func (s *State) EnsureDefaults(newDraft func() protocol.Draft) {
	if !s.defaultsOK {
		s.defaultsOK = true
		s.SideVisible = false
		s.ActiveNav = ""
	}
	if len(s.Tabs) == 0 {
		s.OpenConnectTab(newDraft)
	}
	if s.SFTP == nil {
		s.SFTP = make(map[string]*SFTPState)
	}
	if s.AgentExpanded == nil {
		s.AgentExpanded = make(map[string]bool)
	}
	if s.PaneRects == nil {
		s.PaneRects = make(map[string]PaneRect)
	}
}

// OpenConnectTab appends a new connection form tab and activates it.
func (s *State) OpenConnectTab(newDraft func() protocol.Draft) string {
	s.tabSeq++
	id := "tab-new-" + itoa(s.tabSeq)
	draft := protocol.Draft{Protocol: termio.ProtocolSSH}
	if newDraft != nil {
		draft = newDraft()
	}
	s.Tabs = append(s.Tabs, Tab{
		ID:       id,
		Kind:     TabConnect,
		Protocol: draft.Protocol,
		Draft:    draft,
	})
	s.ActiveTab = id
	return id
}

// Active returns the active tab or nil.
func (s *State) Active() *Tab {
	return s.TabByID(s.ActiveTab)
}

// TabByID returns the tab with the given id, or nil.
func (s *State) TabByID(id string) *Tab {
	for i := range s.Tabs {
		if s.Tabs[i].ID == id {
			return &s.Tabs[i]
		}
	}
	return nil
}

// TabBySessID returns the session tab holding sessID in any pane, or nil.
func (s *State) TabBySessID(sessID string) *Tab {
	if sessID == "" {
		return nil
	}
	for i := range s.Tabs {
		t := &s.Tabs[i]
		if t.Kind != TabSession {
			continue
		}
		t.EnsurePanes()
		for j := range t.Panes {
			if t.Panes[j].SessID == sessID {
				return t
			}
		}
	}
	return nil
}

// PaneOf locates sessID across all tabs and panes.
func (s *State) PaneOf(sessID string) (tabID string, idx int, ok bool) {
	if sessID == "" {
		return "", 0, false
	}
	for i := range s.Tabs {
		t := &s.Tabs[i]
		if t.Kind != TabSession {
			continue
		}
		t.EnsurePanes()
		for j := range t.Panes {
			if t.Panes[j].SessID == sessID {
				return t.ID, j, true
			}
		}
	}
	return "", 0, false
}

// InsertTabAfter inserts t directly after the tab with id afterID;
// appends when afterID is empty or unknown.
func (s *State) InsertTabAfter(afterID string, t Tab) {
	for i := range s.Tabs {
		if s.Tabs[i].ID == afterID {
			s.Tabs = append(s.Tabs, Tab{})
			copy(s.Tabs[i+2:], s.Tabs[i+1:])
			s.Tabs[i+1] = t
			return
		}
	}
	s.Tabs = append(s.Tabs, t)
}

// RemoveTab drops a tab by id.
func (s *State) RemoveTab(id string) {
	out := s.Tabs[:0]
	for _, t := range s.Tabs {
		if t.ID != id {
			out = append(out, t)
		}
	}
	s.Tabs = out
	if s.ActiveTab == id {
		s.ActiveTab = ""
		if len(s.Tabs) > 0 {
			s.ActiveTab = s.Tabs[len(s.Tabs)-1].ID
		}
	}
}

// EnsurePanes lazily migrates a session tab that predates panes (or a
// test-constructed Tab) into a single-pane layout mirroring its flat
// fields. No-op for connect-form tabs and tabs that already have panes.
func (t *Tab) EnsurePanes() {
	if t.Kind != TabSession || len(t.Panes) > 0 {
		return
	}
	p := Pane{
		SessID:     t.SessID,
		Status:     t.Status,
		ConnErr:    t.ConnErr,
		Label:      t.Title,
		Protocol:   t.Protocol,
		BookmarkID: t.BookmarkID,
		OwnerKey:   t.OwnerKey,
	}
	if p.SessID != "" && p.Status == TabStatusIdle {
		p.Status = TabStatusConnected
	}
	t.Panes = []Pane{p}
	t.Layout = LayoutSingle
	t.ActivePane = 0
	t.RatioMain, t.RatioSub = 0.5, 0.5
}

// paneSlotCount maps a layout to its slot count.
func paneSlotCount(l PaneLayout) int {
	switch l {
	case LayoutSplitH, LayoutSplitV:
		return 2
	case LayoutGrid2x2:
		return 4
	default:
		return 1
	}
}

// SyncActive mirrors Panes[ActivePane] onto the tab's flat session
// fields and refreshes the aggregate status and title. Every mutation
// of pane occupancy or ActivePane must end here.
func (t *Tab) SyncActive() {
	if t.Kind != TabSession {
		return
	}
	t.EnsurePanes()
	if t.ActivePane < 0 || t.ActivePane >= len(t.Panes) {
		t.ActivePane = 0
	}
	p := t.Panes[t.ActivePane]
	t.SessID = p.SessID
	t.Protocol = p.Protocol
	t.BookmarkID = p.BookmarkID
	t.OwnerKey = p.OwnerKey
	t.ConnErr = p.ConnErr
	t.Status = aggregatePaneStatus(t.Panes)
	t.Title = firstPaneLabel(t.Panes, t.Title)
}

// aggregatePaneStatus rolls pane states up to the tab dot: any live
// session wins, then any dial in flight, then any past disconnect.
func aggregatePaneStatus(panes []Pane) TabStatus {
	var sawDisc bool
	for _, p := range panes {
		switch p.Status {
		case TabStatusConnected:
			return TabStatusConnected
		case TabStatusConnecting:
			// keep scanning: a connected sibling outranks connecting
		default:
			if p.Status == TabStatusDisconnected {
				sawDisc = true
			}
		}
	}
	for _, p := range panes {
		if p.Status == TabStatusConnecting {
			return TabStatusConnecting
		}
	}
	if sawDisc {
		return TabStatusDisconnected
	}
	return TabStatusIdle
}

// firstPaneLabel picks the title from the first pane that has a label,
// falling back to the existing title when none does.
func firstPaneLabel(panes []Pane, fallback string) string {
	for _, p := range panes {
		if p.Label != "" {
			return p.Label
		}
	}
	return fallback
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
