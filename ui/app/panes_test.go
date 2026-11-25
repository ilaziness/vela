package app

import "testing"

func TestResolvePaneTargetConnectingTab(t *testing.T) {
	// Regression: markDialTarget flags the target Connecting before the
	// dial; attach must still accept the slot (gate is SessID=="").
	st := &State{ActiveTab: "t1", Tabs: []Tab{{
		ID: "t1", Kind: TabConnect, Status: TabStatusConnecting,
	}}}
	c := &Controller{}
	tab, idx, err := c.resolvePaneTarget(st, paneRef{tabID: "t1", pane: -1})
	if err != nil {
		t.Fatalf("resolvePaneTarget: %v", err)
	}
	if tab.Kind != TabSession || idx != 0 || len(tab.Panes) != 1 {
		t.Fatalf("got kind=%v idx=%d panes=%d", tab.Kind, idx, len(tab.Panes))
	}
}
