package app

import (
	"testing"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
)

func sshSessionTab() Tab {
	return Tab{
		ID: "t1", Kind: TabSession, Title: "box",
		SessID: "s1", Status: TabStatusConnected, Protocol: termio.ProtocolSSH,
	}
}

// The three side panels share the stage width, so opening one must
// close the others; toggling the open one closes it.
func TestTogglePanelExclusive(t *testing.T) {
	st := &State{
		ActiveTab: "t1",
		Tabs:      []Tab{sshSessionTab()},
	}
	w := newTestWindow(t, st)
	c := &Controller{logger: zap.NewNop(), sshMgr: ssh.NewManager(zap.NewNop(), "", nil, nil)}
	defer c.StopPerf("s1")

	c.TogglePanel(w, "t1", PanelTunnel)
	tab := st.TabByID("t1")
	if !tab.TunnelOpen || tab.PerfOpen || tab.TransferOpen {
		t.Fatalf("after tunnel: %+v", *tab)
	}

	// Opening perf closes tunnel.
	c.TogglePanel(w, "t1", PanelPerf)
	tab = st.TabByID("t1")
	if !tab.PerfOpen || tab.TunnelOpen || tab.TransferOpen {
		t.Fatalf("after perf: %+v", *tab)
	}

	// Opening transfer closes perf and tunnel stays closed.
	c.TogglePanel(w, "t1", PanelTransfer)
	tab = st.TabByID("t1")
	if tab.PerfOpen || tab.TunnelOpen || !tab.TransferOpen {
		t.Fatalf("after transfer: %+v", *tab)
	}

	// Toggling the open panel closes it.
	c.TogglePanel(w, "t1", PanelTransfer)
	if tab = st.TabByID("t1"); tab.TransferOpen || tab.PerfOpen || tab.TunnelOpen {
		t.Fatalf("after transfer close: %+v", *tab)
	}
}

func TestTogglePanelRejectsNonSession(t *testing.T) {
	st := &State{
		ActiveTab: "t1",
		Tabs: []Tab{
			{ID: "t1", Kind: TabConnect, Status: TabStatusIdle},
			{ID: "t2", Kind: TabSession, SessID: "", Status: TabStatusIdle},
		},
	}
	w := newTestWindow(t, st)
	c := &Controller{logger: zap.NewNop()}

	c.TogglePanel(w, "t1", PanelTunnel)
	c.TogglePanel(w, "t2", PanelTunnel)
	for _, tab := range st.Tabs {
		if tab.PerfOpen || tab.TunnelOpen || tab.TransferOpen {
			t.Fatalf("panel opened on ineligible tab: %+v", tab)
		}
	}
}
