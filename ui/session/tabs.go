package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const tabTitleMaxWidth float32 = 160

// TabBar is flush to the top of the main pane.
// Tips use gui.Tooltip on hover — not WithTooltip (wrapper forces theme padding).
func TabBar(w *gui.Window, ctrl *app.Controller) gui.View {
	st := gui.State[app.State](w)
	th := gui.CurrentTheme()
	items := make([]gui.View, 0, len(st.Tabs)+1)
	for _, t := range st.Tabs {
		items = append(items, tabChip(th, t, t.ID == st.ActiveTab, ctrl))
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

// tabChip is one title+close unit sharing a single fill so they read as
// one tab, not two adjacent ghost buttons.
func tabChip(th gui.Theme, tab app.Tab, active bool, ctrl *app.Controller) gui.View {
	fullTitle := tab.Title
	if tab.Kind == app.TabConnect {
		fullTitle = i18n.T("shell.tab.new_ellipsis")
	}
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
				Sizing: gui.FillFit,
				Color:  bg,
				Radius: gui.RadiusSmall,
				VAlign: gui.VAlignMiddle,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:            "tab-" + tab.ID,
						Variant:       gui.ButtonGhost,
						Label:         fullTitle,
						Colors:        clear,
						Radius:        gui.NoRadius,
						SizeBorder:    gui.NoBorder,
						Padding:       gui.NewPadding(6, 10, 6, 4),
						MaxWidth:      tabTitleMaxWidth,
						FocusDisabled: true,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							gui.State[app.State](ctx.Window).ActiveTab = tab.ID
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:            "tab-close-" + tab.ID,
						Variant:       gui.ButtonGhost,
						Colors:        clear,
						Radius:        gui.NoRadius,
						SizeBorder:    gui.NoBorder,
						Padding:       gui.NewPadding(6, 6, 6, 8),
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
			views = append(views, tabTip("tab-tip-"+tab.ID, fullTitle))
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			ID:      chipID,
			Sizing:  gui.FitFit,
			Content: views,
		}))
	})
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
