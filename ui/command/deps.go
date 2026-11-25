package commandui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/termio"
)

// Deps are process-wide services the command palette window needs.
type Deps struct {
	App      *gui.App
	Commands *command.Service
	Hub      *termio.Hub
	Store    *config.Store
	IconPNG  []byte
}
