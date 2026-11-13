package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
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
	navSettings     = "settings"
)

// IconRail is the far-left nav strip:
// quick-connect | bookmarks | backup | tools | settings.
func IconRail(w *gui.Window, chrome Chrome) gui.View {
	th := gui.CurrentTheme()
	st := gui.State[app.State](w)
	quickOpen := st.ActiveNav == navQuickConnect && st.SideVisible

	// Quick connect: toggle the bookmark side panel.
	quickBtn := railBtn(navQuickConnect, gui.IconBolt, i18n.T("shell.nav.quick_connect"), th, quickOpen, func(ctx gui.EventCtx) {
		ctx.Consume()
		s := gui.State[app.State](ctx.Window)
		if s.ActiveNav == navQuickConnect && s.SideVisible {
			s.SideVisible = false
			return
		}
		s.ActiveNav = navQuickConnect
		s.SideVisible = true
	})

	bookmarksBtn := railBtn(navBookmarks, gui.IconBookmark, i18n.T("header.bookmarks"), th, false, func(ctx gui.EventCtx) {
		ctx.Consume()
		if chrome.OnBookmarks != nil {
			chrome.OnBookmarks()
		}
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
					backupBtn,
					toolsBtn,
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
	cellID := "rail-" + id
	return gui.Interactive(cellID, func(is gui.InteractionState) gui.View {
		bg := gui.Color{}
		if active {
			bg = th.ColorActive
		}
		views := []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            cellID + "-hit",
				Variant:       gui.ButtonGhost,
				Sizing:        gui.FillFill,
				Padding:       gui.PaddingXSmall,
				Color:         bg,
				Radius:        gui.RadiusSmall,
				HAlign:        gui.Some(gui.HAlignCenter),
				VAlign:        gui.Some(gui.VAlignMiddle),
				FocusDisabled: true, // chrome: avoid focus ring when tabs close
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      glyph,
						TextStyle: th.TextStyleIconLarge,
					}),
				},
				OnClick: onClick,
			}),
		}
		if is.Hovered && tip != "" {
			views = append(views, railTip(tip, th))
		}
		return gui.Column(flush(gui.ContainerCfg{
			ID:      cellID,
			Sizing:  gui.FixedFixed,
			Width:   railBtnSize,
			Height:  railBtnSize,
			Radius:  gui.RadiusSmall,
			HAlign:  gui.HAlignCenter,
			VAlign:  gui.VAlignMiddle,
			Content: views,
		}))
	})
}

// railTip floats to the right. gui.Tooltip's TieOff is unexported in
// go-gui v0.92, so side-anchored tips stay a plain Float column.
func railTip(text string, th gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   gui.FloatMiddleRight,
		FloatTieOff:   gui.FloatMiddleLeft,
		FloatOffsetX:  8,
		FloatZIndex:   10,
		Color:         th.ColorInterior,
		ColorBorder:   th.ColorBorder,
		SizeBorder:    gui.BorderThin,
		Radius:        gui.RadiusSmall,
		Padding:       gui.PaddingSmall,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      text,
				TextStyle: th.TextStyleBody,
			}),
		},
	})
}
