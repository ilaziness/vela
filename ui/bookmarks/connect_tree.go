package bookmarks

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

// ConnectTree renders a searchable bookmark tree for quick-connect.
// onActivate receives the bookmark id (without "b:" prefix) on leaf select.
func ConnectTree(groups []*bookmark.Group, query string, onActivate func(id string)) gui.View {
	filtered := filterGroups(groups, query)
	nodes := groupTreeNodes(filtered)
	if len(nodes) == 0 {
		th := gui.CurrentTheme()
		msg := i18n.T("bookmark.no_match")
		if strings.TrimSpace(query) == "" {
			msg = i18n.T("bookmark.connect_empty")
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			Sizing: gui.FillFill,
			Content: []gui.View{
				gui.Label(msg, th.TextStyleSecondary),
			},
		}))
	}
	return gui.Tree(gui.TreeCfg{
		ID:     "qc-tree",
		Sizing: gui.FillFill,
		Indent: 12,
		Nodes:  nodes,
		OnSelect: func(id string, ctx gui.EventCtx) {
			ctx.Consume()
			kind, key := parseNode(id)
			if kind != selBookmark || key == "" {
				return
			}
			onActivate(key)
		},
	})
}
