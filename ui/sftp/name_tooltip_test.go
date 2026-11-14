package sftpui

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/ui/app"
)

func shapeHasText(ly *gui.Layout, text string) bool {
	if ly == nil || ly.Shape == nil {
		return false
	}
	if ly.Shape.TC != nil && ly.Shape.TC.Text == text {
		return true
	}
	for i := range ly.Children {
		if shapeHasText(&ly.Children[i], text) {
			return true
		}
	}
	return false
}

// TestNameTooltip verifies hovering a file name floats the full name,
// on both panes, whether or not the name was ellipsized.
func TestNameTooltip(t *testing.T) {
	short := "short.txt"
	long := "a-very-long-file-name-" + strings.Repeat("x", 60)
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 1400, Height: 800, State: &app.State{}})
	s := &app.SFTPState{
		RemoteDir:      "/srv",
		LocalDir:       "/home/u",
		RemoteSelected: map[string]bool{},
		LocalSelected:  map[string]bool{},
		RemoteEntries: []sftp.FileInfo{
			{Name: long, Mode: "-rw-r--r--", Size: 1, ModTime: "2024-01-02T03:04:05Z"},
			{Name: short, Mode: "-rw-r--r--", Size: 1, ModTime: "2024-01-02T03:04:05Z"},
		},
		LocalEntries: []localfs.Entry{
			{Name: long, Path: "/home/u/" + long, Size: 1, ModTime: "2024-01-02T03:04:05Z"},
			{Name: short, Path: "/home/u/" + short, Size: 1, ModTime: "2024-01-02T03:04:05Z"},
		},
	}
	tab := &app.Tab{ID: "tab1", SessID: "sess1"}
	ctrl := &app.Controller{}
	var root *gui.Layout
	root = w.TestRender(func(win *gui.Window) gui.View {
		return panesRow(win, ctrl, tab, s)
	})
	for _, leaf := range []string{"sftp-r-0-name", "sftp-l-0-name", "sftp-r-1-name", "sftp-l-1-name"} {
		cell := findLayoutByID(root, leaf)
		if cell == nil || cell.Shape == nil {
			t.Fatalf("name cell %s missing", leaf)
		}
		w.EventFn(&gui.Event{Type: gui.EventMouseMove,
			MouseX: cell.Shape.X + cell.Shape.Width/2,
			MouseY: cell.Shape.Y + cell.Shape.Height/2})
		root = w.TestRender(nil)
		full := long
		if leaf == "sftp-r-1-name" || leaf == "sftp-l-1-name" {
			full = short
		}
		if !shapeHasText(root, full) {
			t.Errorf("%s: full-name tooltip missing", leaf)
		}
		w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: 5, MouseY: 5})
		root = w.TestRender(nil)
	}
}
