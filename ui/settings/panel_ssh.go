package settings

import (
	"strconv"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/ssh"
)

func panelSSH(d *Deps, st *State) gui.View {
	c := &st.SSH
	proxyType := strings.TrimSpace(c.ProxyType)
	proxyOpts := []gui.SelectOption{
		{Label: i18n.T("settings.ssh.proxy_off"), Value: ""},
		{Label: "HTTP", Value: "http"},
		{Label: "SOCKS5", Value: "socks5"},
	}

	rows := []gui.View{
		formRow("settings-ssh-keepalive", i18n.T("settings.ssh.keepalive_sec"), strconv.Itoa(c.ServerAliveInterval),
			"", false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					c.ServerAliveInterval = n
				}
			}),
		formRow("settings-ssh-dial", i18n.T("settings.ssh.dial_timeout_sec"), strconv.Itoa(c.DialTimeoutSec),
			"", false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					c.DialTimeoutSec = n
				}
			}),
		switchRow("settings-ssh-reconnect", i18n.T("settings.ssh.auto_reconnect"), c.AutoReconnect, func(gui.EventCtx) {
			c.AutoReconnect = !c.AutoReconnect
		}),
		gui.Select(gui.SelectCfg{
			ID:       "settings-ssh-proxy-type",
			Label:    i18n.T("settings.ssh.proxy_type"),
			Selected: []string{proxyType},
			Options:  proxyOpts,
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).SSH.ProxyType = vals[0]
				}
			},
		}),
	}
	if proxyType != "" {
		rows = append(rows,
			formRow("settings-ssh-proxy-host", i18n.T("settings.ssh.proxy_host"), c.ProxyHost,
				i18n.T("settings.ssh.proxy_host_placeholder"), false, func(v string) { c.ProxyHost = v }),
			formRow("settings-ssh-proxy-port", i18n.T("settings.ssh.proxy_port"), strconv.Itoa(c.ProxyPort),
				"", false, func(v string) {
					if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
						c.ProxyPort = n
					}
				}),
			formRow("settings-ssh-proxy-user", i18n.T("settings.ssh.proxy_username"), c.ProxyUser,
				i18n.T("common.optional"), false, func(v string) { c.ProxyUser = v }),
			formRow("settings-ssh-proxy-pass", i18n.T("settings.ssh.proxy_password"), c.ProxyPassword,
				i18n.T("common.optional"), true, func(v string) { c.ProxyPassword = v }),
		)
	}
	rows = append(rows, saveBtn("settings-ssh-save", i18n.T("common.save"), st.Busy, func(ctx gui.EventCtx) {
		ctx.Consume()
		saveSSH(ctx.Window, d)
	}))
	return panelColumn(i18n.T("settings.ssh.title"), rows)
}

func saveSSH(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	cfg := st.SSH
	cfg.Normalize()
	if err := cfg.ValidateProxy(); err != nil {
		toastErr(w, err)
		return
	}
	st.Busy = true
	draft := cfg
	go func() {
		d.Store.Config.SSH = draft
		err := d.Store.Save()
		w.QueueCommand(func(win *gui.Window) {
			s := gui.State[State](win)
			s.Busy = false
			if err != nil {
				toastErr(win, err)
				return
			}
			s.SSH = draft
			d.SSH.SetOptions(ssh.Options{
				ServerAliveInterval: time.Duration(draft.ServerAliveInterval) * time.Second,
				DialTimeout:         time.Duration(draft.DialTimeoutSec) * time.Second,
			})
			toastOK(win, i18n.T("settings.ssh.save_success"))
		})
	}()
}
