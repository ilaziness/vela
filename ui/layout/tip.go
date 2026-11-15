package layout

import "github.com/go-gui-org/go-gui/gui"

// Tip is the shared hover bubble: the go-gui Tooltip popup with the
// placement the flush chrome needs. WithTooltip is avoided because its
// wrapper Column adds theme padding; a hand-rolled Float column would
// duplicate the theme's tooltip colors, border and shadow. The float
// fields mirror the old hand-rolled bubbles, so position is unchanged;
// FloatAutoFlip and the window clamp come from gui.Tooltip itself.
func Tip(id, text string, anchor, tieOff gui.FloatAttach, offsetX, offsetY float32) gui.View {
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
