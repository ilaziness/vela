package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/bookmarks"
)

const sidePanelWidth float32 = 260

// SidePanel is the quick-connect bookmark tree, flush against the icon rail.
func SidePanel(_ *app.Controller) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(flush(gui.ContainerCfg{
		ID:     "side-panel",
		Sizing: gui.FixedFill,
		Width:  sidePanelWidth,
		Color:  th.ColorPanel,
		Content: []gui.View{
			gui.Column(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Padding: gui.NewPadding(10, 12, 10, 12),
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Input(gui.InputCfg{
						ID:          "bookmark-search",
						Placeholder: i18n.T("bookmark.search_connect_placeholder"),
						Sizing:      gui.FillFit,
						OnTextChanged: func(v string, ctx gui.EventCtx) {
							gui.State[app.State](ctx.Window).SearchQuery = v
							ctx.Consume()
						},
					}),
					bookmarks.TreePlaceholder(),
				},
			})),
		},
	}))
}
