package bookmarks

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

const sidebarWidth float32 = 280

func sidebar(w *gui.Window, d *Deps) gui.View {
	st := gui.State[State](w)
	th := gui.CurrentTheme()

	ioRow := gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID: "bm-import", Variant: gui.ButtonSecondary, Sizing: gui.FillFit,
				Label:    i18n.T("bookmark.import_config"),
				Disabled: st.Busy,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					promptImport(ctx.Window, d)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID: "bm-export", Variant: gui.ButtonSecondary, Sizing: gui.FillFit,
				Label:    i18n.T("bookmark.export_config"),
				Disabled: st.Busy,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					promptExport(ctx.Window, d)
				},
			}),
		},
	}))

	searchRow := gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID: "bm-refresh", Variant: gui.ButtonSecondary, Sizing: gui.FitFit,
				Disabled:  st.Busy,
				A11YLabel: i18n.T("common.refresh"),
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      gui.IconSync,
						TextStyle: th.TextStyleIconSmall,
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					if gui.State[State](ctx.Window).Busy {
						return
					}
					reload(ctx.Window, d)
				},
			}),
			gui.Input(gui.InputCfg{
				ID:          "bm-search",
				Placeholder: i18n.T("bookmark.search_placeholder"),
				Text:        st.Search,
				Sizing:      gui.FillFit,
				OnTextChanged: func(v string, ctx gui.EventCtx) {
					gui.State[State](ctx.Window).Search = v
					ctx.Consume()
				},
			}),
		},
	}))

	content := []gui.View{
		ioRow,
		searchRow,
		bookmarkTree(w, d),
		sidebarBottomActions(st, d),
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "bm-sidebar",
		Sizing:  gui.FixedFill,
		Width:   sidebarWidth,
		Color:   th.ColorPanel,
		Padding: gui.NewPadding(10, 12, 10, 12),
		Spacing: gui.SpacingSmall,
		Content: content,
	}))
}

func sidebarBottomActions(st *State, d *Deps) gui.View {
	rows := make([]gui.View, 0, 2)
	if sel := selectionActionButtons(st, d); len(sel) > 0 {
		rows = append(rows, sidebarActionRow(sel...))
	}
	rows = append(rows, sidebarActionRow(
		sideBtn("bm-add-bookmark", gui.IconPlus, i18n.T("bookmark.add"), st.Busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			addBookmarkDraft(ctx.Window)
		}),
		sideBtn("bm-add-group", gui.IconPlus, i18n.T("bookmark.add_group"), st.Busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			promptAddGroup(ctx.Window, d)
		}),
	))
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: rows,
	}))
}

// sidebarActionRow lays out up to two equal-width buttons so stacked rows share columns.
func sidebarActionRow(btns ...gui.View) gui.View {
	cells := make([]gui.View, 0, 2)
	cells = append(cells, btns...)
	for len(cells) < 2 {
		cells = append(cells, gui.Column(layout.Flush(gui.ContainerCfg{Sizing: gui.FillFit})))
	}
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		Content: cells[:2],
	}))
}

func sideBtn(id, glyph, label string, busy bool, onClick func(gui.EventCtx)) gui.View {
	th := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		ID: id, Variant: gui.ButtonGhost, Sizing: gui.FillFit,
		Disabled: busy,
		HAlign:   gui.Some(gui.HAlignStart),
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				Spacing: gui.SpacingTight,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: glyph, TextStyle: th.TextStyleIconSmall}),
					gui.Text(gui.TextCfg{Text: label, TextStyle: th.TextStyleBody}),
				},
			})),
		},
		OnClick: onClick,
	})
}

func selectionActionButtons(st *State, d *Deps) []gui.View {
	busy := st.Busy
	// Unsaved draft: only allow discard — don't expose group rename/delete.
	if st.Editing && st.Draft.ID == "" {
		return []gui.View{sideBtn("bm-del-bm", gui.IconTrash, i18n.T("bookmark.delete_bookmark"), busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			confirmDeleteBookmark(ctx.Window, d)
		})}
	}
	switch st.SelKind {
	case selBookmark:
		return []gui.View{
			sideBtn("bm-copy", gui.IconLayer, i18n.T("bookmark.copy_bookmark"), busy, func(ctx gui.EventCtx) {
				ctx.Consume()
				doCopyBookmark(ctx.Window, d)
			}),
			sideBtn("bm-del-bm", gui.IconTrash, i18n.T("bookmark.delete_bookmark"), busy, func(ctx gui.EventCtx) {
				ctx.Consume()
				confirmDeleteBookmark(ctx.Window, d)
			}),
		}
	case selGroup:
		if isDefaultGroup(st.SelID) {
			return nil
		}
		return []gui.View{
			sideBtn("bm-rename-group", gui.IconPencil, i18n.T("bookmark.rename_group"), busy, func(ctx gui.EventCtx) {
				ctx.Consume()
				promptRenameGroup(ctx.Window, d)
			}),
			sideBtn("bm-del-group", gui.IconTrash, i18n.T("bookmark.delete_group"), busy, func(ctx gui.EventCtx) {
				ctx.Consume()
				confirmDeleteGroup(ctx.Window, d)
			}),
		}
	}
	return nil
}

func bookmarkTree(w *gui.Window, d *Deps) gui.View {
	st := gui.State[State](w)
	groups := filterGroups(st.Groups, st.Search)
	nodes := make([]gui.TreeNodeCfg, 0, len(groups))
	for _, g := range groups {
		if g == nil {
			continue
		}
		kids := make([]gui.TreeNodeCfg, 0, len(g.Bookmarks))
		for _, b := range g.Bookmarks {
			kids = append(kids, gui.TreeNodeCfg{
				ID:   nodeBook(b.ID),
				Text: treeBookmarkText(b),
				Icon: iconGlyph(b.Icon),
			})
		}
		nodes = append(nodes, gui.TreeNodeCfg{
			ID:    nodeGroup(g.Name),
			Text:  displayGroupName(g.Name),
			Icon:  gui.IconFolder,
			Nodes: kids,
		})
	}
	if len(nodes) == 0 {
		th := gui.CurrentTheme()
		msg := i18n.T("bookmark.no_match")
		if strings.TrimSpace(st.Search) == "" {
			msg = i18n.T("bookmark.select_to_edit_hint")
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			Sizing: gui.FillFill,
			Content: []gui.View{
				gui.Label(msg, th.TextStyleSecondary),
			},
		}))
	}
	return gui.Tree(gui.TreeCfg{
		ID:     "bm-tree",
		Sizing: gui.FillFill,
		Indent: 12, // theme default 25 leaves a wide empty strip when expanded
		Nodes:  nodes,
		OnSelect: func(id string, ctx gui.EventCtx) {
			ctx.Consume()
			selectTreeNode(ctx.Window, d, id)
		},
	})
}
