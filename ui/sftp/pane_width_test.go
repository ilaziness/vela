package sftpui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/ui/app"
)

func findLayoutByID(ly *gui.Layout, id string) *gui.Layout {
	if ly == nil || ly.Shape == nil {
		return nil
	}
	if ly.Shape.ID == id {
		return ly
	}
	for i := range ly.Children {
		if found := findLayoutByID(&ly.Children[i], id); found != nil {
			return found
		}
	}
	return nil
}

func paneWidths(t *testing.T, winW, winH int, remoteNames, localNames []string) (float32, float32) {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: winW, Height: winH, State: &app.State{}})
	s := &app.SFTPState{
		RemoteDir:      "/srv",
		LocalDir:       "/home/u",
		RemoteSelected: map[string]bool{},
		LocalSelected:  map[string]bool{},
	}
	for i, n := range remoteNames {
		s.RemoteEntries = append(s.RemoteEntries, sftp.FileInfo{
			Name: n, Mode: "-rw-r--r--", Size: int64(100 + i), ModTime: "2024-01-02T03:04:05Z",
		})
	}
	for i, n := range localNames {
		s.LocalEntries = append(s.LocalEntries, localfs.Entry{
			Name: n, Path: "/home/u/" + n, Size: int64(100 + i), ModTime: "2024-01-02T03:04:05Z",
		})
	}
	tab := &app.Tab{ID: "tab1", SessID: "sess1"}
	ctrl := &app.Controller{}
	root := w.TestRender(func(win *gui.Window) gui.View {
		return panesRow(win, ctrl, tab, s)
	})
	local := findLayoutByID(root, "sftp-pane-local")
	remote := findLayoutByID(root, "sftp-pane-remote")
	if local == nil || remote == nil || local.Shape == nil || remote.Shape == nil {
		t.Fatal("missing pane layouts")
	}
	return local.Shape.Width, remote.Shape.Width
}

func TestPaneWidthsEqualAndStable(t *testing.T) {
	short := []string{"a", "b", "c"}
	long := make([]string, 3)
	for i := range long {
		long[i] = "a-very-long-file-name-that-keeps-going-" + strings.Repeat("x", 40+i*20)
	}
	many := make([]string, 60)
	for i := range many {
		many[i] = fmt.Sprintf("file-%03d-%s", i, strings.Repeat("y", i%37))
	}
	for _, win := range [][2]int{{1400, 800}, {900, 700}, {700, 600}} {
		lw1, rw1 := paneWidths(t, win[0], win[1], short, short)
		lw2, rw2 := paneWidths(t, win[0], win[1], long, long)
		lw3, rw3 := paneWidths(t, win[0], win[1], long, short)
		lw4, rw4 := paneWidths(t, win[0], win[1], many, short)
		t.Logf("win=%v short/short: local=%.1f remote=%.1f", win, lw1, rw1)
		t.Logf("win=%v long/long:   local=%.1f remote=%.1f", win, lw2, rw2)
		t.Logf("win=%v long/short:  local=%.1f remote=%.1f", win, lw3, rw3)
		t.Logf("win=%v many/short:  local=%.1f remote=%.1f", win, lw4, rw4)
		for name, pair := range map[string][2]float32{
			"short/short": {lw1, rw1},
			"long/long":   {lw2, rw2},
			"long/short":  {lw3, rw3},
			"many/short":  {lw4, rw4},
		} {
			if d := pair[0] - pair[1]; d > 1 || d < -1 {
				t.Errorf("win=%v %s: pane widths differ by %.1f (local=%.1f remote=%.1f)", win, name, d, pair[0], pair[1])
			}
		}
		if lw1 != lw2 || rw1 != rw2 || lw1 != lw4 || rw1 != rw4 {
			t.Errorf("win=%v pane widths changed with entries: (%.1f,%.1f) (%.1f,%.1f) (%.1f,%.1f)",
				win, lw1, rw1, lw2, rw2, lw4, rw4)
		}
	}
}
