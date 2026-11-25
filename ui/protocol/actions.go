package protocol

import "github.com/go-gui-org/go-gui/gui"

// FormActionBtn is a variant button that shows a spinner while BusyOp
// matches op. go-gui has no built-in Button.Loading.
func FormActionBtn(
	th gui.Theme,
	id, label, loadingLabel, op string,
	variant gui.ButtonVariant,
	busy bool, busyOp string,
	onClick func(gui.EventCtx),
) gui.View {
	cfg := gui.ButtonCfg{
		ID:       id,
		Variant:  variant,
		Disabled: busy,
		OnClick:  onClick,
	}
	if busy && busyOp == op {
		spinColor := th.TextStyleBody.Color
		labelStyle := th.TextStyleBody
		if variant == gui.ButtonPrimary {
			spinColor = th.ColorTextOnAccent
			labelStyle.Color = th.ColorTextOnAccent
		}
		cfg.Content = []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				Spacing: gui.SpacingTight,
				Padding: gui.PaddingNone,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.SvgSpinner(gui.SvgSpinnerCfg{
						ID:     id + "-spin",
						Kind:   gui.SvgSpinner90Ring,
						Width:  16,
						Height: 16,
						Color:  spinColor,
					}),
					gui.Text(gui.TextCfg{Text: loadingLabel, TextStyle: labelStyle}),
				},
			}),
		}
	} else {
		cfg.Label = label
	}
	return gui.Button(cfg)
}
