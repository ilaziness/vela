package session

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-term/term"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// paneCell renders one slot of a session tab: the terminal, a
// reconnect prompt, or an empty dial target. The frame registers its
// absolute rect every frame so a tab-chip drag can hit-test it, and
// marks the active pane with an accent border when the tab is split.
func paneCell(w *gui.Window, ctrl *app.Controller, tab *app.Tab, idx int) gui.View {
	th := gui.CurrentTheme()
	tab.EnsurePanes()
	if idx < 0 || idx >= len(tab.Panes) {
		return gui.Label(i18n.T("shell.session.none"), th.TextStyleBody)
	}
	p := tab.Panes[idx]
	tabID := tab.ID // captured: *Tab dangles after st.Tabs reallocates

	var body gui.View
	switch {
	case p.SessID != "":
		tm := ctrl.TermFor(p.SessID)
		if tm == nil {
			// SessID without a term is a broken invariant, not a disconnect.
			body = gui.Label(i18n.T("shell.session.term_not_ready"), th.TextStyleBody)
			break
		}
		body = gui.Column(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Content: []gui.View{tm.View(w)},
		}))
	case p.Status == app.TabStatusDisconnected:
		body = paneReconnectView(w, ctrl, tab, idx)
	case p.Status == app.TabStatusConnecting:
		body = centeredLabel(th, i18n.T("shell.session.connecting"))
	default:
		body = emptyPaneView(w, ctrl, tabID, idx)
	}

	cellID := paneCellID(tabID, idx)
	// Every cell keeps a 1px border so switching the active pane never
	// shifts layout; only the active pane (in a split) or the current
	// drag-drop target paints it, in the accent color.
	st := gui.State[app.State](w)
	dragHover := st.DragPane.SrcTabID != "" &&
		st.DragPane.HoverTabID == tabID && st.DragPane.HoverPane == idx
	borderColor := gui.ColorTransparent
	if (len(tab.Panes) > 1 && idx == tab.ActivePane) || dragHover {
		borderColor = th.ColorAccent
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:          cellID,
		Sizing:      gui.FillFill,
		Color:       th.ColorBackground,
		ColorBorder: borderColor,
		SizeBorder:  gui.BorderPx(1),
		AmendLayout: func(ctx gui.EventCtx) {
			if ctx.Layout == nil || ctx.Layout.Shape == nil {
				return
			}
			sh := ctx.Layout.Shape
			st := gui.State[app.State](ctx.Window)
			if st.PaneRects == nil {
				st.PaneRects = make(map[string]app.PaneRect)
			}
			st.PaneRects[fmt.Sprintf("%s#%d", tabID, idx)] = app.PaneRect{
				X: sh.X, Y: sh.Y, W: sh.Width, H: sh.Height,
			}
		},
		Content: []gui.View{paneMenu(w, ctrl, tab, idx, p.SessID, body)},
	}))
}

// centeredLabel fills its parent with one centered line of text.
func centeredLabel(th gui.Theme, text string) gui.View {
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{gui.Label(text, th.TextStyleSecondary)},
	}))
}

// emptyPaneView is the dial target shown in an idle slot: a hint plus
// a click that makes it the active pane.
func emptyPaneView(w *gui.Window, ctrl *app.Controller, tabID string, idx int) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.SetActivePane(ctx.Window, tabID, idx)
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      i18n.T("pane.empty_title"),
				TextStyle: th.TextStyleBody,
			}),
			gui.Text(gui.TextCfg{
				Text:      i18n.T("pane.empty_hint"),
				TextStyle: th.TextStyleSecondary,
			}),
		},
	}))
}

// paneMenu wraps a pane cell in a right-click context menu. Live
// terminals get copy/paste/find/record; every cell gets the pane
// layout actions.
func paneMenu(w *gui.Window, ctrl *app.Controller, tab *app.Tab, idx int, sessID string, body gui.View) gui.View {
	items := paneLayoutMenuItems(ctrl, tab, idx)
	if sessID != "" {
		if tm := ctrl.TermFor(sessID); tm != nil {
			recordText := i18n.T("terminal.start_logging")
			if tm.Recording() {
				recordText = i18n.T("terminal.stop_logging")
			}
			items = append([]gui.MenuItemCfg{
				{ID: "copy", Text: i18n.T("common.copy"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
					tm.RunAction(term.ActionCopy, ctx.Window)
				}},
				{ID: "paste", Text: i18n.T("terminal.paste"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
					tm.RunAction(term.ActionPaste, ctx.Window)
				}},
				{ID: "find", Text: i18n.T("terminal.find"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
					tm.RunAction(term.ActionFind, ctx.Window)
				}},
				gui.MenuSeparator(),
				{ID: "record", Text: recordText, Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
					ctrl.ToggleSessionRecording(ctx.Window, sessID)
				}},
				gui.MenuSeparator(),
			}, items...)
		}
	}
	return gui.ContextMenu(w, gui.ContextMenuCfg{
		ID:      "term-ctx-" + paneCellID(tab.ID, idx),
		Sizing:  gui.FillFill,
		Padding: gui.PaddingNone,
		Items:   items,
		Content: []gui.View{body},
	})
}

// paneLayoutMenuItems is the split/layout group shared by every pane
// cell's context menu. Items that would be no-ops in the current
// state are omitted: the active layout's own split entry, even/single
// on a single-pane tab, and close on an already-empty slot.
func paneLayoutMenuItems(ctrl *app.Controller, tab *app.Tab, idx int) []gui.MenuItemCfg {
	tabID := tab.ID
	tab.EnsurePanes()
	single := tab.Layout == app.LayoutSingle
	paneEmpty := idx >= len(tab.Panes) ||
		(tab.Panes[idx].SessID == "" && tab.Panes[idx].Status == app.TabStatusIdle)

	item := func(id, key string, lay app.PaneLayout) gui.MenuItemCfg {
		return gui.MenuItemCfg{ID: id, Text: i18n.T(key), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.SetPaneLayout(ctx.Window, tabID, lay)
		}}
	}
	var splits []gui.MenuItemCfg
	if tab.Layout != app.LayoutSplitH {
		splits = append(splits, item("split-h", "pane.split_h", app.LayoutSplitH))
	}
	if tab.Layout != app.LayoutSplitV {
		splits = append(splits, item("split-v", "pane.split_v", app.LayoutSplitV))
	}
	if tab.Layout != app.LayoutGrid2x2 {
		splits = append(splits, item("grid", "pane.grid", app.LayoutGrid2x2))
	}

	var mid []gui.MenuItemCfg
	if !single {
		mid = append(mid, gui.MenuItemCfg{ID: "even", Text: i18n.T("pane.even"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.EvenPaneRatios(ctx.Window, tabID)
		}})
	}
	if !paneEmpty && !single {
		mid = append(mid, gui.MenuItemCfg{ID: "close-pane", Text: i18n.T("pane.close"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.ClosePane(ctx.Window, tabID, idx)
		}})
	}

	var last []gui.MenuItemCfg
	if !single {
		last = append(last, gui.MenuItemCfg{ID: "single", Text: i18n.T("pane.single"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.CollapseToSingle(ctx.Window, tabID, idx)
		}})
	}

	var items []gui.MenuItemCfg
	for _, g := range [][]gui.MenuItemCfg{splits, mid, last} {
		if len(g) == 0 {
			continue
		}
		if len(items) > 0 {
			items = append(items, gui.MenuSeparator())
		}
		items = append(items, g...)
	}
	return items
}
