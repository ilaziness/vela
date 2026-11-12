package toolsui

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

func panelTimestamp(d *Deps, st *State) gui.View {
	ph := i18n.T("tools.timestamp.placeholder_to_dt")
	if st.TS.ToTimestamp {
		ph = i18n.T("tools.timestamp.placeholder_to_ts")
	}
	body := []gui.View{
		formCard(
			fieldInput("tools-ts-input", i18n.T("tools.timestamp.input_label"), st.TS.Input, ph, func(v string) {
				st.TS.Input = v
			}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Content: []gui.View{
					switchField("tools-ts-to-ts", i18n.T("tools.timestamp.to_timestamp"), st.TS.ToTimestamp, func() {
						st.TS.ToTimestamp = true
					}),
					switchField("tools-ts-to-dt", i18n.T("tools.timestamp.to_datetime"), !st.TS.ToTimestamp, func() {
						st.TS.ToTimestamp = false
					}),
				},
			})),
			primaryBtn("tools-ts-run", i18n.T("tools.shared.convert"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.TS.Input) == "" {
					toastErr(ctx.Window, i18n.T("tools.shared.input_required"))
					return
				}
				s.TS.Result = d.Core.ConvertTimestamp(s.TS.Input, s.TS.ToTimestamp)
				s.TS.Has = true
			}),
		),
	}
	if st.TS.Has {
		res := st.TS.Result
		rows := []gui.View{statusLine(res.Success, okOrErr(res.Success, res.Error))}
		if res.Success {
			var text string
			if st.TS.ToTimestamp {
				text = i18n.Tf("tools.timestamp.result_ts", strconv.FormatInt(res.Timestamp, 10))
			} else {
				text = i18n.Tf("tools.timestamp.result_dt", res.Datetime)
			}
			rows = append(rows,
				gui.Label(text, gui.CurrentTheme().TextStyleBody),
				secondaryBtn("tools-ts-copy", i18n.T("tools.shared.copy_result"), func(ctx gui.EventCtx) {
					ctx.Consume()
					s := gui.State[State](ctx.Window)
					out := s.TS.Result.Datetime
					if s.TS.ToTimestamp {
						out = strconv.FormatInt(s.TS.Result.Timestamp, 10)
					}
					copyText(ctx.Window, out)
				}),
			)
		}
		body = append(body, resultCard(rows...))
	}
	return panelShell(i18n.T("tools.timestamp.title"), i18n.T("tools.timestamp.desc"), body)
}

func panelCron(d *Deps, st *State) gui.View {
	body := []gui.View{
		formCard(
			fieldInput("tools-cron-expr", i18n.T("tools.cron.expression"), st.Cron.Expr,
				i18n.T("tools.cron.expression_placeholder"), func(v string) { st.Cron.Expr = v }),
			fieldInput("tools-cron-count", i18n.T("tools.cron.next_count"), st.Cron.Count, "1-20", func(v string) {
				st.Cron.Count = v
			}),
			primaryBtn("tools-cron-run", i18n.T("tools.shared.parse"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.Cron.Expr) == "" {
					toastErr(ctx.Window, i18n.T("tools.shared.input_required"))
					return
				}
				n, _ := strconv.Atoi(strings.TrimSpace(s.Cron.Count))
				if n < 1 {
					n = 5
				}
				s.Cron.Result = d.Core.ParseCron(s.Cron.Expr, n)
				s.Cron.Has = true
			}),
		),
	}
	if st.Cron.Has {
		res := st.Cron.Result
		if !res.Success {
			body = append(body, resultCard(statusLine(false, res.Error)))
		} else {
			rows := []gui.View{
				gui.Text(gui.TextCfg{
					Text:      i18n.T("tools.cron.next_times"),
					TextStyle: gui.CurrentTheme().TextStyleBody.Bold(),
				}),
				gui.Label(i18n.Tf("tools.cron.times_n", len(res.NextTimes)), gui.CurrentTheme().TextStyleSecondary),
			}
			for _, t := range res.NextTimes {
				rows = append(rows, gui.Label(t, gui.CurrentTheme().TextStyleBody))
			}
			body = append(body, resultCard(rows...))
		}
	}
	return panelShell(i18n.T("tools.cron.title"), i18n.T("tools.cron.desc"), body)
}

func panelRandom(d *Deps, st *State) gui.View {
	kindOpts := []gui.SelectOption{
		{Label: "UUID v4", Value: "uuid-v4"},
		{Label: "UUID v7", Value: "uuid-v7"},
		{Label: i18n.T("tools.random.kind_string"), Value: "string"},
	}
	charsetOpts := []gui.SelectOption{
		{Label: i18n.T("tools.random.charset_alphanum"), Value: "alphanum"},
		{Label: i18n.T("tools.random.charset_alpha"), Value: "alpha"},
		{Label: i18n.T("tools.random.charset_numeric"), Value: "numeric"},
		{Label: i18n.T("tools.random.charset_hex"), Value: "hex"},
		{Label: i18n.T("tools.random.charset_base64"), Value: "base64"},
		{Label: i18n.T("tools.random.charset_custom"), Value: "custom"},
	}
	fields := []gui.View{
		selectField("tools-rand-kind", i18n.T("tools.random.kind"), st.Random.Kind, kindOpts, func(v string) {
			st.Random.Kind = v
		}),
	}
	if st.Random.Kind == "string" {
		fields = append(fields,
			fieldInput("tools-rand-len", i18n.T("tools.random.length"), st.Random.Length, "1-1024", func(v string) {
				st.Random.Length = v
			}),
			selectField("tools-rand-cs", i18n.T("tools.random.charset"), st.Random.Charset, charsetOpts, func(v string) {
				st.Random.Charset = v
			}),
		)
		if st.Random.Charset == "custom" {
			fields = append(fields, fieldInput("tools-rand-custom", i18n.T("tools.random.custom_charset"),
				st.Random.CustomCharset, "", func(v string) { st.Random.CustomCharset = v }))
		}
	}
	fields = append(fields, primaryBtn("tools-rand-run", i18n.T("tools.shared.generate"), false, func(ctx gui.EventCtx) {
		ctx.Consume()
		s := gui.State[State](ctx.Window)
		n, _ := strconv.Atoi(strings.TrimSpace(s.Random.Length))
		if n < 1 {
			n = 16
		}
		s.Random.Result = d.Core.GenerateRandom(s.Random.Kind, n, s.Random.Charset, s.Random.CustomCharset)
		s.Random.Has = true
	}))

	body := []gui.View{formCard(fields...)}
	if st.Random.Has {
		res := st.Random.Result
		rows := []gui.View{statusLine(res.Success, okOrErr(res.Success, res.Error))}
		if res.Success {
			rows = append(rows,
				gui.Input(gui.InputCfg{
					ID:       "tools-rand-out",
					Label:    i18n.T("tools.shared.result"),
					Text:     res.Result,
					ReadOnly: true,
					Sizing:   gui.FillFit,
				}),
				secondaryBtn("tools-rand-copy", i18n.T("tools.shared.copy_result"), func(ctx gui.EventCtx) {
					ctx.Consume()
					copyText(ctx.Window, gui.State[State](ctx.Window).Random.Result.Result)
				}),
			)
		}
		body = append(body, resultCard(rows...))
	}
	return panelShell(i18n.T("tools.random.title"), i18n.T("tools.random.desc"), body)
}
