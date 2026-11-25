package tunnelui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/tunnel"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// formState lives in the DialogCustom closure, like vaultprompt's
// reply/failMsg pair: CustomView rebuilds every frame from it.
type formState struct {
	typ    string
	local  string
	remote string
	port   string
	errMsg string
}

// OpenForm shows the create-tunnel dialog for a session tab.
func OpenForm(w *gui.Window, ctrl *app.Controller, tab *app.Tab) {
	fs := &formState{typ: tunnel.TypeLocal}
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("tunnel.create_dialog_title"),
		Width:      400,
		FocusID:    "tunnel-form-local",
		CustomView: func(*gui.Window) gui.View {
			return formBody(fs, ctrl, tab)
		},
	})
}

func formBody(fs *formState, ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	content := []gui.View{
		gui.Select(gui.SelectCfg{
			ID:       "tunnel-form-type",
			Label:    i18n.T("tunnel.type_label"),
			Selected: []string{fs.typ},
			Options: []gui.SelectOption{
				{Label: typeLabel(tunnel.TypeLocal), Value: tunnel.TypeLocal},
				{Label: typeLabel(tunnel.TypeRemote), Value: tunnel.TypeRemote},
				{Label: typeLabel(tunnel.TypeDynamic), Value: tunnel.TypeDynamic},
			},
			Sizing: gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					fs.typ = vals[0]
					fs.errMsg = ""
				}
			},
		}),
	}

	switch fs.typ {
	case tunnel.TypeLocal:
		content = append(content,
			addrInput("tunnel-form-local", i18n.T("tunnel.local_address"), "validation.example_local_listen", fs.local,
				func(v string) { fs.local = v }, fs, ctrl, tab),
			addrInput("tunnel-form-remote", i18n.T("tunnel.remote_address"), "tunnel.example_remote_addr", fs.remote,
				func(v string) { fs.remote = v }, fs, ctrl, tab),
		)
	case tunnel.TypeRemote:
		content = append(content,
			addrInput("tunnel-form-local", i18n.T("tunnel.local_target"), "validation.example_local_target", fs.local,
				func(v string) { fs.local = v }, fs, ctrl, tab),
			portInput(fs, ctrl, tab),
		)
	case tunnel.TypeDynamic:
		content = append(content,
			addrInput("tunnel-form-local", i18n.T("tunnel.local_address"), "validation.example_local_listen", fs.local,
				func(v string) { fs.local = v }, fs, ctrl, tab),
			gui.Text(gui.TextCfg{
				Text:      i18n.T("tunnel.dynamic_description"),
				TextStyle: th.TextStyleSecondary,
				Sizing:    gui.FillFit,
			}),
		)
	}

	if fs.errMsg != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      fs.errMsg,
			TextStyle: th.TextStyleError,
			Sizing:    gui.FillFit,
		}))
	}
	content = append(content, gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		HAlign:  gui.HAlignEnd,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "tunnel-form-ok",
				Variant: gui.ButtonPrimary,
				Label:   i18n.T("tunnel.start"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					submit(ctx.Window, fs, ctrl, tab)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "tunnel-form-cancel",
				Variant: gui.ButtonSecondary,
				Label:   i18n.T("common.cancel"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctx.Window.DialogDismiss()
				},
			}),
		},
	})))
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: content,
	}))
}

func addrInput(id, label, placeholderKey, value string, set func(string), fs *formState, ctrl *app.Controller, tab *app.Tab) gui.View {
	return gui.Input(gui.InputCfg{
		ID:          id,
		Label:       label,
		Text:        value,
		Placeholder: i18n.T(placeholderKey),
		Sizing:      gui.FillFit,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			set(v)
			fs.errMsg = ""
			ctx.Consume()
		},
		OnEnter: func(ctx gui.EventCtx) {
			ctx.Consume()
			submit(ctx.Window, fs, ctrl, tab)
		},
	})
}

func portInput(fs *formState, ctrl *app.Controller, tab *app.Tab) gui.View {
	return gui.Input(gui.InputCfg{
		ID:          "tunnel-form-port",
		Label:       i18n.T("tunnel.remote_port"),
		Text:        fs.port,
		Placeholder: i18n.T("tunnel.example_remote_port"),
		Sizing:      gui.FillFit,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			fs.port = v
			fs.errMsg = ""
			ctx.Consume()
		},
		OnEnter: func(ctx gui.EventCtx) {
			ctx.Consume()
			submit(ctx.Window, fs, ctrl, tab)
		},
	})
}

func submit(w *gui.Window, fs *formState, ctrl *app.Controller, tab *app.Tab) {
	draft := app.TunnelDraft{
		Type:       fs.typ,
		LocalAddr:  fs.local,
		RemoteAddr: fs.remote,
		RemotePort: fs.port,
	}
	if msg := app.ValidateTunnelDraft(draft); msg != "" {
		fs.errMsg = msg
		return
	}
	w.DialogDismiss()
	ctrl.StartTunnel(w, tab.ID, draft)
}
