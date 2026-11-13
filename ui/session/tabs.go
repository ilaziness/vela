package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	// tabTitleMaxWidth caps one tab label. Session titles are
	// user@host:port and easily run long, so the chip is kept narrow and
	// the full title is available in the hover tooltip.
	tabTitleMaxWidth float32 = 120
	// tabTitlePadL/R mirror the title button padding below; the ellipsis
	// budget must subtract them because MaxWidth is a border-box width.
	tabTitlePadL float32 = 6
	tabTitlePadR float32 = 2
	// The visible label→close gap is tabTitlePadR + tabClosePadL. Only
	// those two are tight; the close button gets its full-chip height from
	// Sizing: FitFill, so no vertical padding is needed.
	tabClosePadL float32 = 2
	tabClosePadR float32 = 6

	tabDotSize   float32 = 8
	tabDotRadius float32 = tabDotSize / 2
)

// tabTitleBudget is the text width available inside one tab label.
const tabTitleBudget = tabTitleMaxWidth - tabTitlePadL - tabTitlePadR

// TabBar is flush to the top of the main pane.
// Tips use gui.Tooltip on hover — not WithTooltip (wrapper forces theme padding).
func TabBar(w *gui.Window, ctrl *app.Controller) gui.View {
	st := gui.State[app.State](w)
	th := gui.CurrentTheme()
	items := make([]gui.View, 0, len(st.Tabs)+1)
	for _, t := range st.Tabs {
		items = append(items, tabChip(w, th, t, t.ID == st.ActiveTab, ctrl))
	}
	items = append(items, tabAddBtn(th, ctrl))

	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "session-tabs",
		Sizing:  gui.FillFit,
		Color:   th.ColorPanel,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.NoSpacing,
		Content: items,
	}))
}

// tabChip is one dot+title+close unit sharing a single fill so they read
// as one tab, not adjacent ghost buttons.
func tabChip(w *gui.Window, th gui.Theme, tab app.Tab, active bool, ctrl *app.Controller) gui.View {
	fullTitle := tab.Title
	if tab.Kind == app.TabConnect {
		fullTitle = i18n.T("shell.tab.new_ellipsis")
	}
	// go-gui has no text-overflow: ellipsis. Shorten the label ourselves
	// and measure with the style Button renders its label with, so the
	// drawn text cannot spill into the neighbouring chip.
	label := layout.Ellipsize(fullTitle, tabTitleBudget, func(s string) float32 {
		return w.TextWidth(s, gui.DefaultTextStyle)
	})

	clear := gui.Flat(gui.ColorTransparent)
	chipID := "tab-chip-" + tab.ID

	return gui.Interactive(chipID, func(is gui.InteractionState) gui.View {
		bg := gui.Color{}
		switch {
		case active:
			bg = th.ColorActive
		case is.Hovered:
			bg = th.ColorHover
		}

		views := []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:       gui.FillFit,
				Color:        bg,
				Radius:       gui.RadiusSmall,
				VAlign:       gui.VAlignMiddle,
				Spacing:      gui.NoSpacing,
				Padding:      gui.NewPadding(0, 0, 0, 8),
				Clip:         true,
				ClipContents: true,
				Content: []gui.View{
					statusDot(th, tab),
					gui.Button(gui.ButtonCfg{
						ID:            "tab-" + tab.ID,
						Variant:       gui.ButtonGhost,
						Label:         label,
						Colors:        clear,
						Radius:        gui.NoRadius,
						SizeBorder:    gui.NoBorder,
						Padding:       gui.NewPadding(6, tabTitlePadR, 6, tabTitlePadL),
						MaxWidth:      tabTitleMaxWidth,
						FocusDisabled: true,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							gui.State[app.State](ctx.Window).ActiveTab = tab.ID
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:         "tab-close-" + tab.ID,
						Variant:    gui.ButtonGhost,
						Colors:     clear,
						Radius:     gui.NoRadius,
						SizeBorder: gui.NoBorder,
						// FitFill stretches the X to the chip's full height so
						// the tight padding does not shrink its hit area.
						Sizing:        gui.FitFill,
						Padding:       gui.NewPadding(0, tabClosePadR, 0, tabClosePadL),
						FocusDisabled: true,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconClose,
								TextStyle: th.TextStyleIconSmall,
							}),
						},
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctrl.CloseTab(ctx.Window, tab.ID)
						},
					}),
				},
			})),
		}
		if is.Hovered {
			// Tooltip keeps the full title even though the label clipped.
			views = append(views, tabTip("tab-tip-"+tab.ID, fullTitle))
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			ID:      chipID,
			Sizing:  gui.FitFit,
			Content: views,
		}))
	})
}

// statusDot is the colour-only connection indicator in front of the title.
func statusDot(th gui.Theme, tab app.Tab) gui.View {
	return gui.Rectangle(gui.RectangleCfg{
		ID:     "tab-dot-" + tab.ID,
		Sizing: gui.FixedFixed,
		Width:  tabDotSize,
		Height: tabDotSize,
		Radius: tabDotRadius,
		Color:  dotColor(th, tab.Status),
	})
}

func dotColor(th gui.Theme, st app.TabStatus) gui.Color {
	switch st {
	case app.TabStatusConnecting:
		return th.TextStyleWarning.Color
	case app.TabStatusConnected:
		return th.TextStyleSuccess.Color
	case app.TabStatusDisconnected:
		return th.TextStyleError.Color
	default:
		// Idle: the muted "not connected" grey from the theme.
		return th.TextStyleSecondary.Color
	}
}

func tabAddBtn(th gui.Theme, ctrl *app.Controller) gui.View {
	const addID = "tab-add-wrap"
	clear := gui.Flat(gui.ColorTransparent)
	return gui.Interactive(addID, func(is gui.InteractionState) gui.View {
		bg := gui.Color{}
		if is.Hovered {
			bg = th.ColorHover
		}
		views := []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            "tab-add",
				Variant:       gui.ButtonGhost,
				Colors:        clear,
				Color:         bg,
				Radius:        gui.RadiusSmall,
				SizeBorder:    gui.NoBorder,
				Padding:       gui.NewPadding(6, 10, 6, 10),
				FocusDisabled: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      gui.IconPlus,
						TextStyle: th.TextStyleIconSmall,
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					gui.State[app.State](ctx.Window).OpenConnectTab(ctrl.NewSSHDraft)
				},
			}),
		}
		if is.Hovered {
			views = append(views, tabTip("tab-add-tip", i18n.T("shell.tab.new")))
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			ID:      addID,
			Sizing:  gui.FitFit,
			Content: views,
		}))
	})
}

// tabTip uses the built-in Tooltip popup (theme colors/shadow), not
// WithTooltip's padded wrapper.
func tabTip(id, text string) gui.View {
	return gui.Tooltip(gui.TooltipCfg{
		ID: id,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: text,
				Mode: gui.TextModeWrap,
			}),
		},
	})
}
