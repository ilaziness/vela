package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/app"
)

// ToggleQuickConnect flips the quick-connect side panel — the same
// gesture the icon rail's bolt button performs. Shared by the rail
// button and the panel.quickConnect global shortcut.
func ToggleQuickConnect(w *gui.Window, ctrl *app.Controller) {
	st := gui.State[app.State](w)
	if st.ActiveNav == navQuickConnect && st.SideVisible {
		st.SideVisible = false
		return
	}
	st.ActiveNav = navQuickConnect
	st.SideVisible = true
	ctrl.ReloadBookmarkGroups(w)
}
