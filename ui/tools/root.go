package toolsui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

// Root is the toolbox shell: left tool list + right panel.
func Root(w *gui.Window, d *Deps) gui.View {
	st := gui.State[State](w)
	st.ensureLoaded()
	th := gui.CurrentTheme()

	if lang := i18n.Language(); lang != st.Lang {
		st.Lang = lang
		w.SetTitle(i18n.T("window.tools"))
	}

	catalog := d.Core.GetTools()
	navBtns := make([]gui.View, 0, len(catalog))
	for _, tool := range catalog {
		id := tool.ID
		active := st.ActiveID == id
		bg := gui.Color{}
		if active {
			bg = th.ColorActive
		}
		glyph := iconGlyph(tool.Icon)
		navBtns = append(navBtns, gui.Button(gui.ButtonCfg{
			ID:      "tools-nav-" + id,
			Variant: gui.ButtonGhost,
			Sizing:  gui.FillFit,
			Color:   bg,
			HAlign:  gui.Some(gui.HAlignStart),
			Content: []gui.View{
				gui.Row(layout.Flush(gui.ContainerCfg{
					Sizing:  gui.FillFit,
					Spacing: gui.SpacingSmall,
					VAlign:  gui.VAlignMiddle,
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: glyph, TextStyle: th.TextStyleIconSmall}),
						gui.Text(gui.TextCfg{Text: tool.Name, TextStyle: th.TextStyleBody}),
					},
				})),
			},
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if s.ActiveID == id {
					return
				}
				if s.Busy {
					s.Busy = false
					s.RunGen++
				}
				s.ActiveID = id
			},
		}))
	}

	nav := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:         "tools-nav",
		Sizing:     gui.FixedFill,
		Width:      toolsNavWidth,
		Color:      th.ColorPanel,
		Padding:    gui.PaddingSmall,
		Spacing:    gui.SpacingTight,
		Scrollable: true,
		Content:    navBtns,
	}))

	var body gui.View
	switch st.ActiveID {
	case "timestamp":
		body = panelTimestamp(d, st)
	case "encoder":
		body = panelEncoder(d, st)
	case "hash":
		body = panelHash(d, st)
	case "regex":
		body = panelRegex(d, st)
	case "cron":
		body = panelCron(d, st)
	case "cidr":
		body = panelCIDR(d, st)
	case "port-check":
		body = panelPortCheck(d, st)
	case "jwt":
		body = panelJWT(d, st)
	case "random":
		body = panelRandom(d, st)
	case "base-convert":
		body = panelBaseConvert(d, st)
	case "text-diff":
		body = panelTextDiff(d, st)
	case "chmod":
		body = panelChmod(d, st)
	default:
		body = panelJSONYAML(d, st)
	}

	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   th.ColorBackground,
		Content: []gui.View{nav, layout.VRule(), body},
	}))
}
