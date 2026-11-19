package session

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// tabDragHandle is the grip at a session chip's left edge: a visible
// 2x3 dot cluster that starts a drag of the tab's active pane. Only
// tabs whose active pane holds a live session are draggable; clicking
// the rest of the chip still just switches tabs.
func tabDragHandle(w *gui.Window, th gui.Theme, tab app.Tab, ctrl *app.Controller) gui.View {
	tab.EnsurePanes()
	dragIdx := -1
	if tab.Kind == app.TabSession && tab.ActivePane >= 0 && tab.ActivePane < len(tab.Panes) &&
		tab.Panes[tab.ActivePane].SessID != "" {
		dragIdx = tab.ActivePane
	}
	if dragIdx < 0 {
		return gui.Rectangle(gui.RectangleCfg{Sizing: gui.FixedFixed, Width: 0, Height: 0})
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FixedFill,
		Width:  14,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		// CursorResizeAll, not CursorGrab: the win32 backend maps Grab
		// to the plain arrow (no stock open-hand cursor), while
		// SizeAll is the visible "movable" pointer.
		Hover: gui.HoverStyle{Cursor: gui.CursorResizeAll, Color: th.ColorHover},
		OnMouseDown: func(ctx gui.EventCtx) {
			ctx.Consume()
			st := gui.State[app.State](ctx.Window)
			st.DragPane = app.DragPane{SrcTabID: tab.ID, SrcPane: dragIdx}
			ctx.Window.MouseLock(gui.MouseLockCfg{
				MouseMove: func(mv gui.EventCtx) {
					st := gui.State[app.State](mv.Window)
					if st.DragPane.SrcTabID == "" {
						return
					}
					dstTab, dstPane, ok := hitEmptyPane(st, mv.Event.MouseX, mv.Event.MouseY)
					if !ok {
						st.DragPane.HoverTabID, st.DragPane.HoverPane = "", -1
						return
					}
					st.DragPane.HoverTabID, st.DragPane.HoverPane = dstTab, dstPane
				},
				MouseUp: func(up gui.EventCtx) {
					st := gui.State[app.State](up.Window)
					drag := st.DragPane
					st.DragPane = app.DragPane{}
					if drag.SrcTabID == "" {
						return
					}
					if dstTab, dstPane, ok := hitEmptyPane(st, up.Event.MouseX, up.Event.MouseY); ok {
						ctrl.MovePaneTo(st, drag.SrcTabID, drag.SrcPane, dstTab, dstPane)
					}
				},
				Cancel: func(win *gui.Window) {
					gui.State[app.State](win).DragPane = app.DragPane{}
				},
			})
		},
		Content: []gui.View{dragDots(th)},
	}))
}

// dragDots is the 2x3 dot grip marking a chip as draggable.
func dragDots(th gui.Theme) gui.View {
	dot := func() gui.View {
		return gui.Rectangle(gui.RectangleCfg{
			Sizing: gui.FixedFixed,
			Width:  2,
			Height: 2,
			Color:  th.TextStyleSecondary.Color,
		})
	}
	row := func() gui.View {
		return gui.Row(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FitFit,
			Spacing: gui.SpacingPx(2),
			Content: []gui.View{dot(), dot()},
		}))
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		Spacing: gui.SpacingPx(2),
		Content: []gui.View{row(), row(), row()},
	}))
}

// hitEmptyPane resolves a window point to an empty pane slot. Only
// strictly idle slots accept drops — a live or reconnecting pane keeps
// its session.
func hitEmptyPane(st *app.State, x, y float32) (tabID string, idx int, ok bool) {
	for key, r := range st.PaneRects {
		if x < r.X || x > r.X+r.W || y < r.Y || y > r.Y+r.H {
			continue
		}
		hash := strings.LastIndexByte(key, '#')
		if hash < 0 {
			continue
		}
		i, err := strconv.Atoi(key[hash+1:])
		if err != nil {
			continue
		}
		// PaneRects only refreshes for the rendered (active) tab, but
		// stale entries of background tabs linger — a drop must never
		// land in a tab the user cannot see.
		if key[:hash] != st.ActiveTab {
			continue
		}
		tab := st.TabByID(key[:hash])
		if tab == nil || tab.Kind != app.TabSession {
			continue
		}
		tab.EnsurePanes()
		if i < 0 || i >= len(tab.Panes) {
			continue
		}
		p := tab.Panes[i]
		if p.SessID == "" && p.Status == app.TabStatusIdle {
			return key[:hash], i, true
		}
	}
	return "", 0, false
}
