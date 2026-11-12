package backup

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/layout"
)

func showBusy(w *gui.Window, message string) {
	w.Dialog(gui.DialogCfg{
		DialogType:     gui.DialogCustom,
		Title:          message,
		Width:          320,
		EscapeDisabled: true,
		CustomView: func(*gui.Window) gui.View {
			return gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.SvgSpinner(gui.SvgSpinnerCfg{
						Kind:   gui.SvgSpinner90Ring,
						Width:  36,
						Height: 36,
					}),
				},
			}))
		},
	})
}
