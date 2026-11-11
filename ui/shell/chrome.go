package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/layout"
)

func flush(cfg gui.ContainerCfg) gui.ContainerCfg { return layout.Flush(cfg) }

func vRule() gui.View { return layout.VRule() }
