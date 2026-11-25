package backup

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

func showChoose(w *gui.Window, d *Deps) {
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("header.backup_restore"),
		Width:      360,
		CustomView: func(*gui.Window) gui.View {
			th := gui.CurrentTheme()
			return gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "backup-choose-upload",
						Variant: gui.ButtonPrimary,
						Sizing:  gui.FillFit,
						Content: []gui.View{
							gui.Row(layout.Flush(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SpacingSmall,
								VAlign:  gui.VAlignMiddle,
								Content: []gui.View{
									gui.Text(gui.TextCfg{Text: gui.IconUpload, TextStyle: th.TextStyleIconMedium}),
									gui.Text(gui.TextCfg{Text: i18n.T("header.backup"), TextStyle: th.TextStyleBody}),
								},
							})),
						},
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctx.Window.DialogDismiss()
							showConfirmUpload(ctx.Window, d)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "backup-choose-restore",
						Variant: gui.ButtonSecondary,
						Sizing:  gui.FillFit,
						Content: []gui.View{
							gui.Row(layout.Flush(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SpacingSmall,
								VAlign:  gui.VAlignMiddle,
								Content: []gui.View{
									gui.Text(gui.TextCfg{Text: gui.IconDownload, TextStyle: th.TextStyleIconMedium}),
									gui.Text(gui.TextCfg{Text: i18n.T("header.restore"), TextStyle: th.TextStyleBody}),
								},
							})),
						},
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctx.Window.DialogDismiss()
							openRestoreList(ctx.Window, d)
						},
					}),
					dialogButtons(gui.Button(gui.ButtonCfg{
						ID:      "backup-choose-cancel",
						Variant: gui.ButtonSecondary,
						Label:   i18n.T("common.cancel"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctx.Window.DialogDismiss()
						},
					})),
				},
			}))
		},
	})
}
