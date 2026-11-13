package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/bookmarks"
)

const sidePanelWidth float32 = 220

// SidePanel is the quick-connect bookmark tree, flush against the icon rail.
func SidePanel(w *gui.Window, ctrl *app.Controller) gui.View {
	st := gui.State[app.State](w)
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
					gui.Row(flush(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Spacing: gui.SpacingSmall,
						VAlign:  gui.VAlignMiddle,
						Content: []gui.View{
							gui.Button(gui.ButtonCfg{
								ID: "qc-refresh", Variant: gui.ButtonSecondary, Sizing: gui.FitFit,
								Disabled:  st.Busy,
								A11YLabel: i18n.T("common.refresh"),
								Content: []gui.View{
									gui.Text(gui.TextCfg{
										Text:      gui.IconSync,
										TextStyle: th.TextStyleIconSmall,
									}),
								},
								OnClick: func(ctx gui.EventCtx) {
									ctx.Consume()
									if gui.State[app.State](ctx.Window).Busy {
										return
									}
									ctrl.ReloadBookmarkGroups(ctx.Window)
								},
							}),
							gui.Input(gui.InputCfg{
								ID:          "qc-search",
								Placeholder: i18n.T("bookmark.search_connect_placeholder"),
								Text:        st.SearchQuery,
								Sizing:      gui.FillFit,
								OnTextChanged: func(v string, ctx gui.EventCtx) {
									gui.State[app.State](ctx.Window).SearchQuery = v
									ctx.Consume()
								},
							}),
						},
					})),
					bookmarks.ConnectTree(st.BookmarkGroups, st.SearchQuery, func(id string) {
						ctrl.ConnectBookmark(w, id)
					}),
				},
			})),
		},
	}))
}
