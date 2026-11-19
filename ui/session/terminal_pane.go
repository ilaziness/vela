package session

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
)

// paneMinSize keeps a split pane usable; below it the Splitter refuses
// to shrink further.
const paneMinSize float32 = 120

// TerminalPane embeds the go-term views of a session tab's panes,
// arranged by the tab's layout, or the reconnect view for a dropped
// pane.
func TerminalPane(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	if tab == nil || tab.Kind != app.TabSession {
		return gui.Label(i18n.T("shell.session.none"), gui.CurrentTheme().TextStyleBody)
	}
	tab.EnsurePanes()
	switch tab.Layout {
	case app.LayoutSplitH:
		return panePair(w, ctrl, tab, gui.SplitterHorizontal)
	case app.LayoutSplitV:
		return panePair(w, ctrl, tab, gui.SplitterVertical)
	case app.LayoutGrid2x2:
		return paneGrid(w, ctrl, tab)
	default:
		return paneCell(w, ctrl, tab, 0)
	}
}

// panePair renders a two-slot layout: SplitH is left|right, SplitV is
// top|bottom.
func panePair(w *gui.Window, ctrl *app.Controller, tab *app.Tab, orient gui.SplitterOrientation) gui.View {
	return gui.Splitter(paneSplitCfg(w, ctrl, tab, "pair", orient, 0, 1, 0, tab.RatioMain))
}

// paneGrid renders the 2x2 layout: an outer left|right split at
// RatioMain, each half split top|bottom at RatioSub. Pane order is
// row-major: 0=TL, 1=TR, 2=BL, 3=BR — so the left half holds 0 and 2.
func paneGrid(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	tabID := tab.ID
	left := gui.Splitter(paneSplitCfg(w, ctrl, tab, "gl", gui.SplitterVertical, 0, 2, 1, tab.RatioSub))
	right := gui.Splitter(paneSplitCfg(w, ctrl, tab, "gr", gui.SplitterVertical, 1, 3, 1, tab.RatioSub))
	return gui.Splitter(gui.SplitterCfg{
		ID:          "pane-grid-" + tabID,
		Orientation: gui.SplitterHorizontal,
		Sizing:      gui.FillFill,
		First:       gui.SplitterPaneCfg{Content: []gui.View{left}, MinSize: paneMinSize},
		Second:      gui.SplitterPaneCfg{Content: []gui.View{right}, MinSize: paneMinSize},
		Ratio:       gui.SomeF(tab.RatioMain),
		HandleSize:  gui.SomeF(1),
		ColorHandle: th.ColorBorder,
		ColorGrip:   gui.ColorTransparent,
		OnChange: func(r float32, _ gui.SplitterCollapsed, ctx gui.EventCtx) {
			writePaneRatio(ctx, tabID, 0, r)
		},
	})
}

// paneSplitCfg builds one divider. which 0 writes RatioMain, 1 writes
// RatioSub; the lookup goes through State because the Tab pointer from
// View-build time may dangle after st.Tabs reallocates.
func paneSplitCfg(w *gui.Window, ctrl *app.Controller, tab *app.Tab, tag string, orient gui.SplitterOrientation, first, second, which int, ratio float32) gui.SplitterCfg {
	th := gui.CurrentTheme()
	tabID := tab.ID
	return gui.SplitterCfg{
		ID:          fmt.Sprintf("pane-%s-%s", tag, tabID),
		Orientation: orient,
		Sizing:      gui.FillFill,
		First:       gui.SplitterPaneCfg{Content: []gui.View{paneCell(w, ctrl, tab, first)}, MinSize: paneMinSize},
		Second:      gui.SplitterPaneCfg{Content: []gui.View{paneCell(w, ctrl, tab, second)}, MinSize: paneMinSize},
		Ratio:       gui.SomeF(ratio),
		HandleSize:  gui.SomeF(1),
		ColorHandle: th.ColorBorder,
		ColorGrip:   gui.ColorTransparent,
		OnChange: func(r float32, _ gui.SplitterCollapsed, ctx gui.EventCtx) {
			writePaneRatio(ctx, tabID, which, r)
		},
	}
}

// writePaneRatio stores a dragged divider ratio back into the tab,
// clamped so neither pane collapses.
func writePaneRatio(ctx gui.EventCtx, tabID string, which int, r float32) {
	if r < 0.05 {
		r = 0.05
	}
	if r > 0.95 {
		r = 0.95
	}
	t := gui.State[app.State](ctx.Window).TabByID(tabID)
	if t == nil {
		return
	}
	if which == 0 {
		t.RatioMain = r
	} else {
		t.RatioSub = r
	}
}

// paneCellID is the stable per-slot element id shared by the context
// menu wrapper and the drag-drop rect registry.
func paneCellID(tabID string, idx int) string {
	return fmt.Sprintf("pane-cell-%s-%d", tabID, idx)
}
