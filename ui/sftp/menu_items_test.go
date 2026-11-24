package sftpui

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/ui/app"
)

func TestRemoteItemMenuHasNewItems(t *testing.T) {
	s := &app.SFTPState{RemoteDir: "/tmp"}
	tab := &app.Tab{SessID: "s1"}
	cfg := remoteItemMenu(nil, tab, s, sftp.FileInfo{Name: "f.txt"})
	want := map[string]bool{"copy": true, "cut": true, "compress": true, "decompress": true}
	found := map[string]bool{}
	for _, it := range cfg.Items {
		if want[it.ID] {
			found[it.ID] = true
		}
		if it.ID == "paste" {
			t.Error("paste should be hidden with empty clipboard")
		}
	}
	for id := range want {
		if !found[id] {
			t.Errorf("missing menu item %q", id)
		}
	}
}

func TestRemoteMenusSharePaneItems(t *testing.T) {
	s := &app.SFTPState{RemoteDir: "/tmp"}
	tab := &app.Tab{SessID: "s1"}
	shared := []string{"deselect", "refresh", "newfile", "newfolder", "uploadfile", "uploadfolder"}
	menus := map[string]map[string]bool{
		"item":  menuIDs(remoteItemMenu(nil, tab, s, sftp.FileInfo{Name: "f.txt"}).Items),
		"blank": menuIDs(remoteBlankMenu(nil, tab, s).Items),
	}
	for name, ids := range menus {
		for _, want := range shared {
			if !ids[want] {
				t.Errorf("%s menu missing shared item %q", name, want)
			}
		}
	}
}

func menuIDs(items []gui.MenuItemCfg) map[string]bool {
	ids := make(map[string]bool, len(items))
	for _, it := range items {
		ids[it.ID] = true
	}
	return ids
}
