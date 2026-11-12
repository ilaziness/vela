package backup

import (
	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
)

func showConfirmUpload(w *gui.Window, d *Deps) {
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogConfirm,
		Title:      i18n.T("header.confirm_upload_title"),
		Body:       i18n.T("header.confirm_upload_message"),
		OnOkYes: func(win *gui.Window) {
			runUpload(win, d)
		},
		OnCancelNo: func(win *gui.Window) {
			showChoose(win, d)
		},
	})
}

func runUpload(w *gui.Window, d *Deps) {
	withVault(w, d, i18n.T("password.encrypt_decrypt_required"), func() {
		showBusy(w, i18n.T("header.uploading_backup"))
		go func() {
			err := uploadSync(d)
			w.QueueCommand(func(win *gui.Window) {
				win.DialogDismiss()
				if err != nil {
					d.Logger.Error("backup upload failed", zap.Error(err))
					toastErrMsg(win, i18n.Tf("header.upload_failed", errText(err)))
					return
				}
				toastOK(win, i18n.T("header.upload_success"))
			})
		}()
	})
}
