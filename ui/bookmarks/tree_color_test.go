package bookmarks

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
)

func TestGroupTreeNodesBookmarkColor(t *testing.T) {
	groups := []*bookmark.Group{{
		Name: "lab",
		Bookmarks: []bookmark.Bookmark{
			{ID: "1", Title: "named", Color: "red"},
			{ID: "2", Title: "hex", Color: "#336699"},
			{ID: "3", Title: "none", Color: ""},
			{ID: "4", Title: "bad", Color: "magenta"},
		},
	}}
	nodes := groupTreeNodes(groups)
	if len(nodes) != 1 || len(nodes[0].Nodes) != 4 {
		t.Fatalf("nodes = %d groups / %d books", len(nodes), len(nodes[0].Nodes))
	}
	if nodes[0].TextStyle != (gui.TextStyle{}) {
		t.Fatal("group row should keep the theme default style")
	}

	def := gui.CurrentTheme().TextStyleDef
	wantNamed, ok := parseBookmarkColor("red")
	if !ok {
		t.Fatal("named red should parse")
	}
	wantHex, ok := parseBookmarkColor("#336699")
	if !ok {
		t.Fatal("hex should parse")
	}

	named := nodes[0].Nodes[0]
	if named.TextStyle.Color != wantNamed {
		t.Fatalf("named color = %s, want %s", named.TextStyle.Color.Hex(), wantNamed.Hex())
	}
	if named.TextStyle.Size != def.Size || named.TextStyle.Family != def.Family {
		t.Fatalf("named style drifted from tree default: size %.1f family %q", named.TextStyle.Size, named.TextStyle.Family)
	}
	if named.TextStyleIcon != (gui.TextStyle{}) {
		t.Fatal("bookmark icon should keep the theme default")
	}

	hex := nodes[0].Nodes[1]
	if hex.TextStyle.Color != wantHex {
		t.Fatalf("hex color = %s, want %s", hex.TextStyle.Color.Hex(), wantHex.Hex())
	}
	for _, n := range nodes[0].Nodes[2:] {
		if n.TextStyle != (gui.TextStyle{}) {
			t.Fatalf("%s should keep the theme default, got color %s", n.Text, n.TextStyle.Color.Hex())
		}
	}
}

func TestBookmarkTreeTextUsesColor(t *testing.T) {
	groups := []*bookmark.Group{{
		Name: "lab",
		Bookmarks: []bookmark.Bookmark{
			{ID: "1", Title: "Red Host", User: "root", Host: "10.0.0.1", Color: "red"},
			{ID: "2", Title: "Plain Host", User: "root", Host: "10.0.0.2"},
			{ID: "3", Title: "Hex Host", User: "root", Host: "10.0.0.3", Color: "#336699"},
		},
	}}
	redText := treeBookmarkText(groups[0].Bookmarks[0])
	plainText := treeBookmarkText(groups[0].Bookmarks[1])
	hexText := treeBookmarkText(groups[0].Bookmarks[2])
	wantRed, _ := parseBookmarkColor("red")
	wantHex, _ := parseBookmarkColor("#336699")
	defColor := gui.CurrentTheme().TextStyleDef.Color

	check := func(t *testing.T, root *gui.Layout) {
		t.Helper()
		red := findText(root, redText)
		plain := findText(root, plainText)
		hex := findText(root, hexText)
		if red == nil || plain == nil || hex == nil {
			t.Fatalf("missing row text red=%v plain=%v hex=%v", red != nil, plain != nil, hex != nil)
		}
		assertTextColor(t, red, wantRed)
		assertTextColor(t, plain, defColor)
		assertTextColor(t, hex, wantHex)
		if red.Shape.Height != plain.Shape.Height || hex.Shape.Height != plain.Shape.Height {
			t.Fatalf("row text height drifted: red %.1f plain %.1f hex %.1f",
				red.Shape.Height, plain.Shape.Height, hex.Shape.Height)
		}
	}

	t.Run("quick-connect", func(t *testing.T) {
		w := gui.NewTestWindow(t, gui.WindowCfg{Width: 320, Height: 280})
		root := w.TestRender(func(win *gui.Window) gui.View {
			return ConnectTree(groups, "", func(string) {})
		})
		expandGroup(t, w, root, "lab")
		root = w.TestRender(nil)
		check(t, root)
	})

	t.Run("manager", func(t *testing.T) {
		w := gui.NewTestWindow(t, gui.WindowCfg{
			Width: 320, Height: 280,
			State: &State{Groups: groups},
		})
		root := w.TestRender(func(win *gui.Window) gui.View {
			return bookmarkTree(win, nil)
		})
		expandGroup(t, w, root, "lab")
		root = w.TestRender(nil)
		check(t, root)
	})
}

func expandGroup(t *testing.T, w *gui.Window, root *gui.Layout, name string) {
	t.Helper()
	ly := findText(root, name)
	if ly == nil || ly.Shape == nil {
		t.Fatalf("group %q not in tree", name)
	}
	w.EventFn(&gui.Event{
		Type: gui.EventMouseDown, MouseButton: gui.MouseLeft,
		MouseX: ly.Shape.X + ly.Shape.Width/2,
		MouseY: ly.Shape.Y + ly.Shape.Height/2,
	})
}

func findText(ly *gui.Layout, text string) *gui.Layout {
	if ly == nil {
		return nil
	}
	if ly.Shape != nil && ly.Shape.TC != nil && ly.Shape.TC.Text == text {
		return ly
	}
	for i := range ly.Children {
		if hit := findText(&ly.Children[i], text); hit != nil {
			return hit
		}
	}
	return nil
}

func assertTextColor(t *testing.T, ly *gui.Layout, want gui.Color) {
	t.Helper()
	if ly.Shape == nil || ly.Shape.TC == nil || ly.Shape.TC.TextStyle == nil {
		t.Fatal("text style missing")
	}
	got := ly.Shape.TC.TextStyle.Color
	if got.R != want.R || got.G != want.G || got.B != want.B || got.A != want.A {
		t.Fatalf("text %q color = %s, want %s", ly.Shape.TC.Text, got.Hex(), want.Hex())
	}
}
