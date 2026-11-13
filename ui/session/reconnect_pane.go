package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// reconnectPaneWidth bounds the centered message block so a long error
// wraps instead of stretching the pane.
const reconnectPaneWidth float32 = 420

// ReconnectPane replaces the terminal of a dropped session. The tab stays
// open so the user can dial the same target again without retyping the form.
func ReconnectPane(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	st := gui.State[app.State](w)
	connecting := tab.Status == app.TabStatusConnecting

	content := []gui.View{
		gui.Label(i18n.T("shell.session.disconnected"), th.TextStyleBody),
	}
	if connecting {
		content = append(content, gui.Label(i18n.T("shell.session.connecting"), th.TextStyleSecondary))
	}
	if tab.ConnErr != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      tab.ConnErr,
			TextStyle: th.TextStyleError,
			Mode:      gui.TextModeWrap,
		}))
	}
	content = append(content, gui.Button(gui.ButtonCfg{
		ID:       "tab-reconnect-" + tab.ID,
		Variant:  gui.ButtonPrimary,
		Label:    i18n.T("shell.session.reconnect"),
		Sizing:   gui.FitFit,
		Disabled: connecting || st.Busy,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.ReconnectTab(ctx.Window, tab.ID)
		},
	}))

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Color:  th.ColorBackground,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FixedFit,
				Width:   reconnectPaneWidth,
				HAlign:  gui.HAlignCenter,
				Spacing: gui.SpacingMedium,
				Content: content,
			})),
		},
	}))
}
