package sftpui

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	transferColWidth  = 40
	transferBtnSize   = 26
	transferBarHeight = 110
)

// transferColumn is the narrow center column between the two panes:
// left arrow downloads the remote selection, right arrow uploads the
// local selection. Each button only enables while its pane has a
// selection.
func transferColumn(ctrl *app.Controller, tab *app.Tab, s *app.SFTPState) gui.View {
	th := gui.CurrentTheme()
	sessID := tab.SessID
	canDown := len(s.RemoteSelected) > 0
	canUp := len(s.LocalSelected) > 0
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "sftp-transfer-col",
		Sizing:  gui.FixedFill,
		Width:   transferColWidth,
		VAlign:  gui.VAlignMiddle,
		HAlign:  gui.HAlignCenter,
		Spacing: gui.SpacingTight,
		Color:   th.ColorPanel,
		Content: []gui.View{
			tipBtn("sftp-download", gui.IconArrowLeft, i18n.T("sftp.download"), th, transferBtnSize, gui.ButtonSecondary, canDown, !canDown, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPDownloadSelected(ctx.Window, sessID)
			}),
			tipBtn("sftp-upload", gui.IconArrowRight, i18n.T("sftp.upload"), th, transferBtnSize, gui.ButtonSecondary, canUp, !canUp, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPUploadSelected(ctx.Window, sessID)
			}),
		},
	}))
}

// transferBar renders the bottom transfer queue; nil when empty.
func transferBar(ctrl *app.Controller, sessID string, s *app.SFTPState) gui.View {
	if s == nil || len(s.Transfers) == 0 {
		return nil
	}
	th := gui.CurrentTheme()
	rows := make([]gui.View, 0, len(s.Transfers)+1)
	for i, p := range s.Transfers {
		rows = append(rows, transferRow(ctrl, sessID, p, i))
	}
	// Header row with a clear-finished action.
	header := gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "sftp-transfers-head",
		Sizing:  gui.FillFixed,
		Height:  22,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 6, 0, 2),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: i18n.T("transfer.title"), TextStyle: th.TextStyleLabel, Sizing: gui.FillFit}),
			gui.Text(gui.TextCfg{Text: fmt.Sprintf("%d", len(s.Transfers)), TextStyle: th.TextStyleSecondary, Sizing: gui.FitFit}),
			gui.Button(gui.ButtonCfg{
				ID:            "sftp-transfers-clear",
				Label:         i18n.T("transfer.clear_finished"),
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctrl.SFTPClearFinished(ctx.Window, sessID, false)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:            "sftp-transfers-clear-failed",
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
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "sftp-transfers",
		Sizing:  gui.FixedFill,
		Height:  transferBarHeight,
		Spacing: gui.NoSpacing,
		Color:   th.ColorPanel,
		Content: []gui.View{
			header,
			gui.Column(layout.Flush(gui.ContainerCfg{
				ID:           "sftp-transfers-list",
				Sizing:       gui.FillFill,
				Scrollable:   true,
				Clip:         true,
				ClipContents: true,
				Spacing:      gui.NoSpacing,
				Content:      rows,
			})),
		},
	}))
}

// transferRow is one queue entry: name, progress bar, status, actions.
func transferRow(ctrl *app.Controller, sessID string, p transfer.ProgressData, idx int) gui.View {
	th := gui.CurrentTheme()
	name := transferDisplayName(p)
	pct := float32(0)
	if p.TotalSize > 0 {
		pct = float32(p.Transferred) / float32(p.TotalSize)
	}
	status := transferStatusText(p)

	var actions []gui.View
	if !p.Done {
		actions = append(actions, miniBtn(fmt.Sprintf("sftp-t%d-cancel", idx), i18n.T("common.cancel"), th, func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.SFTPCancelTransfer(ctx.Window, sessID, p.ID)
		}))
	} else {
		if p.Error != "" {
			actions = append(actions, miniBtn(fmt.Sprintf("sftp-t%d-retry", idx), i18n.T("common.retry"), th, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPRetryTransfer(ctx.Window, sessID, p.ID)
			}))
		}
		actions = append(actions, miniBtn(fmt.Sprintf("sftp-t%d-dismiss", idx), i18n.T("transfer.dismiss"), th, func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.SFTPDismissTransfer(ctx.Window, sessID, p.ID)
		}))
	}

	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      fmt.Sprintf("sftp-t%d", idx),
		Sizing:  gui.FillFixed,
		Height:  26,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 6, 0, 2),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: transferTypeIcon(p.TransferType), TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
			gui.Text(gui.TextCfg{Text: name, TextStyle: th.TextStyleBody, Sizing: gui.FitFit, Clip: true}),
			gui.ProgressBar(gui.ProgressBarCfg{
				ID:         fmt.Sprintf("sftp-t%d-bar", idx),
				Percent:    pct,
				Sizing:     gui.FillFit,
				Height:     8,
				TextShow:   false,
				Indefinite: !p.Done && p.TotalSize <= 0,
			}),
			gui.Text(gui.TextCfg{Text: status, TextStyle: th.TextStyleSecondary, Sizing: gui.FitFit}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingTight,
				Content: actions,
			})),
		},
	}))
}

func miniBtn(id, label string, th gui.Theme, onClick func(gui.EventCtx)) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:            id,
		Label:         label,
		Variant:       gui.ButtonGhost,
		FocusDisabled: true,
		OnClick:       onClick,
	})
}
