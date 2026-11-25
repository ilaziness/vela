package layout

import "github.com/go-gui-org/go-gui/gui"

// RobotIconCfg configures the hand-drawn robot glyph.
type RobotIconCfg struct {
	// Size is the square side in logical px.
	Size float32
	// Color paints the outline and the filled details.
	Color gui.Color
}

// RobotIcon draws a minimal line-art robot. The bundled feathericon
// face has no robot glyph, so the AI assistant mark is drawn from
// primitives; go-gui itself stays unmodified. No ID is set, so the
// canvas is not cached and a theme change repaints the current color.
func RobotIcon(cfg RobotIconCfg) gui.View {
	size := cfg.Size
	if size <= 0 {
		size = 16
	}
	return gui.DrawCanvas(gui.DrawCanvasCfg{
		Sizing: gui.FixedFixed,
		Width:  size,
		Height: size,
		OnDraw: func(dc *gui.DrawContext) {
			drawRobot(dc, cfg.Color)
		},
	})
}

// drawRobot lays the robot out proportionally, so it scales with the
// themed icon size: antenna on top, a round-cornered head, two eyes
// and a mouth.
func drawRobot(dc *gui.DrawContext, color gui.Color) {
	w, h := dc.Width, dc.Height
	side := min(w, h)
	if side <= 0 {
		return
	}
	stroke := side * 0.075
	if stroke < 1 {
		stroke = 1
	}
	cx := w / 2
	pad := side * 0.08
	headW := side - pad*2
	headH := side * 0.56
	headY := h - pad - headH

	// Antenna: a short stem with a ball at the top.
	tipR := side * 0.05
	dc.FilledCircle(cx, tipR, tipR, color)
	dc.Line(cx, tipR*2, cx, headY, color, stroke)

	// Head.
	dc.RoundedRect(pad, headY, headW, headH, headW*0.24, color, stroke)

	// Eyes.
	eyeR := side * 0.075
	eyeY := headY + headH*0.42
	dc.FilledCircle(pad+headW*0.3, eyeY, eyeR, color)
	dc.FilledCircle(pad+headW*0.7, eyeY, eyeR, color)

	// Mouth.
	mouthY := headY + headH*0.76
	dc.Line(pad+headW*0.3, mouthY, pad+headW*0.7, mouthY, color, stroke)
}
