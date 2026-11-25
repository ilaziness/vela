// Package chrome draws the self-drawn window frame used when
// general.frameless is on. WindowCfg.Decorations drops the native
// frame at creation time, so the app supplies the drag strips, the
// min/max/close controls and a resize grip. Every piece is an
// immediate-mode view shared by the main window and the secondary
// windows; each window captures its frameless flag at creation and
// keeps it in its own State.
package chrome

import (
	"math"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Decoration maps the frameless setting to the WindowCfg.Decorations
// value. It is read once at window creation; the frame style cannot
// change afterwards, which is why toggling the setting asks for a
// restart.
func Decoration(frameless bool) gui.WindowDecoration {
	if frameless {
		return gui.DecorationNone
	}
	return gui.DecorationDefault
}

const (
	doubleClickInterval  = 500 * time.Millisecond
	doubleClickTolerance = 8.0
)

// ClickTracker detects the title-strip double click that toggles
// maximize. It lives in each window's State: package-level mutable
// state is banned, and the gesture is per window anyway.
type ClickTracker struct {
	lastTime time.Time
	lastX    float32
	lastY    float32
}

// DoubleClick reports whether (x, y) is a second left press within the
// double-click interval and tolerance of the previous one. A match
// resets the streak so a triple click cannot toggle twice.
func (t *ClickTracker) DoubleClick(x, y float32) bool {
	now := time.Now()
	hit := !t.lastTime.IsZero() &&
		now.Sub(t.lastTime) <= doubleClickInterval &&
		math.Abs(float64(x-t.lastX)) <= doubleClickTolerance &&
		math.Abs(float64(y-t.lastY)) <= doubleClickTolerance
	if hit {
		t.lastTime = time.Time{}
		return true
	}
	t.lastTime, t.lastX, t.lastY = now, x, y
	return false
}

// DragOrToggle is the OnMouseDown body for a drag strip: a double click
// toggles maximize, anything else starts the OS move loop (which keeps
// snapping and edge tiling working). Only the left button drags; other
// presses are left for whatever else wants them.
func DragOrToggle(ctx gui.EventCtx, tracker *ClickTracker) {
	e := ctx.Event
	if e == nil || e.MouseButton != gui.MouseLeft {
		return
	}
	if tracker != nil && tracker.DoubleClick(e.MouseX, e.MouseY) {
		if ctx.Window.IsMaximized() {
			ctx.Window.Restore()
		} else {
			ctx.Window.Maximize()
		}
	} else {
		ctx.Window.StartWindowDrag()
	}
	ctx.Consume()
}
