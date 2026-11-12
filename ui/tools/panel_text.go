package toolsui

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

func panelRegex(d *Deps, st *State) gui.View {
	body := []gui.View{
		formCard(
			fieldInput("tools-regex-pat", i18n.T("tools.regex.pattern"), st.Regex.Pattern,
				i18n.T("tools.regex.pattern_placeholder"), func(v string) { st.Regex.Pattern = v }),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Content: []gui.View{
					switchField("tools-regex-i", i18n.T("tools.regex.flag_i"), st.Regex.FlagI, func() {
						st.Regex.FlagI = !st.Regex.FlagI
					}),
					switchField("tools-regex-m", i18n.T("tools.regex.flag_m"), st.Regex.FlagM, func() {
						st.Regex.FlagM = !st.Regex.FlagM
					}),
					switchField("tools-regex-g", i18n.T("tools.regex.flag_g"), st.Regex.FlagG, func() {
						st.Regex.FlagG = !st.Regex.FlagG
					}),
				},
			})),
			multiInput("tools-regex-text", i18n.T("tools.regex.test_text"), st.Regex.Text,
				i18n.T("tools.regex.test_placeholder"), multiMinH, func(v string) { st.Regex.Text = v }),
			primaryBtn("tools-regex-run", i18n.T("tools.shared.run"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.Regex.Pattern) == "" || strings.TrimSpace(s.Regex.Text) == "" {
					toastErr(ctx.Window, i18n.T("tools.shared.input_required"))
					return
				}
				flags := ""
				if s.Regex.FlagI {
					flags += "i"
				}
				if s.Regex.FlagM {
					flags += "m"
				}
				if s.Regex.FlagG {
					flags += "g"
				}
				s.Regex.Result = d.Core.RegexMatch(s.Regex.Pattern, s.Regex.Text, flags)
				s.Regex.Has = true
			}),
		),
	}
	if st.Regex.Has {
		res := st.Regex.Result
		if res.Error != "" {
			body = append(body, resultCard(statusLine(false, res.Error)))
		} else {
			rows := []gui.View{
				gui.Text(gui.TextCfg{
					Text:      i18n.T("tools.regex.results"),
					TextStyle: gui.CurrentTheme().TextStyleBody.Bold(),
				}),
				gui.Label(i18n.Tf("tools.regex.match_count", res.Count), gui.CurrentTheme().TextStyleSecondary),
			}
			if res.Count == 0 {
				rows = append(rows, gui.Label(i18n.T("tools.regex.no_matches"), gui.CurrentTheme().TextStyleSecondary))
			} else {
				for i, m := range res.Matches {
					rows = append(rows, gui.Label(
						i18n.Tf("tools.regex.match_n", i+1, m.Text, m.Index),
						gui.CurrentTheme().TextStyleBody,
					))
					if len(m.Groups) > 0 {
						rows = append(rows, gui.Label(
							i18n.T("tools.regex.capture_groups")+": "+strings.Join(m.Groups, ", "),
							gui.CurrentTheme().TextStyleSecondary,
						))
					}
				}
			}
			body = append(body, resultCard(rows...))
		}
	}
	return panelShell(i18n.T("tools.regex.title"), i18n.T("tools.regex.desc"), body)
}

func panelTextDiff(d *Deps, st *State) gui.View {
	body := []gui.View{
		formCard(
			multiInput("tools-diff-left", i18n.T("tools.text_diff.original"), st.Diff.Left, "", multiMinH, func(v string) {
				st.Diff.Left = v
			}),
			multiInput("tools-diff-right", i18n.T("tools.text_diff.revised"), st.Diff.Right, "", multiMinH, func(v string) {
				st.Diff.Right = v
			}),
			primaryBtn("tools-diff-run", i18n.T("tools.shared.compare"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.Diff.Left) == "" && strings.TrimSpace(s.Diff.Right) == "" {
					toastErr(ctx.Window, i18n.T("tools.text_diff.both_empty"))
					return
				}
				s.Diff.Result = d.Core.DiffText(s.Diff.Left, s.Diff.Right)
				s.Diff.Has = true
			}),
		),
	}
	if st.Diff.Has {
		res := st.Diff.Result
		if !res.Success {
			body = append(body, resultCard(statusLine(false, res.Error)))
		} else {
			th := gui.CurrentTheme()
			lines := make([]gui.View, 0, len(res.Lines)+1)
			lines = append(lines, gui.Text(gui.TextCfg{
				Text:      i18n.T("tools.shared.result"),
				TextStyle: th.TextStyleBody.Bold(),
			}))
			for _, line := range res.Lines {
				style := th.TextStyleCode
				prefix := "  "
				switch line.Type {
				case "add":
					prefix = "+ "
					style = th.TextStyleSuccess
				case "remove":
					prefix = "- "
					style = th.TextStyleError
				}
				lines = append(lines, gui.Text(gui.TextCfg{
					Text:      prefix + line.Content,
					TextStyle: style,
				}))
			}
			body = append(body, resultCard(lines...))
		}
	}
	return panelShell(i18n.T("tools.text_diff.title"), i18n.T("tools.text_diff.desc"), body)
}

func panelBaseConvert(d *Deps, st *State) gui.View {
	body := []gui.View{
		formCard(
			fieldInput("tools-base-in", i18n.T("tools.base_convert.input"), st.Base.Input, "", func(v string) {
				st.Base.Input = v
			}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					fieldInput("tools-base-from", i18n.T("tools.base_convert.from_base"), st.Base.FromBase, "2-36", func(v string) {
						st.Base.FromBase = v
					}),
					fieldInput("tools-base-to", i18n.T("tools.base_convert.to_base"), st.Base.ToBase, "2-36", func(v string) {
						st.Base.ToBase = v
					}),
				},
			})),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					secondaryBtn("tools-base-swap", i18n.T("tools.base_convert.swap"), func(ctx gui.EventCtx) {
						ctx.Consume()
						s := gui.State[State](ctx.Window)
						s.Base.FromBase, s.Base.ToBase = s.Base.ToBase, s.Base.FromBase
						if s.Base.Has && s.Base.Result.Success && s.Base.Result.Result != "" {
							s.Base.Input = s.Base.Result.Result
							s.Base.Has = false
						}
					}),
					primaryBtn("tools-base-run", i18n.T("tools.base_convert.convert"), false, func(ctx gui.EventCtx) {
						ctx.Consume()
						s := gui.State[State](ctx.Window)
						if strings.TrimSpace(s.Base.Input) == "" {
							toastErr(ctx.Window, i18n.T("tools.base_convert.input_required"))
							return
						}
						from, err := strconv.Atoi(strings.TrimSpace(s.Base.FromBase))
						if err != nil {
							toastErr(ctx.Window, i18n.T("tool.err.from_base"))
							return
						}
						to, err := strconv.Atoi(strings.TrimSpace(s.Base.ToBase))
						if err != nil {
							toastErr(ctx.Window, i18n.T("tool.err.to_base"))
							return
						}
						s.Base.Result = d.Core.ConvertBase(s.Base.Input, from, to)
						s.Base.Has = true
					}),
				},
			})),
		),
	}
	if st.Base.Has {
		res := st.Base.Result
		rows := []gui.View{statusLine(res.Success, okOrErr(res.Success, res.Error))}
		if res.Success {
			rows = append(rows,
				gui.Input(gui.InputCfg{
					ID:       "tools-base-out",
					Label:    i18n.T("tools.base_convert.result"),
					Text:     res.Result,
					ReadOnly: true,
					Sizing:   gui.FillFit,
				}),
				secondaryBtn("tools-base-copy", i18n.T("tools.base_convert.copy_result"), func(ctx gui.EventCtx) {
					ctx.Consume()
					copyText(ctx.Window, gui.State[State](ctx.Window).Base.Result.Result)
				}),
			)
		}
		body = append(body, resultCard(rows...))
	}
	return panelShell(i18n.T("tools.base_convert.title"), i18n.T("tools.base_convert.desc"), body)
}
