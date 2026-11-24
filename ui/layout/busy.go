package layout

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

// BusyDialog shows a modal spinner dialog for a long-running operation.
// Escape stays disabled; onCancel (when non-nil) renders a Cancel button
// that invokes it. The caller dismisses the dialog when the operation
// finishes (QueueCommand + DialogDismiss).
func BusyDialog(w *gui.Window, title, detail string, onCancel func()) {
	th := w.Theme()
	content := []gui.View{
		gui.SvgSpinner(gui.SvgSpinnerCfg{
			Kind:   gui.SvgSpinner90Ring,
			Width:  36,
			Height: 36,
		}),
	}
	if detail != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      detail,
			TextStyle: th.TextStyleSecondary,
			Mode:      gui.TextModeWrap,
		}))
	}
	if onCancel != nil {
		content = append(content, gui.Button(gui.ButtonCfg{
			ID:      "busy-cancel",
			Label:   i18n.T("common.cancel"),
			Variant: gui.ButtonSecondary,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				onCancel()
			},
		}))
	}
	w.Dialog(gui.DialogCfg{
		DialogType:     gui.DialogCustom,
		Title:          title,
		Width:          320,
		EscapeDisabled: true,
		CustomView: func(*gui.Window) gui.View {
			return gui.Column(Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				HAlign:  gui.HAlignCenter,
				Content: content,
			}))
		},
	})
}
