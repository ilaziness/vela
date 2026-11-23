package commandui

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/usererr"
)

func toastOK(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastSuccess, Duration: 2 * time.Second})
}

func toastErr(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastError, Duration: 3 * time.Second})
}

// errText renders errors for toasts. usererr.Error() is a JSON RPC
// contract; the native UI shows the coded message in the current language.
func errText(err error) string {
	if err == nil {
		return ""
	}
	if ue, ok := usererr.As(err); ok {
		if ue.Code != "" {
			if len(ue.Params) == 0 {
				return i18n.T(ue.Code)
			}
			args := make([]any, len(ue.Params))
			for i, p := range ue.Params {
				args[i] = p
			}
			return i18n.Tf(ue.Code, args...)
		}
		return ue.Message
	}
	return err.Error()
}
