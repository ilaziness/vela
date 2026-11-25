package toolsui

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

func panelJSONYAML(d *Deps, st *State) gui.View {
	actionOpts := []gui.SelectOption{
		{Label: i18n.T("tools.json_yaml.format"), Value: "format"},
		{Label: i18n.T("tools.json_yaml.minify"), Value: "minify"},
		{Label: i18n.T("tools.json_yaml.validate"), Value: "validate"},
		{Label: i18n.T("tools.json_yaml.to_yaml"), Value: "json2yaml"},
		{Label: i18n.T("tools.json_yaml.to_json"), Value: "yaml2json"},
	}
	body := []gui.View{
		formCard(
			selectField("tools-json-action", i18n.T("tools.json_yaml.action"), st.JSON.Action, actionOpts, func(v string) {
				st.JSON.Action = v
			}),
			multiInput("tools-json-input", i18n.T("tools.shared.input"), st.JSON.Input, "", multiMinH, func(v string) {
				st.JSON.Input = v
			}),
			primaryBtn("tools-json-run", i18n.T("tools.shared.run"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.JSON.Input) == "" {
					toastErr(ctx.Window, i18n.T("tools.shared.input_required"))
					return
				}
				s.JSON.Result = d.Core.FormatJSONYAML(s.JSON.Action, s.JSON.Input)
				s.JSON.Has = true
			}),
		),
	}
	if st.JSON.Has {
		res := st.JSON.Result
		rows := []gui.View{statusLine(res.Success, okOrErr(res.Success, res.Error))}
		if res.Success {
			rows = append(rows,
				multiReadonly("tools-json-out", i18n.T("tools.shared.result"), res.Result, multiMinH),
				secondaryBtn("tools-json-copy", i18n.T("tools.shared.copy_result"), func(ctx gui.EventCtx) {
					ctx.Consume()
					copyText(ctx.Window, gui.State[State](ctx.Window).JSON.Result.Result)
				}),
			)
		}
		body = append(body, resultCard(rows...))
	}
	return panelShell(i18n.T("tools.json_yaml.title"), i18n.T("tools.json_yaml.desc"), body)
}

func panelEncoder(d *Deps, st *State) gui.View {
	typeOpts := []gui.SelectOption{
		{Label: "Base64", Value: "base64"},
		{Label: "URL", Value: "url"},
		{Label: "HTML", Value: "html"},
	}
	body := []gui.View{
		formCard(
			selectField("tools-enc-type", i18n.T("tools.encoder.encoding_type"), st.Encoder.Type, typeOpts, func(v string) {
				st.Encoder.Type = v
			}),
			multiInput("tools-enc-input", i18n.T("tools.shared.input"), st.Encoder.Input,
				i18n.T("tools.encoder.input_placeholder"), multiMinH, func(v string) {
					st.Encoder.Input = v
				}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					primaryBtn("tools-enc-encode", i18n.T("tools.shared.encode"), false, func(ctx gui.EventCtx) {
						ctx.Consume()
						s := gui.State[State](ctx.Window)
						if strings.TrimSpace(s.Encoder.Input) == "" {
							toastErr(ctx.Window, i18n.T("tools.encoder.encode_required"))
							return
						}
						res := d.Core.Encode(s.Encoder.Type, s.Encoder.Input)
						s.Encoder.Has = true
						s.Encoder.Error = res.Error
						s.Encoder.Output = res.Result
					}),
					secondaryBtn("tools-enc-decode", i18n.T("tools.shared.decode"), func(ctx gui.EventCtx) {
						ctx.Consume()
						s := gui.State[State](ctx.Window)
						if strings.TrimSpace(s.Encoder.Input) == "" {
							toastErr(ctx.Window, i18n.T("tools.encoder.decode_required"))
							return
						}
						res := d.Core.Decode(s.Encoder.Type, s.Encoder.Input)
						s.Encoder.Has = true
						s.Encoder.Error = res.Error
						s.Encoder.Output = res.Result
					}),
				},
			}),
		),
	}
	if st.Encoder.Has {
		ok := st.Encoder.Error == ""
		rows := []gui.View{statusLine(ok, okOrErr(ok, st.Encoder.Error))}
		if ok {
			rows = append(rows,
				multiReadonly("tools-enc-out", i18n.T("tools.shared.result"), st.Encoder.Output, multiMinH),
				secondaryBtn("tools-enc-copy", i18n.T("tools.shared.copy_result"), func(ctx gui.EventCtx) {
					ctx.Consume()
					copyText(ctx.Window, gui.State[State](ctx.Window).Encoder.Output)
				}),
			)
		}
		body = append(body, resultCard(rows...))
	}
	return panelShell(i18n.T("tools.encoder.title"), i18n.T("tools.encoder.desc"), body)
}

func panelHash(d *Deps, st *State) gui.View {
	algoOpts := []gui.SelectOption{
		{Label: "MD5", Value: "md5"},
		{Label: "SHA1", Value: "sha1"},
		{Label: "SHA256", Value: "sha256"},
		{Label: "SHA512", Value: "sha512"},
	}
	body := []gui.View{
		formCard(
			multiInput("tools-hash-input", i18n.T("tools.shared.input"), st.Hash.Input,
				i18n.T("tools.hash.input_placeholder"), 120, func(v string) {
					st.Hash.Input = v
				}),
			selectField("tools-hash-algo", i18n.T("tools.hash.algorithm"), st.Hash.Algorithm, algoOpts, func(v string) {
				st.Hash.Algorithm = v
			}),
			primaryBtn("tools-hash-run", i18n.T("tools.shared.calculate"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.Hash.Input) == "" {
					toastErr(ctx.Window, i18n.T("tools.shared.input_required"))
					return
				}
				s.Hash.Result = d.Core.CalculateHash(s.Hash.Input, s.Hash.Algorithm)
				s.Hash.Has = true
			}),
		),
	}
	if st.Hash.Has {
		res := st.Hash.Result
		msg := i18n.T("tools.hash.hash_success")
		if !res.Success {
			msg = i18n.Tf("tools.hash.hash_failed", res.Error)
		}
		rows := []gui.View{statusLine(res.Success, msg)}
		if res.Success {
			rows = append(rows,
				gui.Label(i18n.Tf("tools.hash.chip_algorithm", res.Algorithm), gui.CurrentTheme().TextStyleSecondary),
				multiReadonly("tools-hash-out", i18n.T("tools.hash.chip_result"), res.Result, 80),
				secondaryBtn("tools-hash-copy", i18n.T("tools.shared.copy_result"), func(ctx gui.EventCtx) {
					ctx.Consume()
					copyText(ctx.Window, gui.State[State](ctx.Window).Hash.Result.Result)
				}),
			)
		}
		body = append(body, resultCard(rows...))
	}
	return panelShell(i18n.T("tools.hash.title"), i18n.T("tools.hash.desc"), body)
}

func panelJWT(d *Deps, st *State) gui.View {
	body := []gui.View{
		formCard(
			multiInput("tools-jwt-token", "JWT", st.JWT.Token,
				"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", 120, func(v string) {
					st.JWT.Token = v
				}),
			primaryBtn("tools-jwt-run", i18n.T("tools.shared.decode"), false, func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if strings.TrimSpace(s.JWT.Token) == "" {
					toastErr(ctx.Window, i18n.T("tools.jwt.jwt_required"))
					return
				}
				s.JWT.Result = d.Core.DecodeJWT(s.JWT.Token)
				s.JWT.Has = true
			}),
		),
	}
	if st.JWT.Has {
		res := st.JWT.Result
		if !res.Success {
			body = append(body, resultCard(statusLine(false, res.Error)))
		} else {
			body = append(body, resultCard(
				multiReadonly("tools-jwt-header", "Header", res.Header, 100),
				secondaryBtn("tools-jwt-copy-h", i18n.T("tools.jwt.copy_header"), func(ctx gui.EventCtx) {
					ctx.Consume()
					copyText(ctx.Window, gui.State[State](ctx.Window).JWT.Result.Header)
				}),
				multiReadonly("tools-jwt-payload", "Payload", res.Payload, 140),
				secondaryBtn("tools-jwt-copy-p", i18n.T("tools.jwt.copy_payload"), func(ctx gui.EventCtx) {
					ctx.Consume()
					copyText(ctx.Window, gui.State[State](ctx.Window).JWT.Result.Payload)
				}),
				multiReadonly("tools-jwt-sig", i18n.T("tools.jwt.signature"), res.Signature, 80),
			))
		}
	}
	return panelShell(i18n.T("tools.jwt.title"), i18n.T("tools.jwt.desc"), body)
}

func okOrErr(ok bool, err string) string {
	if ok {
		return i18n.T("tools.shared.success")
	}
	if err != "" {
		return err
	}
	return i18n.Tf("tools.shared.failed", "")
}
