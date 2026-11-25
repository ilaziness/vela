package session

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/app"
)

func findID(ly *gui.Layout, id string) (*gui.Layout, bool) {
	if ly == nil || ly.Shape == nil {
		return nil, false
	}
	if ly.Shape.ID == id {
		return ly, true
	}
	for i := range ly.Children {
		if found, ok := findID(&ly.Children[i], id); ok {
			return found, true
		}
	}
	return nil, false
}

// textUnder returns the first text drawn at or below ly.
func textUnder(ly *gui.Layout) (string, bool) {
	if ly == nil {
		return "", false
	}
	if ly.Shape != nil && ly.Shape.TC != nil && ly.Shape.TC.Text != "" {
		return ly.Shape.TC.Text, true
	}
	for i := range ly.Children {
		if s, ok := textUnder(&ly.Children[i]); ok {
			return s, true
		}
	}
	return "", false
}

func TestTabChipStatusDotAndEllipsis(t *testing.T) {
	const longTitle = "ss-dever@192.168.82.151:6022-very-long-suffix"
	st := &app.State{
		ActiveTab: "t1",
		Tabs: []app.Tab{
			{ID: "t1", Kind: app.TabSession, Title: longTitle, Status: app.TabStatusConnected, SessID: "s1"},
			{ID: "t2", Kind: app.TabSession, Title: "short", Status: app.TabStatusDisconnected},
			{ID: "t3", Kind: app.TabConnect, Status: app.TabStatusIdle},
		},
	}
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 800, Height: 600, State: st})
	root := w.TestRender(func(win *gui.Window) gui.View {
		return TabBar(win, &app.Controller{})
	})

	// Every chip carries a dot of the fixed 8x8 geometry.
	colors := make(map[string]gui.Color, 3)
	for _, id := range []string{"t1", "t2", "t3"} {
		dot, ok := findID(root, "tab-dot-"+id)
		if !ok || dot.Shape == nil {
			t.Fatalf("missing status dot for %s", id)
		}
		if dot.Shape.Width != tabDotSize || dot.Shape.Height != tabDotSize {
			t.Fatalf("dot %s = %.0fx%.0f, want %.0fx%.0f", id,
				dot.Shape.Width, dot.Shape.Height, tabDotSize, tabDotSize)
		}
		colors[id] = dot.Shape.Color
	}
	// Connected, disconnected and idle must not share a colour.
	if colors["t1"] == colors["t2"] || colors["t1"] == colors["t3"] || colors["t2"] == colors["t3"] {
		t.Fatalf("status dots share colours: %+v", colors)
	}

	// The long title is shortened and never exceeds the label budget.
	btn, ok := findID(root, "tab-t1")
	if !ok {
		t.Fatal("missing title button for t1")
	}
	label, ok := textUnder(btn)
	if !ok {
		t.Fatal("missing title text for t1")
	}
	if label == longTitle {
		t.Fatalf("long title was not shortened: %q", label)
	}
	if !strings.HasSuffix(label, "…") {
		t.Fatalf("shortened title %q does not end in an ellipsis", label)
	}
	if got := w.TextWidth(label, gui.DefaultTextStyle); got > tabTitleBudget {
		t.Fatalf("title %q measures %.0f, budget %.0f", label, got, tabTitleBudget)
	}

	// A short title is left alone.
	btn2, ok := findID(root, "tab-t2")
	if !ok {
		t.Fatal("missing title button for t2")
	}
	if label2, _ := textUnder(btn2); label2 != "short" {
		t.Fatalf("short title = %q, want untouched", label2)
	}
}

// The close X must stay a full-height hit target even though the label and
// close button are packed tight, and the label must not run under it.
func TestTabChipCloseHitArea(t *testing.T) {
	st := &app.State{
		ActiveTab: "t1",
		Tabs:      []app.Tab{{ID: "t1", Kind: app.TabSession, Title: "box", Status: app.TabStatusConnected, SessID: "s1"}},
	}
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 800, Height: 600, State: st})
	root := w.TestRender(func(win *gui.Window) gui.View {
		return TabBar(win, &app.Controller{})
	})

	chip, ok := findID(root, "tab-chip-t1")
	if !ok || chip.Shape == nil {
		t.Fatal("missing chip")
	}
	title, ok := findID(root, "tab-t1")
	if !ok || title.Shape == nil {
		t.Fatal("missing title button")
	}
	closeBtn, ok := findID(root, "tab-close-t1")
	if !ok || closeBtn.Shape == nil {
		t.Fatal("missing close button")
	}

	if closeBtn.Shape.Height != title.Shape.Height {
		t.Fatalf("close hit area height = %.1f, want the chip's %.1f",
			closeBtn.Shape.Height, title.Shape.Height)
	}
	if closeBtn.Shape.X < title.Shape.X+title.Shape.Width {
		t.Fatalf("close button overlaps the label: close x=%.1f, label ends at %.1f",
			closeBtn.Shape.X, title.Shape.X+title.Shape.Width)
	}
	if gap := closeBtn.Shape.X - (title.Shape.X + title.Shape.Width); gap != 0 {
		t.Fatalf("label→close gap = %.1f, want 0 (tight)", gap)
	}
}
