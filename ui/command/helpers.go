package commandui

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func toastOK(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastSuccess, Duration: 2 * time.Second})
}

func toastErr(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastError, Duration: 3 * time.Second})
}
