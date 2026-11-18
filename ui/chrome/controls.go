package chrome

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

// ControlWidth is the width of one window-control button. Height comes
// from the strip (FixedFill), so the hover fill runs edge to edge like
// a native caption button.
const ControlWidth float32 = 36

// Controls is the minimize/maximize/close cluster, placed at the right
// edge of a title strip. Close routes through the window's
// OnCloseRequest hook, so unsaved-changes prompts and shutdown paths
// keep working exactly like the native caption button.
func Controls(w *gui.Window) gui.View {
	th := gui.CurrentTheme()
	return gui.Row(layout.Flush(gui.ContainerCfg{
		// FitFill hugs the three buttons so the cluster sits at the
		// right edge; a Fill width would split the free space with
		// the caller's spacer and leave a gap before the buttons.
		Sizing: gui.FitFill,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			control("chrome-min", gui.IconMinus, i18n.T("window.minimize"), th, gui.Color{}, func(ctx gui.EventCtx) {
				ctx.Window.Minimize()
				ctx.Consume()
			}),
			control("chrome-max", maximizeIcon(w), maximizeLabel(w), th, gui.Color{}, func(ctx gui.EventCtx) {
				if ctx.Window.IsMaximized() {
					ctx.Window.Restore()
				} else {
					ctx.Window.Maximize()
				}
				ctx.Consume()
			}),
			// Only the close button gets the conventional red hover;
			// an unset Hover leaves every other state on the ghost
			// variant's own colors.
			control("chrome-close", gui.IconClose, i18n.T("common.close"), th, th.ColorErrorSubtle, func(ctx gui.EventCtx) {
				gui.DispatchCloseRequest(ctx.Window)
				ctx.Consume()
			}),
		},
	}))
}

// control is one ghost icon button with a hover tooltip. ButtonGhost
// already supplies the theme hover fill; FocusDisabled keeps the strip
// out of the tab order. The Interactive wrapper exists only to show
// the tip, the same pattern session.tabAddBtn uses. hover overrides
// the ghost hover fill (close button's red); the zero Color keeps it.
func control(id, icon, label string, th gui.Theme, hover gui.Color, onClick func(gui.EventCtx)) gui.View {
	wrapID := id + "-wrap"
	return gui.Interactive(wrapID, func(is gui.InteractionState) gui.View {
		views := []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            id,
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				Sizing:        gui.FixedFill,
				Width:         ControlWidth,
				Colors:        gui.ColorSet{Hover: hover},
				A11YCfg:       gui.A11YCfg{A11YLabel: label},
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: icon, TextStyle: th.TextStyleIconSmall}),
				},
				OnClick: onClick,
			}),
		}
		if is.Hovered {
			views = append(views, tipBelow(id+"-tip", label))
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			ID:      wrapID,
			Sizing:  gui.FitFill,
			Content: views,
		}))
	})
}

// tipBelow hangs the bubble under the control: these buttons live on
// the window's top edge, so the tip opens downward like a native
// caption tooltip.
func tipBelow(id, text string) gui.View {
	return layout.Tooltip(id, text, gui.FloatBottomCenter, gui.FloatTopCenter, -3, -3)
}

func maximizeIcon(w *gui.Window) string {
	if w.IsMaximized() {
		return gui.IconCompress
	}
	return gui.IconExpand
}

func maximizeLabel(w *gui.Window) string {
	if w.IsMaximized() {
		return i18n.T("window.restore")
	}
	return i18n.T("window.maximize")
}
