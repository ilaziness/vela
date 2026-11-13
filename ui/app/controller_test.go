package app

import (
	"errors"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"
)

func newTestWindow(t *testing.T, st *State) *gui.Window {
	t.Helper()
	return gui.NewTestWindow(t, gui.WindowCfg{Width: 800, Height: 600, State: st})
}

func TestMarkDisconnectedKeepsTab(t *testing.T) {
	st := &State{
		ActiveTab: "t1",
		Tabs: []Tab{
			{ID: "t1", Kind: TabSession, Title: "box", SessID: "s1", Status: TabStatusConnected},
			{ID: "t2", Kind: TabConnect, Status: TabStatusIdle},
		},
	}
	w := newTestWindow(t, st)
	c := &Controller{}

	c.markDisconnected(w, "s1")

	if len(st.Tabs) != 2 {
		t.Fatalf("tab count = %d, want the tab kept", len(st.Tabs))
	}
	tab := st.TabByID("t1")
	if tab == nil {
		t.Fatal("session tab was removed")
	}
	if tab.SessID != "" {
		t.Fatalf("SessID = %q, want cleared", tab.SessID)
	}
	if tab.Status != TabStatusDisconnected {
		t.Fatalf("status = %v, want disconnected", tab.Status)
	}
	if tab.Kind != TabSession {
		t.Fatalf("kind = %v, want session", tab.Kind)
	}
	if st.ActiveTab != "t1" {
		t.Fatalf("active tab = %q, want unchanged", st.ActiveTab)
	}

	// A second exit for the same session (Term.Close fires it too) is a no-op.
	tab.Status = TabStatusIdle
	c.markDisconnected(w, "s1")
	if tab := st.TabByID("t1"); tab.Status != TabStatusIdle {
		t.Fatalf("second exit changed a detached tab: %v", tab.Status)
	}
}

func TestMarkDisconnectedUnknownSession(t *testing.T) {
	st := &State{
		ActiveTab: "t1",
		Tabs:      []Tab{{ID: "t1", Kind: TabSession, SessID: "s1", Status: TabStatusConnected}},
	}
	w := newTestWindow(t, st)
	c := &Controller{}

	c.markDisconnected(w, "other")

	if tab := st.TabByID("t1"); tab.SessID != "s1" || tab.Status != TabStatusConnected {
		t.Fatalf("unrelated session touched: %+v", *tab)
	}
}

func TestMarkDisconnectedWhileClosing(t *testing.T) {
	st := &State{
		ActiveTab: "t1",
		Tabs:      []Tab{{ID: "t1", Kind: TabSession, SessID: "s1", Status: TabStatusConnected}},
	}
	w := newTestWindow(t, st)
	c := &Controller{closing: true}

	c.markDisconnected(w, "s1")

	if len(st.Tabs) != 1 || st.Tabs[0].SessID != "s1" {
		t.Fatalf("shutdown path mutated tabs: %+v", st.Tabs)
	}
}

func TestDialFailedRestoresDot(t *testing.T) {
	err := errors.New("dial refused")
	cases := []struct {
		name       string
		kind       TabKind
		start      TabStatus
		wantStatus TabStatus
		wantErr    string
	}{
		{
			// A dropped tab must stay red so its reconnect button works again.
			name: "session goes back to disconnected", kind: TabSession,
			start: TabStatusConnecting, wantStatus: TabStatusDisconnected, wantErr: "dial refused",
		},
		{
			// A connect form goes back to grey: it never connected.
			name: "form goes back to idle", kind: TabConnect,
			start: TabStatusConnecting, wantStatus: TabStatusIdle,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := &State{
				ActiveTab: "t1",
				Tabs:      []Tab{{ID: "t1", Kind: tc.kind, Status: tc.start}},
			}
			w := newTestWindow(t, st)
			c := &Controller{logger: zap.NewNop()}

			c.dialFailed(w, "t1", err)

			tab := st.TabByID("t1")
			if tab.Status != tc.wantStatus {
				t.Fatalf("status = %v, want %v", tab.Status, tc.wantStatus)
			}
			if tab.ConnErr != tc.wantErr {
				t.Fatalf("ConnErr = %q, want %q", tab.ConnErr, tc.wantErr)
			}
		})
	}
}

func TestDialFailedUnknownTab(t *testing.T) {
	st := &State{ActiveTab: "", Tabs: []Tab{{ID: "t1", Kind: TabSession, Status: TabStatusConnecting}}}
	w := newTestWindow(t, st)
	c := &Controller{logger: zap.NewNop()}

	c.dialFailed(w, "", errors.New("boom"))

	if tab := st.TabByID("t1"); tab.Status != TabStatusConnecting {
		t.Fatalf("unrelated tab touched: %v", tab.Status)
	}
}
