package settings

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/agent"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/keybind"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/update"
)

// Deps are process-wide services the settings window needs.
type Deps struct {
	App     *gui.App
	Store   *config.Store
	Ctrl    *app.Controller
	Agent   *agent.Service
	SSH     *ssh.Manager
	Vault   *secret.Vault
	Keys    *sshkey.Service
	Keybind *keybind.Manager
	DB      *database.Database
	IconPNG []byte
	Update  *update.Deps
	// OnLanguageChanged fires after a language save so services holding
	// localized builtin data (command palette) can reload.
	OnLanguageChanged func(lang string)
}
