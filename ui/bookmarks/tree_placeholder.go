package bookmarks

import "github.com/go-gui-org/go-gui/gui"

// TreePlaceholder renders a static bookmark tree matching the layout mock.
func TreePlaceholder() gui.View {
	return gui.Tree(gui.TreeCfg{
		ID:     "bookmark-tree",
		Sizing: gui.FillFill,
		Nodes: []gui.TreeNodeCfg{
			{ID: "folder-ssc", Text: "ssc", Icon: "📁", Nodes: nil},
			{ID: "folder-yika", Text: "易卡", Icon: "📁"},
			{ID: "folder-hope", Text: "希望城", Icon: "📁"},
			{
				ID:   "folder-default",
				Text: "默认",
				Icon: "📁",
				Nodes: []gui.TreeNodeCfg{
					{ID: "bm-local", Text: "本地", Icon: "🔖"},
				},
			},
		},
	})
}
