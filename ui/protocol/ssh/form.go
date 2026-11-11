package sshui

import (
	"strconv"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/protocol"
)

// Form renders the basic SSH fields into draft.Payload (*Payload).
// Key/cert/jump live in bookmarks later — not on this quick form.
// Layout chrome (centering / protocol switch) lives in session.FormHost.
func Form(bus protocol.Bus, draft *protocol.Draft) gui.View {
	p := ensurePayload(draft)
	th := gui.CurrentTheme()
	busy, busyOp := bus.Busy()

	field := func(id, label, text string, password bool, onChange func(string)) gui.View {
		return gui.Input(gui.InputCfg{
			ID:         id,
			Label:      label,
			Text:       text,
			IsPassword: password,
			Sizing:     gui.FillFit,
			Disabled:   busy,
			OnTextChanged: func(v string, ctx gui.EventCtx) {
				onChange(v)
				ctx.Consume()
			},
		})
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		HAlign:  gui.HAlignStart,
		Content: []gui.View{
			field("ssh-host", i18n.T("bookmark.field.host")+"*", p.Host, false, func(v string) { p.Host = v }),
			field("ssh-port", i18n.T("bookmark.field.port")+"*", strconv.Itoa(p.Port), false, func(v string) {
				n, err := strconv.Atoi(v)
				if err == nil {
					p.Port = n
				}
			}),
			field("ssh-user", i18n.T("bookmark.field.username")+"*", p.User, false, func(v string) { p.User = v }),
			field("ssh-pass", i18n.T("bookmark.field.password"), p.Password, true, func(v string) { p.Password = v }),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					protocol.FormActionBtn(th, "ssh-test",
						i18n.T("bookmark.test_connect"), i18n.T("shell.action.testing"), "test",
						gui.ButtonSecondary, busy, busyOp, func(ctx gui.EventCtx) {
							ctx.Consume()
							bus.Test(*draft)
						}),
					protocol.FormActionBtn(th, "ssh-connect",
						i18n.T("common.connect"), i18n.T("shell.action.connecting"), "connect",
						gui.ButtonPrimary, busy, busyOp, func(ctx gui.EventCtx) {
							ctx.Consume()
							bus.Connect(*draft)
						}),
					gui.Button(gui.ButtonCfg{
						ID:       "ssh-save",
						Variant:  gui.ButtonGhost,
						Label:    i18n.T("common.save"),
						Disabled: busy,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctx.Window.Toast(gui.ToastCfg{
								Body:     i18n.T("shell.placeholder.save_bookmark"),
								Severity: gui.ToastInfo,
							})
						},
					}),
				},
			})),
		},
	}))
}

func ensurePayload(draft *protocol.Draft) *Payload {
	if draft == nil {
		return &Payload{Port: 22}
	}
	switch v := draft.Payload.(type) {
	case *Payload:
		if v.Port <= 0 {
			v.Port = 22
		}
		return v
	case Payload:
		p := v
		if p.Port <= 0 {
			p.Port = 22
		}
		draft.Payload = &p
		return &p
	default:
		p := DefaultPayload()
		draft.Payload = &p
		return &p
	}
}
