package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
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
	toolSFTP     = "sftp"
	toolPerf     = "perf"
	toolTunnel   = "tunnel"
	toolTransfer = "transfer"
)

// TabRail is the right-side tool strip bound to the active live
// session tab, top to bottom: sftp | performance | tunnel | transfer
// list.
func TabRail(ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	perfOpen := tab.PerfOpen

	// The SFTP toggle only exists for connected SSH sessions; other
	// tabs keep the slot empty rather than showing a dead button.
	var sftpBtn gui.View
	if app.SFTPControls(tab) {
		sftpBtn = toolBtn(toolSFTP, gui.IconFolderOpen, i18n.T("shell.tools.sftp"), th, tab.SFTPOpen, func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.ToggleSFTP(ctx.Window, tab.ID)
		})
	}

	// The three side panels share the stage width, so TogglePanel
	// keeps at most one of them open per tab.
	perfBtn := toolBtn(toolPerf, gui.IconActivity, i18n.T("shell.tools.perf"), th, perfOpen, func(ctx gui.EventCtx) {
		ctx.Consume()
		ctrl.TogglePanel(ctx.Window, tab.ID, app.PanelPerf)
	})

	tunnelBtn := toolBtn(toolTunnel, gui.IconPlug, i18n.T("shell.tools.tunnel"), th, tab.TunnelOpen, func(ctx gui.EventCtx) {
		ctx.Consume()
		ctrl.TogglePanel(ctx.Window, tab.ID, app.PanelTunnel)
	})

	// The transfer list opens the transfer panel.
	transferBtn := toolBtn(toolTransfer, gui.IconListTask, i18n.T("shell.tools.transfer"), th, tab.TransferOpen, func(ctx gui.EventCtx) {
		ctx.Consume()
		ctrl.TogglePanel(ctx.Window, tab.ID, app.PanelTransfer)
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
					sftpBtn,
					perfBtn,
					tunnelBtn,
					transferBtn,
				},
			})),
		},
	}))
}

// toolBtn mirrors shell.railBtn; the hover tip floats to the left
// because this rail hugs the window's right edge.
func toolBtn(
	id, glyph, tip string,
	th gui.Theme,
	active bool,
	onClick func(gui.EventCtx),
) gui.View {
	return layout.IconTipButton(layout.IconTipButtonCfg{
		ID:        "tool-" + id,
		Glyph:     glyph,
		Tip:       tip,
		Th:        th,
		OnClick:   onClick,
		Variant:   gui.ButtonGhost,
		Active:    active,
		Size:      toolBtnSize,
		Icon:      th.TextStyleIconLarge,
		Padding:   gui.PaddingXSmall,
		Radius:    gui.RadiusSmall,
		Placement: layout.TipRight,
	})
}
