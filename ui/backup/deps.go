package backup

import (
	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
)

// Deps are process-wide services the backup/restore dialogs need.
type Deps struct {
	Store  *config.Store
	Vault  *secret.Vault
	DB     *database.Database
	Logger *zap.Logger
}

// Start opens the backup/restore flow on the main window.
// Requires sync to be configured; otherwise shows an error toast.
func Start(w *gui.Window, d *Deps) {
	if !d.Store.Config.Sync.IsConfigured() {
		toastErrMsg(w, i18n.T("header.sync_config_required"))
		return
	}
	showChoose(w, d)
}
