package chrome

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/layout"
)

// TitleBarHeight is the drag strip height of frameless secondary
// windows. The main window reuses the tab bar instead of this strip.
const TitleBarHeight float32 = 36

// ResizeGripSize is the bottom-right corner hot zone that starts the
// OS resize loop. Windows and X11 have no border to grab on a
// DecorationNone window; macOS resizes from any edge on its own.
const ResizeGripSize float32 = 16

// TitleBar is the drag strip + window controls for a frameless
// secondary window: title on the left, controls on the right, the
// whole strip draggable (empty areas bubble up to the row's
// OnMouseDown; interactive children consume their own presses).
// tracker is the window's per-State ClickTracker. Close routes
// through the window's OnCloseRequest hook.
func TitleBar(w *gui.Window, title string, tracker *ClickTracker) gui.View {
	th := gui.CurrentTheme()
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "chrome-titlebar",
		Sizing:  gui.FillFixed,
		Height:  TitleBarHeight,
		Color:   th.ColorPanel,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.NewPadding(0, 0, 0, gui.PadMedium),
		OnMouseDown: func(ctx gui.EventCtx) {
			DragOrToggle(ctx, tracker)
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: title, TextStyle: th.TextStyleBody}),
			gui.Rectangle(gui.RectangleCfg{Sizing: gui.FillFit}),
			Controls(w),
		},
	}))
}

// Frame wraps a secondary window's content with the frameless chrome:
// the drag strip on top and the resize grip floating bottom-right.
func Frame(w *gui.Window, title string, tracker *ClickTracker, content gui.View) gui.View {
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			TitleBar(w, title, tracker),
			content,
			ResizeGrip(),
		},
	}))
}

// ResizeGrip is a Float child to append to a window's root container:
// it anchors to the bottom-right corner and starts the OS resize loop
// for that corner on press. macOS ignores it (AppKit resizes a
// borderless window from any edge).
func ResizeGrip() gui.View {
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:      gui.FixedFixed,
		Width:       ResizeGripSize,
		Height:      ResizeGripSize,
		Float:       true,
		FloatAnchor: gui.FloatBottomRight,
		FloatTieOff: gui.FloatBottomRight,
		FloatZIndex: 20,
		// The corner has no visual affordance of its own; the
		// diagonal-arrow cursor is the whole hint, like a native
		// resize border.
		Hover: gui.HoverStyle{Cursor: gui.CursorResizeNWSE},
		OnMouseDown: func(ctx gui.EventCtx) {
			ctx.Window.StartWindowResize(gui.EdgeBottomRight)
			ctx.Consume()
		},
	}))
}
