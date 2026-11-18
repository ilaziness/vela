package layout

import "github.com/go-gui-org/go-gui/gui"

// Tooltip is the shared hover bubble: the go-gui Tooltip popup with the
// placement the flush chrome needs. WithTooltip is avoided because its
// wrapper Column adds theme padding; a hand-rolled Float column would
// duplicate the theme's tooltip colors, border and shadow. The float
// fields mirror the old hand-rolled bubbles, so position is unchanged;
// FloatAutoFlip and the window clamp come from gui.Tooltip itself.
func Tooltip(id, text string, anchor, tieOff gui.FloatAttach, offsetX, offsetY float32) gui.View {
	return gui.Tooltip(gui.TooltipCfg{
		ID:          id,
		Anchor:      gui.Some(anchor),
		TieOff:      gui.Some(tieOff),
		OffsetX:     gui.SomeF(offsetX),
		OffsetY:     gui.SomeF(offsetY),
		FloatZIndex: 10,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: text, Mode: gui.TextModeWrap}),
		},
	})
}

// TipText renders display text and floats tip on hover — the shared form
// of the Interactive+Text+Tooltip cell used by clipped labels (file
// names, transfer paths). The Interactive root must carry the same ID or
// the hover state never turns true (gui.Interactive contract).
func TipText(id, tip, display string, style gui.TextStyle) gui.View {
	return gui.Interactive(id, func(is gui.InteractionState) gui.View {
		content := []gui.View{
			gui.Text(gui.TextCfg{Text: display, TextStyle: style, Sizing: gui.FillFit, Clip: true}),
		}
		if is.Hovered && tip != "" {
			content = append(content, Tooltip(id+"-tip", tip,
				gui.FloatBottomCenter, gui.FloatTopCenter, 0, 4))
		}
		return gui.Row(Flush(gui.ContainerCfg{
			ID:      id,
			Sizing:  gui.FillFit,
			VAlign:  gui.VAlignMiddle,
			Clip:    true,
			Content: content,
		}))
	})
}

// TipPlacement picks where IconTipButton's hover bubble opens.
type TipPlacement uint8

const (
	// TipBelow centers the bubble under the button.
	TipBelow TipPlacement = iota
	// TipRight opens the bubble at the button's right edge (edge rails).
	TipRight
	// TipBelowLeft anchors the bubble's left edge under the button.
	TipBelowLeft
)

// IconTipButtonCfg configures a ghost icon button with a hover Tooltip.
// It is the shared form of the Interactive+Button+Tooltip pattern that
// chrome.control, session.toolBtn, sftp.tipBtn, shell.railBtn and the
// agent header each implemented locally.
type IconTipButtonCfg struct {
	ID    string
	Glyph string
	Tip   string
	// Th is the current theme; callers pass gui.CurrentTheme().
	Th      gui.Theme
	OnClick func(gui.EventCtx)

	// Variant picks the button hierarchy; pass gui.ButtonGhost for the
	// chrome-style icon buttons (zero value is ButtonSecondary).
	Variant  gui.ButtonVariant
	Active   bool // paints th.ColorActive behind the glyph (toggle state)
	Disabled bool

	// Size > 0 pins a square FixedFixed cell and stretches the button to
	// fill it; 0 hugs the button content.
	Size float32
	// Icon is the glyph style; zero takes th.TextStyleIconSmall.
	Icon    gui.TextStyle
	Padding gui.Padding // zero keeps the variant's theme padding
	Radius  gui.Radius  // zero keeps the variant's theme radius

	// Placement picks the bubble side; zero centers it under the button.
	Placement TipPlacement
}

// IconTipButton builds the configured icon button. The Interactive root
// carries cfg.ID so the hover state reaches the builder (gui.Interactive
// contract); the button and bubble take ID-suffixed leaves.
func IconTipButton(cfg IconTipButtonCfg) gui.View {
	th := cfg.Th
	cellID := cfg.ID
	icon := cfg.Icon
	if icon == (gui.TextStyle{}) {
		icon = th.TextStyleIconSmall
	}
	return gui.Interactive(cellID, func(is gui.InteractionState) gui.View {
		bg := gui.Color{}
		if cfg.Active {
			bg = th.ColorActive
		}
		var buttonSizing gui.Sizing
		wrapSizing := gui.FitFit
		var wrapW, wrapH float32
		if cfg.Size > 0 {
			buttonSizing = gui.FillFill
			wrapSizing = gui.FixedFixed
			wrapW, wrapH = cfg.Size, cfg.Size
		}
		content := []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            cellID + "-hit",
				Variant:       cfg.Variant,
				Sizing:        buttonSizing,
				Padding:       cfg.Padding,
				Color:         bg,
				Radius:        cfg.Radius,
				Disabled:      cfg.Disabled,
				FocusDisabled: true,
				HAlign:        gui.Some(gui.HAlignCenter),
				VAlign:        gui.Some(gui.VAlignMiddle),
				A11YCfg:       gui.A11YCfg{A11YLabel: cfg.Tip},
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: cfg.Glyph, TextStyle: icon}),
				},
				OnClick: cfg.OnClick,
			}),
		}
		if is.Hovered && cfg.Tip != "" {
			content = append(content, iconTip(cfg, cellID))
		}
		return gui.Column(Flush(gui.ContainerCfg{
			ID:      cellID,
			Sizing:  wrapSizing,
			Width:   wrapW,
			Height:  wrapH,
			Radius:  cfg.Radius,
			HAlign:  gui.HAlignCenter,
			VAlign:  gui.VAlignMiddle,
			Content: content,
		}))
	})
}

func iconTip(cfg IconTipButtonCfg, cellID string) gui.View {
	switch cfg.Placement {
	case TipRight:
		return Tooltip(cellID+"-tip", cfg.Tip,
			gui.FloatMiddleRight, gui.FloatMiddleLeft, 8, 0)
	case TipBelowLeft:
		return Tooltip(cellID+"-tip", cfg.Tip,
			gui.FloatBottomLeft, gui.FloatTopLeft, 0, 4)
	default:
		return Tooltip(cellID+"-tip", cfg.Tip,
			gui.FloatBottomCenter, gui.FloatTopCenter, 0, 4)
	}
}
