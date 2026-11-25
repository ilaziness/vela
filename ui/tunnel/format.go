package tunnelui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/tunnel"
	"github.com/ilaziness/vela/ui/app"
)

// typeIcon picks the Feather glyph for a tunnel type.
func typeIcon(typ string) string {
	switch typ {
	case tunnel.TypeLocal:
		return gui.IconArrowDown
	case tunnel.TypeRemote:
		return gui.IconArrowUp
	case tunnel.TypeDynamic:
		return gui.IconGlobe
	default:
		return gui.IconLink
	}
}

func typeLabel(typ string) string {
	return app.TunnelTypeLabel(typ)
}

// infoLine formats a running tunnel's addresses. Both local and remote
// tunnels read as "local side → remote side"; dynamic shows only the
// SOCKS listen address.
func infoLine(info tunnel.TunnelInfo) string {
	switch info.TunnelType {
	case tunnel.TypeDynamic:
		return info.LocalAddr
	default:
		return info.LocalAddr + " → " + info.RemoteAddr
	}
}
