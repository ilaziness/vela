package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// Rail geometry. Cells nearly fill the strip (1px inset each side).
//
// Do not wrap cells in gui.WithTooltip: its Column takes the theme
// container PaddingMedium (14px L/R), so a 38px button becomes a 66px
// cell that overflows the rail and looks right-shifted / unfilled.
const (
	railWidth   float32 = 30
	railInset   float32 = 1
	railBtnSize float32 = railWidth - railInset*2
)

// Nav IDs for ActiveNav / side panel wiring.
const (
	navQuickConnect = "quick-connect"
	navBookmarks    = "bookmarks"
	navBackup       = "backup"
	navTools        = "tools"
	navCommand      = "command"
	navSettings     = "settings"
)

// IconRail is the far-left nav strip:
// quick-connect | bookmarks | backup | tools | settings.
func IconRail(w *gui.Window, ctrl *app.Controller, chrome Chrome) gui.View {
	th := gui.CurrentTheme()
	st := gui.State[app.State](w)
	quickOpen := st.ActiveNav == navQuickConnect && st.SideVisible

	// Quick connect: toggle the bookmark side panel.
	quickBtn := railBtn(navQuickConnect, gui.IconBolt, i18n.T("shell.nav.quick_connect"), th, quickOpen, func(ctx gui.EventCtx) {
		ctx.Consume()
		ToggleQuickConnect(ctx.Window, ctrl)
	})

	bookmarksBtn := railBtn(navBookmarks, gui.IconBookmark, i18n.T("header.bookmarks"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		if chrome.OnBookmarks != nil {
			chrome.OnBookmarks()
		}
	})

	agentBtn := railViewBtn("agent",
		layout.RobotIcon(layout.RobotIconCfg{
			Size:  th.TextStyleIconLarge.Size,
			Color: th.TextStyleIconLarge.Color,
		}),
		i18n.T("ai.chat.user_assistant"), th, st.AgentOpen, func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.AgentToggle(ctx.Window)
		})

	backupBtn := railBtn(navBackup, gui.IconSync, i18n.T("header.backup_restore"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		if chrome.OnBackup != nil {
			chrome.OnBackup()
		}
	})

	toolsBtn := railBtn(navTools, gui.IconWrench, i18n.T("header.tools"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		if chrome.OnTools != nil {
			chrome.OnTools()
		}
	})

	commandBtn := railBtn(navCommand, gui.IconTerminal, i18n.T("header.command_palette"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		if chrome.OnCommand != nil {
			chrome.OnCommand()
		}
	})

	settingsBtn := railBtn(navSettings, gui.IconGear, i18n.T("header.settings"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		if chrome.OnSettings != nil {
			chrome.OnSettings()
		}
	})

	return gui.Column(flush(gui.ContainerCfg{
		ID:      "icon-rail",
		Sizing:  gui.FixedFill,
		Width:   railWidth,
		Color:   th.ColorPanel,
		HAlign:  gui.HAlignCenter,
		Padding: gui.NewPadding(0, railInset, 0, railInset),
		Content: []gui.View{
			gui.Column(flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingTight,
				Padding: gui.NewPadding(4, 0, 4, 0),
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					quickBtn,
					bookmarksBtn,
					agentBtn,
					backupBtn,
					toolsBtn,
					commandBtn,
					settingsBtn,
				},
			})),
		},
	}))
}

func railBtn(
	id, glyph, tip string,
	th gui.Theme,
	active bool,
	onClick func(gui.EventCtx),
) gui.View {
	return railViewBtn(id,
		gui.Text(gui.TextCfg{Text: glyph, TextStyle: th.TextStyleIconLarge}),
		tip, th, active, onClick)
}

// railViewBtn is railBtn for an icon the glyph face cannot express.
func railViewBtn(
	id string,
	icon gui.View,
	tip string,
	th gui.Theme,
	active bool,
	onClick func(gui.EventCtx),
) gui.View {
	return layout.IconTipButton(layout.IconTipButtonCfg{
		ID:        "rail-" + id,
		IconView:  icon,
		Tip:       tip,
		Th:        th,
		OnClick:   onClick,
		Variant:   gui.ButtonGhost,
		Active:    active,
		Size:      railBtnSize,
		Padding:   gui.PaddingXSmall,
		Radius:    gui.RadiusSmall,
		Placement: layout.TipRight,
	})
}
