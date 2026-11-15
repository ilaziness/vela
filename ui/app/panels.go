package app

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/termio"
)

// PanelKind identifies the right-side panels that share the stage
// width next to the terminal. At most one is open per tab: the SFTP
// panel is not one of them because it stacks over the stage instead
// of squeezing it.
type PanelKind int

const (
	PanelPerf PanelKind = iota
	PanelTunnel
	PanelTransfer
)

// TogglePanel flips one side panel for a tab. Opening a panel closes
// whichever of the others was open, and closing perf stops its poller
// even when another panel is what closed it.
func (c *Controller) TogglePanel(w *gui.Window, tabID string, kind PanelKind) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.Kind != TabSession || tab.SessID == "" || tab.Protocol != termio.ProtocolSSH {
		return
	}
	closePerf := func() {
		if tab.PerfOpen {
			tab.PerfOpen = false
			c.StopPerf(tab.SessID)
		}
	}
	switch kind {
	case PanelPerf:
		if tab.PerfOpen {
			closePerf()
			return
		}
		tab.PerfOpen = true
		tab.TunnelOpen = false
		tab.TransferOpen = false
		c.StartPerf(w, tab.SessID)
	case PanelTunnel:
		if tab.TunnelOpen {
			tab.TunnelOpen = false
			return
		}
		tab.TunnelOpen = true
		closePerf()
		tab.TransferOpen = false
		if tab.BookmarkID != "" {
			c.refreshTunnelRules(w, tab.BookmarkID)
		}
	case PanelTransfer:
		if tab.TransferOpen {
			tab.TransferOpen = false
			return
		}
		tab.TransferOpen = true
		closePerf()
		tab.TunnelOpen = false
		c.refreshTransfers(w, tab.SessID)
	}
}
