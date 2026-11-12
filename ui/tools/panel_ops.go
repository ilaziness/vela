package toolsui

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

func panelChmod(d *Deps, st *State) gui.View {
	dirOpts := []gui.SelectOption{
		{Label: i18n.T("tools.chmod.to_symbolic"), Value: "toSymbolic"},
		{Label: i18n.T("tools.chmod.to_octal"), Value: "toOctal"},
	}
	ph := i18n.T("tools.chmod.input_placeholder_octal")
	if st.Chmod.Direction == "toOctal" {
		ph = i18n.T("tools.chmod.input_placeholder_symbolic")
	}
	body := []gui.View{
		formCard(
			selectField("tools-chmod-dir", i18n.T("tools.chmod.direction"), st.Chmod.Direction, dirOpts, func(v string) {
				st.Chmod.Direction = v
			}),
			fieldInput("tools-chmod-in", i18n.T("tools.shared.input"), st.Chmod.Input, ph, func(v string) {
				st.Chmod.Input = v
			}),
			primaryBtn("tools-chmod-run", i18n.T("tools.shared.convert"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.Chmod.Input) == "" {
					toastErr(ctx.Window, i18n.T("tools.chmod.permission_required"))
					return
				}
				s.Chmod.Result = d.Core.ConvertChmod(s.Chmod.Input, s.Chmod.Direction)
				s.Chmod.Has = true
			}),
		),
	}
	if st.Chmod.Has {
		res := st.Chmod.Result
		if !res.Success {
			body = append(body, resultCard(statusLine(false, res.Error)))
		} else {
			th := gui.CurrentTheme()
			rows := []gui.View{statusLine(true, i18n.T("tools.shared.success"))}
			if res.Octal != "" {
				rows = append(rows, gui.Label(i18n.Tf("tools.chmod.octal", res.Octal), th.TextStyleBody))
			}
			if res.Symbolic != "" {
				rows = append(rows, gui.Label(i18n.Tf("tools.chmod.symbolic", res.Symbolic), th.TextStyleBody))
			}
			rows = append(rows, secondaryBtn("tools-chmod-copy", i18n.T("tools.shared.copy_result"), func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				out := s.Chmod.Result.Symbolic
				if s.Chmod.Direction == "toOctal" {
					out = s.Chmod.Result.Octal
				}
				copyText(ctx.Window, out)
			}))
			body = append(body, resultCard(rows...))
		}
	}
	return panelShell(i18n.T("tools.chmod.title"), i18n.T("tools.chmod.desc"), body)
}
