package commandui

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/command"
)

func TestCommandTreeNodesSortAndMarkCustom(t *testing.T) {
	all := map[string][]command.Info{
		"zebra": {},
		"alpha": {
			{Name: "zeta", Command: "z"},
			{Name: "beta", Command: "b", IsCustom: true},
		},
	}
	nodes := commandTreeNodes(all)
	if len(nodes) != 2 {
		t.Fatalf("categories = %d, want 2", len(nodes))
	}
	// "alpha" sorts before "zebra ".
	if nodes[0].ID != nodeCatPrefix+"alpha" {
		t.Fatalf("first category = %q", nodes[0].ID)
	}
	kids := nodes[0].Nodes
	if len(kids) != 2 || kids[0].Text != "beta *" || kids[1].Text != "zeta" {
		t.Fatalf("kids = %+v", kids)
	}
	if kids[0].Icon != gui.IconPencil || kids[1].Icon != gui.IconTerminal {
		t.Fatalf("icons = %q %q", kids[0].Icon, kids[1].Icon)
	}
	if kids[0].ID != nodeCmdPrefix+"alpha"+nodeCmdSep+"beta" {
		t.Fatalf("leaf id = %q", kids[0].ID)
	}
}

func TestSelectedIDs(t *testing.T) {
	st := &State{selectedSess: map[string]bool{"b": true, "a": true}}
	ids := st.selectedIDs()
	if len(ids) != 2 {
		t.Fatalf("ids = %v", ids)
	}
	st2 := &State{}
	st2.ensureLoaded()
	if len(st2.selectedIDs()) != 0 {
		t.Fatal("empty selection should yield no ids")
	}
}
