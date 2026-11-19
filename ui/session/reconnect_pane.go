package session

import (
	"fmt"

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
	return paneReconnectView(w, ctrl, tab, 0)
}

// paneReconnectView is the per-slot variant: it reads the pane's own
// status and error, and reconnects just that slot.
func paneReconnectView(w *gui.Window, ctrl *app.Controller, tab *app.Tab, idx int) gui.View {
	th := gui.CurrentTheme()
	st := gui.State[app.State](w)
	tab.EnsurePanes()
	status := tab.Status
	connErr := tab.ConnErr
	if idx >= 0 && idx < len(tab.Panes) {
		status = tab.Panes[idx].Status
		connErr = tab.Panes[idx].ConnErr
	}
	connecting := status == app.TabStatusConnecting
	tabID := tab.ID

	content := []gui.View{
		gui.Label(i18n.T("shell.session.disconnected"), th.TextStyleBody),
	}
	if connecting {
		content = append(content, gui.Label(i18n.T("shell.session.connecting"), th.TextStyleSecondary))
	}
	if connErr != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      connErr,
			TextStyle: th.TextStyleError,
			Mode:      gui.TextModeWrap,
		}))
	}
	btnID := fmt.Sprintf("pane-reconnect-%s-%d", tab.ID, idx)
	if idx == 0 {
		btnID = "tab-reconnect-" + tab.ID
	}
	content = append(content, gui.Button(gui.ButtonCfg{
		ID:       btnID,
		Variant:  gui.ButtonPrimary,
		Label:    i18n.T("shell.session.reconnect"),
		Sizing:   gui.FitFit,
		Disabled: connecting || st.Busy,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.ReconnectPane(ctx.Window, tabID, idx)
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
