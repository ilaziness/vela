package shell

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	agentui "github.com/ilaziness/vela/ui/agent"
	"github.com/ilaziness/vela/ui/app"
	chromeui "github.com/ilaziness/vela/ui/chrome"
	"github.com/ilaziness/vela/ui/session"
	sftpui "github.com/ilaziness/vela/ui/sftp"
	transferui "github.com/ilaziness/vela/ui/transfer"
	tunnelui "github.com/ilaziness/vela/ui/tunnel"
)

// Root builds the edge-to-edge shell: icon rail | [bookmarks] | main.
func Root(w *gui.Window, ctrl *app.Controller, chrome Chrome) gui.View {
	st := gui.State[app.State](w)
	st.EnsureDefaults(ctrl.NewSSHDraft)
	th := gui.CurrentTheme()

	cols := []gui.View{IconRail(w, ctrl, chrome), vRule()}
	if st.SideVisible && st.ActiveNav == navQuickConnect {
		cols = append(cols, SidePanel(w, ctrl), vRule())
	}
	cols = append(cols, mainPane(w, ctrl, st))
	if st.Frameless {
		// Bottom-right corner resize for the borderless frame; the
		// float child overlays the layout, so it needs no column.
		cols = append(cols, chromeui.ResizeGrip())
	}

	return gui.Row(flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   th.ColorBackground,
		Content: cols,
	}))
}

func mainPane(w *gui.Window, ctrl *app.Controller, st *app.State) gui.View {
	th := gui.CurrentTheme()
	tab := st.Active()
	var body gui.View
	switch {
	case tab == nil:
		body = gui.Label(i18n.T("shell.tab.open_hint"), th.TextStyleSecondary)
	case tab.Kind == app.TabSession:
		body = session.TerminalPane(w, ctrl, tab)
	default:
		body = session.FormHost(w, ctrl, tab)
	}

	// [terminal] [perf panel?] [transfer panel?] | [tool rail?] — the rail belongs to a
	// live SSH session tab and appears only once it has connected;
	// every tool it offers (sftp, perf, tunnel, transfers) is
	// SSH-only, so serial sessions get no rail. The SFTP panel stacks
	// over the whole stage; the rail and tab bar stay visible and the
	// terminal keeps running underneath.
	if tab != nil && tab.Kind == app.TabSession && tab.SessID != "" && tab.Protocol == termio.ProtocolSSH {
		stage := body
		if tab.PerfOpen {
			stage = gui.Row(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Content: []gui.View{stage, session.PerfPanel(w, ctrl, tab)},
			}))
		}
		if tab.TunnelOpen {
			stage = gui.Row(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Content: []gui.View{stage, tunnelui.TunnelPanel(ctrl, tab)},
			}))
		}
		if tab.TransferOpen {
			stage = gui.Row(flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Content: []gui.View{stage, transferui.TransferPanel(ctrl, tab)},
			}))
		}
		if tab.SFTPOpen {
			stage = gui.Stack(flush(gui.ContainerCfg{
				Sizing: gui.FillFill,
				Content: []gui.View{
					stage,
					sftpui.View(w, ctrl, tab),
				},
			}))
		}
		body = gui.Row(flush(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Content: []gui.View{stage, vRule(), session.TabRail(ctrl, tab)},
		}))
	}

	main := gui.Column(flush(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Color:  th.ColorBackground,
		Content: []gui.View{
			session.TabBar(w, ctrl),
			body,
		},
	}))

	// The agent docks on the right edge and squeezes the whole stage
	// (tab bar + terminal + tool rails). Ratio is the terminal share;
	// clamped so the agent pane never exceeds 60% of the stage.
	ratio := st.AgentRatio
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.65
	}
	// The splitter is unconditional: it puts the stage under the
	// "agent-split"/"pane:first" scopes, and focus, blur tracking and scroll
	// offsets are all keyed by effective ID — adding it on open re-keyed the
	// whole stage. Closed keeps the tree and collapses the second pane.
	cfg := gui.SplitterCfg{
		ID:          "agent-split",
		Orientation: gui.SplitterHorizontal,
		Sizing:      gui.FillFill,
		First: gui.SplitterPaneCfg{
			Content: []gui.View{main},
			MinSize: 240,
		},
		Second: gui.SplitterPaneCfg{
			MinSize: 280,
		},
	}
	if st.AgentOpen {
		cfg.Ratio = gui.SomeF(ratio)
		cfg.Second.Content = []gui.View{agentui.Panel(w, ctrl)}
		cfg.OnChange = func(r float32, _ gui.SplitterCollapsed, ctx gui.EventCtx) {
			r = f32Clamp01(r)
			if r < 0.4 {
				r = 0.4
			}
			if r > 0.95 {
				r = 0.95
			}
			gui.State[app.State](ctx.Window).AgentRatio = r
		}
		return gui.Splitter(cfg)
	}
	// Closed: an empty second pane pinned to zero. Collapsible is what lets
	// the splitter honour the collapse; the handle cannot go below a pixel, so
	// it is painted out rather than left as a line at the window edge, and with
	// no divider to drag it takes no OnChange — a stray press there must not
	// rewrite the stored ratio.
	cfg.Collapsed = gui.SplitterCollapseSecond
	cfg.Second.Collapsible = true
	cfg.HandleSize = gui.SomeF(1)
	cfg.ColorHandle = gui.ColorTransparent
	cfg.ColorGrip = gui.ColorTransparent
	return gui.Splitter(cfg)
}

func f32Clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
