package shell_test

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/shell"
)

func TestIconRailButtonGeometry(t *testing.T) {
	w := gui.NewTestWindow(t, gui.WindowCfg{
		Width:  800,
		Height: 600,
		State:  &app.State{},
	})
	root := w.TestRender(func(win *gui.Window) gui.View {
		return shell.Root(win, &app.Controller{})
	})

	rail, ok := findID(root, "icon-rail")
	if !ok || rail.Shape == nil {
		t.Fatal("missing icon-rail")
	}
	if rail.Shape.Width != 30 {
		t.Fatalf("rail width = %v, want 30", rail.Shape.Width)
	}

	for _, id := range []string{"rail-quick-connect", "rail-bookmarks", "rail-settings"} {
		cell, ok := findID(root, id)
		if !ok || cell.Shape == nil {
			t.Fatalf("missing %s", id)
		}
		if cell.Shape.Width != 28 || cell.Shape.Height != 28 {
			t.Fatalf("%s size = %.0fx%.0f, want 28x28", id, cell.Shape.Width, cell.Shape.Height)
		}
		if cell.Shape.Padding.Left != 0 || cell.Shape.Padding.Right != 0 {
			t.Fatalf("%s padding L/R = %.0f/%.0f, want 0", id,
				cell.Shape.Padding.Left, cell.Shape.Padding.Right)
		}
	}
}

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
