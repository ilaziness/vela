package transferui

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const transferPanelWidth float32 = 360

// TransferPanel is the transfer list panel shown between the terminal
// and the tool rail, rendered top-to-bottom: header, scrollable list.
func TransferPanel(ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	sessID := tab.SessID
	transfers := ctrl.TransferSnapshot(sessID)

	header := gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "transfer-header-" + tab.ID,
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 6, 0, 2),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      i18n.T("transfer.title"),
				TextStyle: th.TextStyleLabel,
				Sizing:    gui.FillFit,
			}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("%d", len(transfers)),
				TextStyle: th.TextStyleSecondary,
				Sizing:    gui.FitFit,
			}),
			gui.Button(gui.ButtonCfg{
				ID:            "transfer-clear-finished",
				Label:         i18n.T("transfer.clear_finished"),
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctrl.SFTPClearFinished(ctx.Window, sessID, false)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:            "transfer-clear-failed",
				Label:         i18n.T("transfer.clear_failed"),
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctrl.SFTPClearFinished(ctx.Window, sessID, true)
				},
			}),
		},
	}))

	if len(transfers) == 0 {
		body := gui.Label(i18n.T("transfer.empty"), th.TextStyleSecondary)
		return transferColumn(tab, header, body)
	}

	rows := make([]gui.View, 0, len(transfers)*2)
	for i, p := range transfers {
		rows = append(rows, transferRow(ctrl, tab, p, i))
		if i < len(transfers)-1 {
			rows = append(rows, hDivider(th))
		}
	}

	body := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:         "transfer-list",
		Sizing:     gui.FillFill,
		Scrollable: true,
		ScrollMode: gui.ScrollVerticalOnly,
		// No ClipContents: its stencil masks the overlaid scrollbar too,
		// so the list scrolled with no bar. Clip still scissors the rows.
		Clip:    true,
		Spacing: gui.NoSpacing,
		// Right gutter keeps rows clear of the overlay scrollbar.
		Padding: gui.NewPadding(0, 12, 0, 0),
		Content: rows,
	}))

	return transferColumn(tab, header, body)
}

func transferColumn(tab *app.Tab, header, body gui.View) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "transfer-panel-" + tab.ID,
		Sizing:  gui.FixedFill,
		Width:   transferPanelWidth,
		Color:   th.ColorPanel,
		Spacing: gui.NoSpacing,
		Content: []gui.View{
			header,
			body,
		},
	}))
}

// hDivider is the 1px horizontal separator between transfer rows.
func hDivider(th gui.Theme) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFixed,
		Height: 1,
		Color:  th.ColorInterior,
	}))
}
