//go:build !windows

package chrome

import "github.com/go-gui-org/go-gui/gui"

// CenterOnScreen is a no-op off Windows: the X11 and macOS window
// managers place borderless windows acceptably on their own, and
// neither exposes a screen-size API through go-gui.
func CenterOnScreen(w *gui.Window) {}
