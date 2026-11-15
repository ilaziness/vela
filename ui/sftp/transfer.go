package sftpui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	transferColWidth = 40
	transferBtnSize  = 26
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
