package session

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/protocol"
)

const connectFormMaxWidth float32 = 480

// FormHost fills the main pane edge-to-edge; the form is centered
// horizontally and pinned to the top so SSH/serial swaps do not jump
// the protocol switcher (no floating card / rounded panel).
func FormHost(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	reg := ctrl.Registry()
	entries := reg.Entries()
	opts := make([]gui.SegmentOption, 0, len(entries))
	for _, e := range entries {
		opts = append(opts, gui.SegmentOption{
			Label:    e.Label,
			Value:    string(e.Protocol),
			Disabled: !e.Enabled,
		})
	}
	proto := string(tab.Protocol)
	if proto == "" {
		proto = string(termio.ProtocolSSH)
	}

	body := gui.Label(i18n.T("shell.protocol.unimplemented"), th.TextStyleSecondary)
	if e, ok := reg.Get(tab.Protocol); ok && e.Enabled && e.Form != nil {
		body = e.Form(ctrl, &tab.Draft)
	}

	form := gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FixedFit,
		Width:   connectFormMaxWidth,
		Spacing: gui.SpacingLarge,
		HAlign:  gui.HAlignStart,
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing: gui.FillFit,
				HAlign: gui.HAlignCenter,
				Content: []gui.View{
					gui.SegmentedControl(gui.SegmentedControlCfg{
						ID:      "connect-proto-" + tab.ID,
						Value:   proto,
						Options: opts,
						Sizing:  gui.FitFit,
						OnSelect: func(v string, ctx gui.EventCtx) {
							ctx.Consume()
							p := termio.Protocol(v)
							entry, ok := reg.Get(p)
							if !ok || !entry.Enabled {
								return
							}
							tab.Protocol = p
							if entry.NewDraft != nil {
								tab.Draft = entry.NewDraft()
							} else {
								tab.Draft = protocol.Draft{Protocol: p}
							}
						},
					}),
				},
			})),
			body,
		},
	}))

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignTop,
		Padding: gui.NewPadding(48, 0, 0, 0),
		Color:   th.ColorBackground,
		Content: []gui.View{form},
	}))
}
