package toolsui

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	toolsNavWidth float32 = 200
	multiMinH     float32 = 160
)

func toastOK(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastSuccess, Duration: 2 * time.Second})
}

func toastErr(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastError, Duration: 3 * time.Second})
}

func copyText(w *gui.Window, text string) {
	if text == "" {
		return
	}
	w.SetClipboard(text)
	toastOK(w, i18n.T("common.copied"))
}

func fieldInput(id, label, text, placeholder string, onChange func(string)) gui.View {
	return gui.Input(gui.InputCfg{
		ID:          id,
		Label:       label,
		Text:        text,
		Placeholder: placeholder,
		Sizing:      gui.FillFit,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			onChange(v)
			ctx.Consume()
		},
	})
}

func multiInput(id, label, text, placeholder string, minH float32, onChange func(string)) gui.View {
	return gui.Input(gui.InputCfg{
		ID:          id,
		Label:       label,
		Text:        text,
		Placeholder: placeholder,
		Mode:        gui.InputMultiline,
		MinHeight:   minH,
		Sizing:      gui.FillFit,
		Scrollable:  true,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			onChange(v)
			ctx.Consume()
		},
	})
}

func multiReadonly(id, label, text string, minH float32) gui.View {
	return gui.Input(gui.InputCfg{
		ID:         id,
		Label:      label,
		Text:       text,
		Mode:       gui.InputMultiline,
		MinHeight:  minH,
		Sizing:     gui.FillFit,
		ReadOnly:   true,
		Scrollable: true,
	})
}

func primaryBtn(id, label string, disabled bool, onClick func(gui.EventCtx)) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:       id,
		Variant:  gui.ButtonPrimary,
		Label:    label,
		Sizing:   gui.FillFit,
		Disabled: disabled,
		OnClick:  onClick,
	})
}

func secondaryBtn(id, label string, onClick func(gui.EventCtx)) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:      id,
		Variant: gui.ButtonSecondary,
		Label:   label,
		Sizing:  gui.FillFit,
		OnClick: onClick,
	})
}

func formCard(content ...gui.View) gui.View {
	return panelCard(gui.SpacingMedium, content...)
}

func resultCard(content ...gui.View) gui.View {
	return panelCard(gui.SpacingSmall, content...)
}

func panelCard(spacing gui.Spacing, content ...gui.View) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:      gui.FillFit,
		Color:       th.ColorPanel,
		Radius:      gui.RadiusMedium,
		Padding:     gui.PaddingMedium,
		Spacing:     spacing,
		HAlign:      gui.HAlignStart,
		Content:     content,
		SizeBorder:  gui.BorderThin,
		ColorBorder: th.ColorBorder,
	})
}

func statusLine(ok bool, text string) gui.View {
	th := gui.CurrentTheme()
	style := th.TextStyleError
	if ok {
		style = th.TextStyleSuccess
	}
	return gui.Text(gui.TextCfg{Text: text, TextStyle: style})
}

func warnLine(text string) gui.View {
	th := gui.CurrentTheme()
	return gui.Text(gui.TextCfg{Text: text, TextStyle: th.TextStyleWarning})
}

func panelShell(title, desc string, body []gui.View) gui.View {
	th := gui.CurrentTheme()
	content := []gui.View{
		gui.Text(gui.TextCfg{Text: title, TextStyle: th.TextStyleTitle}),
		gui.Label(desc, th.TextStyleSecondary),
	}
	content = append(content, body...)
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Scrollable: true,
		Spacing:    gui.SpacingMedium,
		Padding:    gui.PaddingMedium,
		HAlign:     gui.HAlignStart,
		VAlign:     gui.VAlignTop,
		Content:    content,
	}))
}

func selectField(id, label, value string, opts []gui.SelectOption, onChange func(string)) gui.View {
	return gui.Select(gui.SelectCfg{
		ID:       id,
		Label:    label,
		Selected: []string{value},
		Options:  opts,
		Sizing:   gui.FillFit,
		OnSelect: func(vals []string, ctx gui.EventCtx) {
			ctx.Consume()
			if len(vals) > 0 {
				onChange(vals[0])
			}
		},
	})
}

func switchField(id, label string, on bool, toggle func()) gui.View {
	return gui.Switch(gui.SwitchCfg{
		ID:       id,
		Label:    label,
		Selected: on,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			toggle()
		},
	})
}
