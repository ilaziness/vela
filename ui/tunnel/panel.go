package tunnelui

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/tunnel"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const tunnelPanelWidth float32 = 360

// TunnelPanel is the tunnel management column shown between the
// terminal/perf area and the tool rail: saved bookmark rules (when the
// session came from a bookmark) plus the session's running tunnels.
func TunnelPanel(ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	sessID := tab.SessID
	running := ctrl.TunnelSnapshot(sessID)

	header := gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "tunnel-header-" + tab.ID,
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 6, 0, 2),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      i18n.T("tunnel.manage_title"),
				TextStyle: th.TextStyleLabel,
				Sizing:    gui.FillFit,
			}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("%d", len(running)),
				TextStyle: th.TextStyleSecondary,
				Sizing:    gui.FitFit,
			}),
			gui.Button(gui.ButtonCfg{
				ID:            "tunnel-create-" + tab.ID,
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconPlus, TextStyle: th.TextStyleIconSmall}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					OpenForm(ctx.Window, ctrl, tab)
				},
			}),
		},
	}))

	var content []gui.View
	if tab.BookmarkID != "" {
		content = append(content, sectionLabel(th, i18n.T("tunnel.saved_rules")))
		rules, loaded, errMsg := ctrl.TunnelRules(tab.BookmarkID)
		switch {
		case !loaded:
			content = append(content, hintText(th, i18n.T("common.loading")))
		case errMsg != "":
			content = append(content, gui.Text(gui.TextCfg{
				Text:      errMsg,
				TextStyle: th.TextStyleError,
				Sizing:    gui.FillFit,
			}))
		case len(rules) == 0:
			content = append(content, hintText(th, i18n.T("tunnel.rules_empty")))
		default:
			for i, rule := range rules {
				content = append(content, ruleRow(ctrl, tab, rule, sessID))
				if i < len(rules)-1 {
					content = append(content, hDivider(th))
				}
			}
		}
	}
	content = append(content, sectionLabel(th, i18n.T("tunnel.running")))

	if len(running) == 0 {
		content = append(content, hintText(th, i18n.T("tunnel.no_tunnels")))
	} else {
		for i, info := range running {
			content = append(content, runningRow(ctrl, tab, info))
			if i < len(running)-1 {
				content = append(content, hDivider(th))
			}
		}
	}

	body := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:         "tunnel-list",
		Sizing:     gui.FillFill,
		Scrollable: true,
		ScrollMode: gui.ScrollVerticalOnly,
		// No ClipContents: its stencil masks the overlaid scrollbar too,
		// so the list scrolled with no bar. Clip still scissors the rows.
		Clip:    true,
		Spacing: gui.SpacingTight,
		// Right gutter keeps rows clear of the overlay scrollbar.
		Padding: gui.NewPadding(0, 12, 0, 0),
		Content: content,
	}))

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "tunnel-panel-" + tab.ID,
		Sizing:  gui.FixedFill,
		Width:   tunnelPanelWidth,
		Color:   th.ColorPanel,
		Spacing: gui.NoSpacing,
		Content: []gui.View{header, body},
	}))
}

// hDivider is the 1px horizontal separator between rows.
func hDivider(th gui.Theme) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFixed,
		Height: 1,
		Color:  th.ColorInterior,
	}))
}

func sectionLabel(th gui.Theme, text string) gui.View {
	return gui.Text(gui.TextCfg{
		Text:      text,
		TextStyle: th.TextStyleSecondary,
		Sizing:    gui.FillFit,
	})
}

func hintText(th gui.Theme, text string) gui.View {
	return gui.Label(text, th.TextStyleSecondary)
}

// ruleRow is one saved rule: type + summary + start/delete actions.
func ruleRow(ctrl *app.Controller, tab *app.Tab, rule app.TunnelRule, sessID string) gui.View {
	th := gui.CurrentTheme()
	active := ctrl.TunnelRuleActive(sessID, rule)
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "tunnel-rule-" + rule.ID,
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: typeIcon(rule.Type), TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing: gui.FillFit,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: typeLabel(rule.Type), TextStyle: th.TextStyleBody, Sizing: gui.FillFit}),
					gui.Text(gui.TextCfg{Text: rule.Summary(), TextStyle: th.TextStyleSecondary, Sizing: gui.FillFit}),
				},
			})),
			gui.Button(gui.ButtonCfg{
				ID:            "tunnel-rule-start-" + rule.ID,
				Label:         i18n.T("tunnel.start"),
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				Disabled:      active,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctrl.StartTunnelRule(ctx.Window, tab.ID, rule)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:            "tunnel-rule-del-" + rule.ID,
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconTrash, TextStyle: th.TextStyleIconSmall}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctrl.DeleteTunnelRule(ctx.Window, tab.ID, rule)
				},
			}),
		},
	}))
}

// runningRow is one live tunnel with a stop action.
func runningRow(ctrl *app.Controller, tab *app.Tab, info tunnel.TunnelInfo) gui.View {
	th := gui.CurrentTheme()
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "tunnel-run-" + info.ID,
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: typeIcon(info.TunnelType), TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing: gui.FillFit,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: typeLabel(info.TunnelType), TextStyle: th.TextStyleBody, Sizing: gui.FillFit}),
					gui.Text(gui.TextCfg{Text: infoLine(info), TextStyle: th.TextStyleSecondary, Sizing: gui.FillFit}),
				},
			})),
			gui.Button(gui.ButtonCfg{
				ID:            "tunnel-stop-" + info.ID,
				Label:         i18n.T("tunnel.stop"),
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctrl.StopTunnel(ctx.Window, info.ID)
				},
			}),
		},
	}))
}
