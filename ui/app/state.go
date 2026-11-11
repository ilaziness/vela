package app

import (
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/protocol"
)

// TabKind distinguishes connect form tabs from live sessions.
type TabKind int

const (
	TabConnect TabKind = iota
	TabSession
)

// Tab is one main-area tab.
type Tab struct {
	ID       string
	Title    string
	Kind     TabKind
	Protocol termio.Protocol
	Draft    protocol.Draft
	SessID   string
}

// State is the per-window gui.State root (immediate-mode data only).
type State struct {
	ActiveNav   string
	SideVisible bool
	Tabs        []Tab
	ActiveTab   string
	Busy        bool
	BusyOp      string // "connect" | "test" while Busy
	SearchQuery string
	tabSeq      int
	defaultsOK  bool
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
		Title:    "新建连接…",
		Kind:     TabConnect,
		Protocol: draft.Protocol,
		Draft:    draft,
	})
	s.ActiveTab = id
	return id
}

// Active returns the active tab or nil.
func (s *State) Active() *Tab {
	for i := range s.Tabs {
		if s.Tabs[i].ID == s.ActiveTab {
			return &s.Tabs[i]
		}
	}
	return nil
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
