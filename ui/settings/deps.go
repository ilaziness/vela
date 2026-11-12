package settings

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/keybind"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/ui/app"
)

// Deps are process-wide services the settings window needs.
type Deps struct {
	App     *gui.App
	Store   *config.Store
	Ctrl    *app.Controller
	SSH     *ssh.Manager
	Vault   *secret.Vault
	Keys    *sshkey.Service
	Keybind *keybind.Manager
	IconPNG []byte
}
