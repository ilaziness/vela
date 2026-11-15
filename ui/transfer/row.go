package transferui

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// transferRow is one queue entry: name, progress bar, status, actions.
func transferRow(ctrl *app.Controller, tab *app.Tab, p transfer.ProgressData, idx int) gui.View {
	th := gui.CurrentTheme()
	sessID := tab.SessID
	name := transferDisplayName(p)
	pct := float32(0)
	if p.TotalSize > 0 {
		pct = float32(p.Transferred) / float32(p.TotalSize)
	}
	status := transferStatusText(p)

	// Ellipsize filename to fit max width
	maxNameWidth := float32(280)
	name = layout.Ellipsize(name, maxNameWidth, func(s string) float32 {
		return ctrl.Window().TextWidth(s, th.TextStyleBody)
	})

	// Full path shown in the tooltip; the visible path is ellipsized.
	fullPath := transferPathText(p)
	pathText := layout.Ellipsize(fullPath, 280, func(s string) float32 {
		return ctrl.Window().TextWidth(s, th.TextStyleSecondary)
	})

	// Icon color: use different color for upload vs download. The style
	// keeps the Feather icon family; only the color changes.
	iconStyle := th.TextStyleIconSmall
	if p.TransferType == transfer.TypeUpload {
		iconStyle.Color = th.TextStyleSuccess.Color
	} else {
		iconStyle.Color = th.TextStyleWarning.Color
	}

	var actions []gui.View
	if !p.Done {
		actions = append(actions, miniBtn(fmt.Sprintf("transfer-t%d-cancel", idx), "×", i18n.T("common.cancel"), func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.SFTPCancelTransfer(ctx.Window, sessID, p.ID)
		}))
	} else {
		if p.Error != "" {
			actions = append(actions, miniBtn(fmt.Sprintf("transfer-t%d-retry", idx), "↻", i18n.T("common.retry"), func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPRetryTransfer(ctx.Window, sessID, p.ID)
			}))
		}
		actions = append(actions, miniBtn(fmt.Sprintf("transfer-t%d-dismiss", idx), "✕", i18n.T("transfer.dismiss"), func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.SFTPDismissTransfer(ctx.Window, sessID, p.ID)
		}))
	}

	// First column: icon + name on same line, path below, progress bar below
	leftCol := gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingTight,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: transferTypeIcon(p.TransferType), TextStyle: iconStyle, Sizing: gui.FitFit}),
					tipCell(fmt.Sprintf("transfer-t%d-name", idx), transferNameTip(p), name, th.TextStyleBody),
				},
			})),
			tipCell(fmt.Sprintf("transfer-t%d-path", idx), fullPath, pathText, th.TextStyleSecondary),
			gui.ProgressBar(gui.ProgressBarCfg{
				ID:         fmt.Sprintf("transfer-t%d-bar", idx),
				Percent:    pct,
				Sizing:     gui.FillFill,
				Height:     6,
				TextShow:   false,
				Indefinite: !p.Done && p.TotalSize <= 0,
			}),
		},
	}))

	// Second column: status + actions
	rightCol := gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FitFill,
		Spacing: gui.SpacingTight,
		HAlign:  gui.HAlignRight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: status, TextStyle: th.TextStyleSecondary, Sizing: gui.FitFit}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingTight,
				Content: actions,
			})),
		},
	}))

	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      fmt.Sprintf("transfer-t%d", idx),
		Sizing:  gui.FillFixed,
		Height:  64,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingMedium,
		Padding: gui.NewPadding(0, 6, 0, 2),
		Content: []gui.View{
			leftCol,
			rightCol,
		},
	}))
}

// tipCell renders ellipsized text and floats tip on hover. The
// Interactive root must carry the same ID or the hover state never
// turns true (gui.Interactive contract).
func tipCell(id, tip, display string, style gui.TextStyle) gui.View {
	return gui.Interactive(id, func(is gui.InteractionState) gui.View {
		content := []gui.View{
			gui.Text(gui.TextCfg{Text: display, TextStyle: style, Sizing: gui.FillFit, Clip: true}),
		}
		if is.Hovered {
			content = append(content, layout.Tip(id+"-tip", tip,
				gui.FloatBottomCenter, gui.FloatTopCenter, 0, 4))
		}
		return gui.Row(layout.Flush(gui.ContainerCfg{
			ID:      id,
			Sizing:  gui.FillFit,
			VAlign:  gui.VAlignMiddle,
			Clip:    true,
			Content: content,
		}))
	})
}

// miniBtn wraps a button with a hover tooltip, same Interactive
// contract as tipCell.
func miniBtn(id, label, tip string, onClick func(gui.EventCtx)) gui.View {
	return gui.Interactive(id, func(is gui.InteractionState) gui.View {
		content := []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            id + "-btn",
				Label:         label,
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				OnClick:       onClick,
			}),
		}
		if is.Hovered && tip != "" {
			content = append(content, layout.Tip(id+"-tip", tip,
				gui.FloatBottomCenter, gui.FloatTopCenter, 0, 4))
		}
		return gui.Row(layout.Flush(gui.ContainerCfg{
			ID:      id,
			Sizing:  gui.FillFit,
			VAlign:  gui.VAlignMiddle,
			Content: content,
		}))
	})
}
