package shell

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"

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

// IconRail is the far-left nav strip. One top group only:
// 快速连接 | 书签 | 备份恢复 | 工具箱 | 设置.
func IconRail(w *gui.Window) gui.View {
	th := gui.CurrentTheme()
	st := gui.State[app.State](w)
	quickOpen := st.ActiveNav == navQuickConnect && st.SideVisible

	placeholder := func(id, glyph, tip string) gui.View {
		return railBtn(id, glyph, tip, th, false, func(ctx gui.EventCtx) {
			ctx.Consume()
			ctx.Window.Toast(gui.ToastCfg{
				Body:     tip + "（占位）",
				Severity: gui.ToastInfo,
				Duration: 2 * time.Second,
			})
		})
	}

	// 快速连接：弹出书签列（替代原书签轨按钮）。
	quickBtn := railBtn(navQuickConnect, gui.IconBolt, "快速连接", th, quickOpen, func(ctx gui.EventCtx) {
		ctx.Consume()
		s := gui.State[app.State](ctx.Window)
		if s.ActiveNav == navQuickConnect && s.SideVisible {
			s.SideVisible = false
			return
		}
		s.ActiveNav = navQuickConnect
		s.SideVisible = true
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
					placeholder(navBookmarks, gui.IconBookmark, "书签"),
					placeholder(navBackup, gui.IconSync, "备份恢复"),
					placeholder(navTools, gui.IconWrench, "工具箱"),
					placeholder(navSettings, gui.IconGear, "设置"),
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
