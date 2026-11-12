package toolsui

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

func panelCIDR(d *Deps, st *State) gui.View {
	body := []gui.View{
		formCard(
			fieldInput("tools-cidr", "CIDR", st.CIDR.CIDR, i18n.T("tools.cidr.cidr_placeholder"), func(v string) {
				st.CIDR.CIDR = v
			}),
			fieldInput("tools-cidr-ip", i18n.T("tools.cidr.check_ip"), st.CIDR.CheckIP,
				i18n.T("tools.cidr.check_ip_placeholder"), func(v string) { st.CIDR.CheckIP = v }),
			primaryBtn("tools-cidr-run", i18n.T("tools.shared.calculate"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.CIDR.CIDR) == "" {
					toastErr(ctx.Window, i18n.T("tools.cidr.cidr_required"))
					return
				}
				s.CIDR.Result = d.Core.CalculateCIDR(s.CIDR.CIDR, s.CIDR.CheckIP)
				s.CIDR.Has = true
			}),
		),
	}
	if st.CIDR.Has {
		res := st.CIDR.Result
		if !res.Success {
			body = append(body, resultCard(statusLine(false, res.Error)))
		} else {
			th := gui.CurrentTheme()
			rows := []gui.View{
				gui.Label(i18n.Tf("tools.cidr.network", res.Network), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.cidr.netmask", res.Netmask, strconv.Itoa(res.PrefixLen)), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.cidr.broadcast", res.Broadcast), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.cidr.first_host", res.FirstHost), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.cidr.last_host", res.LastHost), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.cidr.host_count", res.HostCount), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.cidr.total_addrs", res.TotalAddrs), th.TextStyleBody),
			}
			if res.Error != "" {
				rows = append(rows, warnLine(res.Error))
			}
			if res.Contains != nil && res.ContainsIP != "" {
				if *res.Contains {
					rows = append(rows, statusLine(true, i18n.Tf("tools.cidr.contains_yes", res.ContainsIP)))
				} else {
					rows = append(rows, warnLine(i18n.Tf("tools.cidr.contains_no", res.ContainsIP)))
				}
			}
			body = append(body, resultCard(rows...))
		}
	}
	return panelShell(i18n.T("tools.cidr.title"), i18n.T("tools.cidr.desc"), body)
}

func panelPortCheck(d *Deps, st *State) gui.View {
	label := i18n.T("tools.shared.check")
	if st.Busy {
		label = i18n.T("tools.shared.checking")
	}
	body := []gui.View{
		formCard(
			fieldInput("tools-port-host", i18n.T("tools.port_check.host"), st.Port.Host,
				i18n.T("tools.port_check.host_placeholder"), func(v string) { st.Port.Host = v }),
			fieldInput("tools-port-port", i18n.T("tools.port_check.port"), st.Port.Port, "1-65535", func(v string) {
				st.Port.Port = v
			}),
			primaryBtn("tools-port-run", label, st.Busy, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if s.Busy {
					return
				}
				host := strings.TrimSpace(s.Port.Host)
				if host == "" {
					toastErr(ctx.Window, i18n.T("tools.port_check.host_required"))
					return
				}
				port, err := strconv.Atoi(strings.TrimSpace(s.Port.Port))
				if err != nil || port < 1 || port > 65535 {
					toastErr(ctx.Window, i18n.T("tools.port_check.port_range"))
					return
				}
				s.Busy = true
				s.RunGen++
				gen := s.RunGen
				win := ctx.Window
				go func() {
					res := d.Core.CheckPort(host, port)
					win.QueueCommand(func(w *gui.Window) {
						st2 := gui.State[State](w)
						if st2.RunGen != gen {
							return
						}
						st2.Busy = false
						if st2.ActiveID != "port-check" {
							return
						}
						st2.Port.Result = res
						st2.Port.Has = true
					})
				}()
			}),
		),
	}
	if st.Port.Has {
		res := st.Port.Result
		th := gui.CurrentTheme()
		if res.Success {
			body = append(body, resultCard(
				statusLine(true, i18n.T("tools.port_check.success")),
				gui.Label(i18n.Tf("tools.port_check.chip_host", res.Host), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.port_check.chip_port", res.Port), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.port_check.chip_latency", res.ResponseTime), th.TextStyleBody),
			))
		} else {
			body = append(body, resultCard(
				statusLine(false, i18n.Tf("tools.port_check.failed", res.Error)),
				gui.Label(i18n.Tf("tools.port_check.chip_host", res.Host), th.TextStyleBody),
				gui.Label(i18n.Tf("tools.port_check.chip_port", res.Port), th.TextStyleBody),
			))
		}
	}
	return panelShell(i18n.T("tools.port_check.title"), i18n.T("tools.port_check.desc"), body)
}
