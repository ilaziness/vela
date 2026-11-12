package toolsui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/tools"
)

// Deps are process-wide services the toolbox window needs.
type Deps struct {
	App     *gui.App
	Core    *tools.Service
	IconPNG []byte
}
