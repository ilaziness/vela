package settings

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

var navItems = []struct{ id, key string }{
	{navGeneral, "settings.nav.general"},
	{navTerminal, "settings.nav.terminal"},
	{navSSH, "settings.nav.ssh"},
	{navSync, "settings.nav.sync"},
	{navAI, "settings.nav.ai"},
	{navKeys, "settings.nav.keys"},
	{navKnownHosts, "settings.nav.known_hosts"},
	{navKeybindings, "settings.nav.keybindings"},
	{navAbout, "settings.nav.about"},
}

// Root is the settings shell: left nav + right panel.
func Root(w *gui.Window, d *Deps) gui.View {
	st := gui.State[State](w)
	st.ensureLoaded(d)
	th := gui.CurrentTheme()

	var body gui.View
	switch st.ActiveNav {
	case navTerminal:
		body = panelTerminal(d, st)
	case navSSH:
		body = panelSSH(d, st)
	case navSync:
		body = panelSync(d, st)
	case navAI:
		body = panelAI(d, st)
	case navKeys:
		body = panelKeys(w, d, st)
	case navKnownHosts:
		body = panelKnownHosts(w, d, st)
	case navKeybindings:
		body = panelKeybindings(w, d, st)
	case navAbout:
		body = panelAbout()
	default:
		body = panelGeneral(d, st)
	}

	navBtns := make([]gui.View, 0, len(navItems))
	for _, it := range navItems {
		id, labelKey := it.id, it.key
		active := st.ActiveNav == id
		bg := gui.Color{}
		if active {
			bg = th.ColorActive
		}
		navBtns = append(navBtns, gui.Button(gui.ButtonCfg{
			ID:      "settings-nav-" + id,
			Variant: gui.ButtonGhost,
			Sizing:  gui.FillFit,
			Color:   bg,
			Label:   i18n.T(labelKey),
			HAlign:  gui.Some(gui.HAlignStart),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				s.ActiveNav = id
				if id == navKeys || id == navKnownHosts || id == navKeybindings || id == navAI {
					refreshLists(s, d)
				}
			},
		}))
	}

	nav := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "settings-nav",
		Sizing:  gui.FixedFill,
		Width:   settingsNavWidth,
		Color:   th.ColorPanel,
		Padding: gui.PaddingSmall,
		Spacing: gui.SpacingTight,
		Content: navBtns,
	}))

	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   th.ColorBackground,
		Content: []gui.View{nav, layout.VRule(), body},
	}))
}
