package bookmarks

import "github.com/go-gui-org/go-gui/gui"

// TreePlaceholder renders a static bookmark tree matching the layout mock.
func TreePlaceholder() gui.View {
	return gui.Tree(gui.TreeCfg{
		ID:     "bookmark-tree",
		Sizing: gui.FillFill,
		Nodes: []gui.TreeNodeCfg{
			{ID: "folder-ssc", Text: "ssc", Icon: gui.IconFolder, Nodes: nil},
			{ID: "folder-yika", Text: "易卡", Icon: gui.IconFolder},
			{ID: "folder-hope", Text: "希望城", Icon: gui.IconFolder},
			{
				ID:   "folder-default",
				Text: "默认",
				Icon: gui.IconFolder,
				Nodes: []gui.TreeNodeCfg{
					{ID: "bm-local", Text: "本地", Icon: gui.IconBookmark},
				},
			},
		},
	})
}
