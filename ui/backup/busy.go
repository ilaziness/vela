package backup

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/layout"
)

func showBusy(w *gui.Window, message string) {
	layout.BusyDialog(w, message, nil)
}
