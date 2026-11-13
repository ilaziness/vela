package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// Tool-rail geometry mirrors the far-left icon rail in ui/shell
// (width 30, 1px inset, nearly filling buttons).
const (
	toolRailWidth float32 = 30
	toolRailInset float32 = 1
	toolBtnSize           = toolRailWidth - toolRailInset*2
)

// Tool IDs for the right-side per-tab rail.
const (
	toolSSH      = "ssh"
	toolPerf     = "perf"
	toolTunnel   = "tunnel"
	toolTransfer = "transfer"
)

// TabRail is the right-side tool strip bound to the active live
// session tab: ssh | performance | tunnel | transfer list. Only
// performance is wired; the rest are placeholders.
func TabRail(ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	perfOpen := tab.PerfOpen

	sshBtn := toolBtn(toolSSH, gui.IconTerminal, i18n.T("shell.tools.ssh"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		placeholder(ctrl, i18n.T("shell.tools.ssh"))
	})

	perfBtn := toolBtn(toolPerf, gui.IconActivity, i18n.T("shell.tools.perf"), th, perfOpen, func(ctx gui.EventCtx) {
		ctx.Consume()
		st := gui.State[app.State](ctx.Window)
		t := st.TabByID(tab.ID)
		if t == nil {
			return
		}
		if t.Protocol != termio.ProtocolSSH {
			// Host metrics come from the SSH probe (/proc); a serial
			// link has nothing to sample.
			ctrl.SetStatus(i18n.T("shell.perf.unsupported"))
			return
		}
		t.PerfOpen = !t.PerfOpen
		if t.PerfOpen {
			ctrl.StartPerf(ctx.Window, t.SessID)
		} else {
			ctrl.StopPerf(t.SessID)
		}
	})

	tunnelBtn := toolBtn(toolTunnel, gui.IconLink, i18n.T("shell.tools.tunnel"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		placeholder(ctrl, i18n.T("shell.tools.tunnel"))
	})

	transferBtn := toolBtn(toolTransfer, gui.IconDownload, i18n.T("shell.tools.transfer"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		placeholder(ctrl, i18n.T("shell.tools.transfer"))
	})

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "tool-rail",
		Sizing:  gui.FixedFill,
		Width:   toolRailWidth,
		Color:   th.ColorPanel,
		HAlign:  gui.HAlignCenter,
		Padding: gui.NewPadding(0, toolRailInset, 0, toolRailInset),
		Content: []gui.View{
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingTight,
				Padding: gui.NewPadding(4, 0, 4, 0),
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					sshBtn,
					perfBtn,
					tunnelBtn,
					transferBtn,
				},
			})),
		},
	}))
}

func placeholder(ctrl *app.Controller, name string) {
	ctrl.SetStatus(i18n.Tf("shell.placeholder.feature", name))
}

// toolBtn mirrors shell.railBtn; the hover tip floats to the left
// because this rail hugs the window's right edge.
func toolBtn(
	id, glyph, tip string,
	th gui.Theme,
	active bool,
	onClick func(gui.EventCtx),
) gui.View {
	cellID := "tool-" + id
	return gui.Interactive(cellID, func(is gui.InteractionState) gui.View {
		bg := gui.Color{}
		if active {
			bg = th.ColorActive
		}
		views := []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            cellID + "-hit",
				Variant:       gui.ButtonGhost,
				Sizing:        gui.FillFill,
				Padding:       gui.PaddingXSmall,
				Color:         bg,
				Radius:        gui.RadiusSmall,
				HAlign:        gui.Some(gui.HAlignCenter),
				VAlign:        gui.Some(gui.VAlignMiddle),
				FocusDisabled: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      glyph,
						TextStyle: th.TextStyleIconLarge,
					}),
				},
				OnClick: onClick,
			}),
		}
		if is.Hovered && tip != "" {
			views = append(views, toolTip(cellID+"-tip", tip, th))
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			ID:      cellID,
			Sizing:  gui.FixedFixed,
			Width:   toolBtnSize,
			Height:  toolBtnSize,
			Radius:  gui.RadiusSmall,
			HAlign:  gui.HAlignCenter,
			VAlign:  gui.VAlignMiddle,
			Content: views,
		}))
	})
}

// toolTip is the left-floating sibling of shell.railTip: anchor the
// middle-right edge of the button, tie off to the tip's middle-left.
func toolTip(id, text string, th gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   gui.FloatMiddleRight,
		FloatTieOff:   gui.FloatMiddleLeft,
		FloatOffsetX:  8,
		FloatZIndex:   10,
		Color:         th.ColorInterior,
		ColorBorder:   th.ColorBorder,
		SizeBorder:    gui.BorderThin,
		Radius:        gui.RadiusSmall,
		Padding:       gui.PaddingSmall,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      text,
				TextStyle: th.TextStyleBody,
			}),
		},
	})
}
