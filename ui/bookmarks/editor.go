package bookmarks

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

func editor(w *gui.Window, d *Deps) gui.View {
	st := gui.State[State](w)
	th := gui.CurrentTheme()
	if !st.Editing {
		return gui.Column(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			HAlign:  gui.HAlignCenter,
			VAlign:  gui.VAlignMiddle,
			Color:   th.ColorBackground,
			Padding: gui.PaddingLarge,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: i18n.T("bookmark.select_to_edit"), TextStyle: th.TextStyleTitle}),
				gui.Label(i18n.T("bookmark.select_to_edit_hint"), th.TextStyleSecondary),
			},
		}))
	}

	isSerial := st.Draft.Protocol == "serial"
	tabOpts := []gui.SegmentOption{
		{Label: i18n.T("bookmark.tab.basic"), Value: tabBasic},
		{Label: i18n.T("bookmark.tab.connection"), Value: tabConnection},
	}
	if !isSerial {
		tabOpts = append(tabOpts,
			gui.SegmentOption{Label: i18n.T("bookmark.tab.auth"), Value: tabAuth},
			gui.SegmentOption{Label: i18n.T("bookmark.tab.session"), Value: tabSession},
		)
	}
	active := st.ActiveTab
	if isSerial && (active == tabAuth || active == tabSession) {
		active = tabConnection
		st.ActiveTab = active
	}

	var body gui.View
	switch active {
	case tabConnection:
		body = tabConnectionBody(st)
	case tabAuth:
		body = tabAuthBody(st)
	case tabSession:
		body = tabSessionBody(st)
	default:
		body = tabBasicBody(st)
	}

	footer := actionsRight(
		gui.Button(gui.ButtonCfg{
			ID: "bm-test", Variant: gui.ButtonSecondary,
			Label: i18n.T("bookmark.test_connect"), Disabled: st.Busy,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				doTest(ctx.Window, d)
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID: "bm-save", Variant: gui.ButtonPrimary,
			Label: i18n.T("common.save"), Disabled: st.Busy,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				doSave(ctx.Window, d)
			},
		}),
	)

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   th.ColorBackground,
		Padding: gui.NewPadding(16, 20, 16, 20),
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.SegmentedControl(gui.SegmentedControlCfg{
				ID:      "bm-tabs",
				Value:   active,
				Options: tabOpts,
				Sizing:  gui.FitFit,
				OnSelect: func(v string, ctx gui.EventCtx) {
					ctx.Consume()
					gui.State[State](ctx.Window).ActiveTab = v
				},
			}),
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Spacing: gui.SpacingMedium,
				Content: []gui.View{body},
			})),
			footer,
		},
	}))
}

func tabBasicBody(st *State) gui.View {
	b := &st.Draft
	rows := []gui.View{
		iconPickerRow(st),
		formRow("bm-title", i18n.T("bookmark.field.name"), b.Title,
			i18n.T("bookmark.field.name_placeholder"), false, func(v string) {
				b.Title = v
				st.Dirty = true
			}),
		gui.Select(gui.SelectCfg{
			ID:       "bm-group",
			Label:    i18n.T("bookmark.field.group"),
			Selected: []string{b.GroupName},
			Options:  groupSelectOptions(st, false),
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) == 0 {
					return
				}
				s := gui.State[State](ctx.Window)
				s.Draft.GroupName = vals[0]
				s.Dirty = true
			},
		}),
		tagsRow(st),
		colorRow(st),
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: rows,
	}))
}

func iconPickerRow(st *State) gui.View {
	th := gui.CurrentTheme()
	btns := make([]gui.View, 0, len(iconPresets))
	for _, p := range iconPresets {
		token := p.Token
		glyph := p.Glyph
		selected := st.Draft.Icon == token
		bg := gui.Color{}
		if selected {
			bg = th.ColorActive
		}
		iconViews := []gui.View{
			gui.Text(gui.TextCfg{Text: glyph, TextStyle: th.TextStyleIconSmall}),
		}
		if token == "" {
			iconViews = append(iconViews, gui.Text(gui.TextCfg{
				Text:      " " + i18n.T("common.default"),
				TextStyle: th.TextStyleBody,
			}))
		}
		btns = append(btns, gui.Button(gui.ButtonCfg{
			ID:      "bm-icon-" + token,
			Variant: gui.ButtonGhost,
			Color:   bg,
			Sizing:  gui.FitFit,
			Content: []gui.View{
				gui.Row(layout.Flush(gui.ContainerCfg{
					Sizing:  gui.FitFit,
					Spacing: gui.SpacingTight,
					VAlign:  gui.VAlignMiddle,
					Content: iconViews,
				})),
			},
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				s.Draft.Icon = token
				s.Dirty = true
			},
		}))
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: i18n.T("bookmark.field.icon"), TextStyle: th.TextStyleLabel}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingTight,
				Content: btns,
			})),
		},
	}))
}

func tagsRow(st *State) gui.View {
	th := gui.CurrentTheme()
	chips := make([]gui.View, 0, len(st.Draft.Tags))
	for i, tag := range st.Draft.Tags {
		idx := i
		t := tag
		chips = append(chips, gui.Button(gui.ButtonCfg{
			ID:      "bm-tag-" + strconv.Itoa(idx),
			Variant: gui.ButtonSecondary,
			Sizing:  gui.FitFit,
			Content: []gui.View{
				gui.Row(layout.Flush(gui.ContainerCfg{
					Sizing:  gui.FitFit,
					Spacing: gui.SpacingTight,
					VAlign:  gui.VAlignMiddle,
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: t, TextStyle: th.TextStyleBody}),
						gui.Text(gui.TextCfg{Text: gui.IconClose, TextStyle: th.TextStyleIconSmall}),
					},
				})),
			},
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if idx < 0 || idx >= len(s.Draft.Tags) {
					return
				}
				s.Draft.Tags = append(s.Draft.Tags[:idx], s.Draft.Tags[idx+1:]...)
				s.Dirty = true
			},
		}))
	}
	parts := []gui.View{
		gui.Text(gui.TextCfg{Text: i18n.T("bookmark.field.tags"), TextStyle: th.TextStyleLabel}),
	}
	if len(chips) > 0 {
		parts = append(parts, gui.Row(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.SpacingTight,
			Content: chips,
		})))
	}
	parts = append(parts, gui.Input(gui.InputCfg{
		ID:          "bm-tag-input",
		Text:        st.TagDraft,
		Placeholder: i18n.T("bookmark.field.tags_placeholder"),
		Sizing:      gui.FillFit,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			gui.State[State](ctx.Window).TagDraft = v
			ctx.Consume()
		},
		OnTextCommit: func(v string, reason gui.InputCommitReason, ctx gui.EventCtx) {
			if reason != gui.InputCommitEnter {
				return
			}
			ctx.Consume()
			v = strings.TrimSpace(v)
			if v == "" {
				return
			}
			s := gui.State[State](ctx.Window)
			s.Draft.Tags = append(s.Draft.Tags, v)
			s.TagDraft = ""
			s.Dirty = true
		},
	}))
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: parts,
	}))
}

func colorRow(st *State) gui.View {
	th := gui.CurrentTheme()
	c, ok := parseBookmarkColor(st.Draft.Color)
	hexLabel := i18n.T("common.default")
	if ok {
		hexLabel = strings.ToLower(c.Hex())
	}
	swatch := gui.ColorSwatch(gui.ColorSwatchCfg{
		ID:        "bm-color-swatch",
		Color:     c,
		Width:     48,
		Height:    28,
		Focusable: true,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			openColorPicker(ctx.Window)
		},
	})
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: i18n.T("bookmark.field.color"), TextStyle: th.TextStyleLabel}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID: "bm-color-none", Variant: gui.ButtonSecondary, Sizing: gui.FitFit,
						Label: i18n.T("common.default"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							s := gui.State[State](ctx.Window)
							s.Draft.Color = ""
							s.Dirty = true
						},
					}),
					swatch,
					gui.Label(hexLabel, th.TextStyleSecondary),
				},
			})),
		},
	}))
}

func openColorPicker(w *gui.Window) {
	st := gui.State[State](w)
	c, ok := parseBookmarkColor(st.Draft.Color)
	if !ok {
		c, _ = gui.ColorFromHex("#1e88e5")
	}
	st.ColorDraft = c
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("bookmark.field.color"),
		Width:      360,
		CustomView: func(win *gui.Window) gui.View {
			s := gui.State[State](win)
			return gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.PaddingMedium,
				Content: []gui.View{
					gui.ColorPicker(gui.ColorPickerCfg{
						ID:    "bm-color-picker",
						Color: s.ColorDraft,
						OnColorChange: func(c gui.Color, ctx gui.EventCtx) {
							gui.State[State](ctx.Window).ColorDraft = c
							ctx.Consume()
						},
					}),
					actionsRight(
						gui.Button(gui.ButtonCfg{
							ID: "bm-color-cancel", Variant: gui.ButtonSecondary,
							Label: i18n.T("common.cancel"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								ctx.Window.DialogDismiss()
							},
						}),
						gui.Button(gui.ButtonCfg{
							ID: "bm-color-ok", Variant: gui.ButtonPrimary,
							Label: i18n.T("common.ok"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								s := gui.State[State](ctx.Window)
								s.Draft.Color = strings.ToLower(s.ColorDraft.Hex())
								s.Dirty = true
								ctx.Window.DialogDismiss()
							},
						}),
					),
				},
			})
		},
	})
}

func tabConnectionBody(st *State) gui.View {
	b := &st.Draft
	proto := b.Protocol
	if proto == "" {
		proto = "ssh"
	}
	rows := []gui.View{
		gui.SegmentedControl(gui.SegmentedControlCfg{
			ID:    "bm-proto",
			Value: proto,
			Options: []gui.SegmentOption{
				{Label: i18n.T("session.protocol.ssh"), Value: "ssh"},
				{Label: i18n.T("session.protocol.serial"), Value: "serial"},
			},
			Sizing: gui.FitFit,
			OnSelect: func(v string, ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if s.Draft.Protocol == v {
					return
				}
				switchProtocol(&s.Draft, v)
				s.EnvText = ""
				s.HasPassword = false
				s.HasKeyPassword = false
				s.HasProxyPass = false
				s.Dirty = true
				if v == "serial" && (s.ActiveTab == tabAuth || s.ActiveTab == tabSession) {
					s.ActiveTab = tabConnection
				}
			},
		}),
	}
	if b.Protocol == "serial" {
		rows = append(rows, serialFields(st)...)
	} else {
		rows = append(rows, sshConnFields(st)...)
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: rows,
	}))
}

func switchProtocol(b *bookmark.Bookmark, proto string) {
	b.Protocol = proto
	if proto == "serial" {
		clearSSHFields(b)
		emptySerialDefaults(b)
	} else {
		clearSerialFields(b)
		if b.Port <= 0 {
			b.Port = 22
		}
		if b.ProxyMode == "" {
			b.ProxyMode = "inherit"
		}
		if b.Charset == "" {
			b.Charset = "utf-8"
		}
	}
}

func sshConnFields(st *State) []gui.View {
	b := &st.Draft
	port := ""
	if b.Port > 0 {
		port = strconv.Itoa(b.Port)
	}
	rows := []gui.View{
		formRow("bm-host", i18n.T("bookmark.field.host")+"*", b.Host,
			i18n.T("bookmark.field.host_placeholder"), false, func(v string) {
				b.Host = v
				st.Dirty = true
			}),
		formRow("bm-port", i18n.T("bookmark.field.port")+"*", port,
			i18n.T("bookmark.field.port_placeholder"), false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					b.Port = n
				} else {
					b.Port = 0
				}
				st.Dirty = true
			}),
		formRow("bm-user", i18n.T("bookmark.field.username")+"*", b.User,
			i18n.T("bookmark.field.username_placeholder"), false, func(v string) {
				b.User = v
				st.Dirty = true
			}),
		jumpSelect(st),
		proxyModeSelect(st),
	}
	if b.ProxyMode == "custom" {
		rows = append(rows, customProxyFields(st)...)
	}
	return rows
}

func jumpSelect(st *State) gui.View {
	opts := []gui.SelectOption{{Label: i18n.T("bookmark.field.proxy_jump_placeholder"), Value: ""}}
	self := st.Draft.ID
	for _, g := range st.Groups {
		if g == nil {
			continue
		}
		for _, b := range g.Bookmarks {
			if b.Protocol != "ssh" || b.ID == "" || b.ID == self {
				continue
			}
			label := b.Title
			if sub := bookmarkSubtitle(b); sub != "" {
				label = b.Title + " (" + sub + ")"
			}
			opts = append(opts, gui.SelectOption{Label: label, Value: b.ID})
		}
	}
	return gui.Select(gui.SelectCfg{
		ID:       "bm-jump",
		Label:    i18n.T("bookmark.field.proxy_jump"),
		Selected: []string{st.Draft.ProxyJumpID},
		Options:  opts,
		Sizing:   gui.FillFit,
		OnSelect: func(vals []string, ctx gui.EventCtx) {
			ctx.Consume()
			s := gui.State[State](ctx.Window)
			if len(vals) > 0 {
				s.Draft.ProxyJumpID = vals[0]
			} else {
				s.Draft.ProxyJumpID = ""
			}
			s.Dirty = true
		},
	})
}

func proxyModeSelect(st *State) gui.View {
	mode := st.Draft.ProxyMode
	if mode == "" {
		mode = "inherit"
	}
	return gui.Select(gui.SelectCfg{
		ID:       "bm-proxy-mode",
		Label:    i18n.T("bookmark.field.proxy_mode"),
		Selected: []string{mode},
		Options: []gui.SelectOption{
			{Label: i18n.T("bookmark.proxy_mode.inherit"), Value: "inherit"},
			{Label: i18n.T("bookmark.proxy_mode.none"), Value: "none"},
			{Label: i18n.T("bookmark.proxy_mode.custom"), Value: "custom"},
		},
		Sizing: gui.FillFit,
		OnSelect: func(vals []string, ctx gui.EventCtx) {
			ctx.Consume()
			if len(vals) == 0 {
				return
			}
			s := gui.State[State](ctx.Window)
			s.Draft.ProxyMode = vals[0]
			if vals[0] == "custom" && s.Draft.ProxyType == "" {
				s.Draft.ProxyType = "http"
			}
			s.Dirty = true
		},
	})
}

func customProxyFields(st *State) []gui.View {
	b := &st.Draft
	port := ""
	if b.ProxyPort > 0 {
		port = strconv.Itoa(b.ProxyPort)
	}
	return []gui.View{
		gui.Select(gui.SelectCfg{
			ID: "bm-proxy-type", Label: i18n.T("bookmark.field.proxy_type"),
			Selected: []string{b.ProxyType},
			Options: []gui.SelectOption{
				{Label: "HTTP", Value: "http"},
				{Label: "SOCKS5", Value: "socks5"},
			},
			Sizing: gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					s := gui.State[State](ctx.Window)
					s.Draft.ProxyType = vals[0]
					s.Dirty = true
				}
			},
		}),
		formRow("bm-proxy-host", i18n.T("bookmark.field.proxy_host"), b.ProxyHost,
			i18n.T("bookmark.proxy_host_placeholder"), false, func(v string) {
				b.ProxyHost = v
				st.Dirty = true
			}),
		formRow("bm-proxy-port", i18n.T("bookmark.field.proxy_port"), port,
			i18n.T("bookmark.proxy_port_placeholder"), false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					b.ProxyPort = n
				} else {
					b.ProxyPort = 0
				}
				st.Dirty = true
			}),
		formRow("bm-proxy-user", i18n.T("bookmark.field.username"), b.ProxyUser, "", false, func(v string) {
			b.ProxyUser = v
			st.Dirty = true
		}),
		formRow("bm-proxy-pass", i18n.T("bookmark.field.password"),
			secretInputValue(b.ProxyPassword),
			secretPlaceholder(st.HasProxyPass, ""), true, func(v string) {
				if applySecretInput(&b.ProxyPassword, &st.HasProxyPass, v) {
					st.Dirty = true
				}
			}),
		gui.Label(i18n.T("bookmark.proxy_jump_hint"), gui.CurrentTheme().TextStyleSecondary),
	}
}

func serialFields(st *State) []gui.View {
	b := &st.Draft
	baud := strconv.Itoa(b.BaudRate)
	if b.BaudRate <= 0 {
		baud = "115200"
	}
	return []gui.View{
		formRow("bm-serial-dev", i18n.T("serial.device"), b.SerialDevice, "", false, func(v string) {
			b.SerialDevice = v
			st.Dirty = true
		}),
		formRow("bm-serial-baud", i18n.T("serial.baud_rate"), baud, "", false, func(v string) {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				b.BaudRate = n
			}
			st.Dirty = true
		}),
		gui.Select(gui.SelectCfg{
			ID: "bm-serial-databits", Label: i18n.T("serial.data_bits"),
			Selected: []string{strconv.Itoa(orDefault(b.DataBits, 8))},
			Options:  intOpts(5, 6, 7, 8),
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					if n, err := strconv.Atoi(vals[0]); err == nil {
						gui.State[State](ctx.Window).Draft.DataBits = n
						gui.State[State](ctx.Window).Dirty = true
					}
				}
			},
		}),
		gui.Select(gui.SelectCfg{
			ID: "bm-serial-parity", Label: i18n.T("serial.parity"),
			Selected: []string{orStr(b.Parity, "none")},
			Options: []gui.SelectOption{
				{Label: "none", Value: "none"}, {Label: "even", Value: "even"},
				{Label: "odd", Value: "odd"}, {Label: "mark", Value: "mark"}, {Label: "space", Value: "space"},
			},
			Sizing: gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).Draft.Parity = vals[0]
					gui.State[State](ctx.Window).Dirty = true
				}
			},
		}),
		gui.Select(gui.SelectCfg{
			ID: "bm-serial-stop", Label: i18n.T("serial.stop_bits"),
			Selected: []string{strconv.Itoa(orDefault(b.StopBits, 1))},
			Options:  intOpts(1, 2),
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					if n, err := strconv.Atoi(vals[0]); err == nil {
						gui.State[State](ctx.Window).Draft.StopBits = n
						gui.State[State](ctx.Window).Dirty = true
					}
				}
			},
		}),
		gui.Select(gui.SelectCfg{
			ID: "bm-serial-flow", Label: i18n.T("serial.flow_control"),
			Selected: []string{orStr(b.FlowControl, "none")},
			Options: []gui.SelectOption{
				{Label: "none", Value: "none"}, {Label: "hardware", Value: "hardware"},
			},
			Sizing: gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).Draft.FlowControl = vals[0]
					gui.State[State](ctx.Window).Dirty = true
				}
			},
		}),
		charsetSelect("bm-serial-charset", b.Charset, func(v string) {
			b.Charset = v
			st.Dirty = true
		}),
	}
}

func tabAuthBody(st *State) gui.View {
	b := &st.Draft
	keyOpts := []gui.SelectOption{{Label: i18n.T("bookmark.field.app_key_none"), Value: ""}}
	for _, k := range st.Keys {
		keyOpts = append(keyOpts, gui.SelectOption{Label: k.Name, Value: k.ID})
	}
	if b.SshKeyID != "" {
		found := false
		for _, o := range keyOpts {
			if o.Value == b.SshKeyID {
				found = true
				break
			}
		}
		if !found {
			keyOpts = append(keyOpts, gui.SelectOption{Label: i18n.T("bookmark.field.app_key_saved"), Value: b.SshKeyID})
		}
	}

	rows := []gui.View{
		gui.Switch(gui.SwitchCfg{
			ID:       "bm-forward-agent",
			Label:    i18n.T("bookmark.field.agent_forward_label"),
			Selected: b.ForwardAgent,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				s.Draft.ForwardAgent = !s.Draft.ForwardAgent
				s.Dirty = true
			},
		}),
		formRow("bm-password", i18n.T("bookmark.field.password"),
			secretInputValue(b.Password),
			secretPlaceholder(st.HasPassword, i18n.T("bookmark.field.password_placeholder")),
			true, func(v string) {
				if applySecretInput(&b.Password, &st.HasPassword, v) {
					st.Dirty = true
				}
			}),
		gui.Select(gui.SelectCfg{
			ID: "bm-app-key", Label: i18n.T("bookmark.field.app_key"),
			Selected: []string{b.SshKeyID},
			Options:  keyOpts,
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				id := ""
				if len(vals) > 0 {
					id = vals[0]
				}
				s.Draft.SshKeyID = id
				if id != "" {
					s.Draft.PrivateKey = ""
					s.Draft.PrivateKeyPassword = ""
					s.HasKeyPassword = false
				}
				s.Dirty = true
			},
		}),
		pathBrowseRow("bm-key-file", i18n.T("bookmark.field.key_file"), b.PrivateKey,
			i18n.T("bookmark.field.key_file_placeholder"), i18n.T("common.select_file"), func(ctx gui.EventCtx) {
				ctx.Consume()
				ctx.Window.NativeOpenDialog(gui.NativeOpenDialogCfg{
					Title: i18n.T("bookmark.field.key_file"),
					OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
						if res.Status != gui.DialogOK || len(res.Paths) == 0 {
							return
						}
						s := gui.State[State](win)
						s.Draft.PrivateKey = res.Paths[0].Path
						s.Draft.SshKeyID = ""
						s.Dirty = true
					},
				})
			}),
		formRow("bm-key-pass", i18n.T("bookmark.field.key_passphrase"),
			secretInputValue(b.PrivateKeyPassword),
			secretPlaceholder(st.HasKeyPassword && b.SshKeyID == "",
				i18n.T("bookmark.field.key_passphrase_placeholder")),
			true, func(v string) {
				if b.SshKeyID != "" {
					return
				}
				if applySecretInput(&b.PrivateKeyPassword, &st.HasKeyPassword, v) {
					st.Dirty = true
				}
			}),
		pathBrowseRow("bm-cert", i18n.T("bookmark.field.cert_file"), b.Certificate,
			i18n.T("bookmark.field.cert_file_placeholder"), i18n.T("common.select_file"), func(ctx gui.EventCtx) {
				ctx.Consume()
				ctx.Window.NativeOpenDialog(gui.NativeOpenDialogCfg{
					Title: i18n.T("bookmark.select_cert"),
					OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
						if res.Status != gui.DialogOK || len(res.Paths) == 0 {
							return
						}
						s := gui.State[State](win)
						s.Draft.Certificate = res.Paths[0].Path
						s.Dirty = true
					},
				})
			}),
	}
	if b.Certificate != "" {
		rows = append(rows, gui.Button(gui.ButtonCfg{
			ID: "bm-clear-cert", Variant: gui.ButtonGhost,
			Label: i18n.T("bookmark.clear_cert"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				s.Draft.Certificate = ""
				s.Dirty = true
			},
		}))
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: rows,
	}))
}

func tabSessionBody(st *State) gui.View {
	b := &st.Draft
	termOpts := []gui.SelectOption{
		{Label: i18n.T("bookmark.term_type_default"), Value: ""},
		{Label: "xterm-256color", Value: "xterm-256color"},
		{Label: "xterm", Value: "xterm"},
		{Label: "vt100", Value: "vt100"},
		{Label: "linux", Value: "linux"},
		{Label: "screen", Value: "screen"},
		{Label: "screen-256color", Value: "screen-256color"},
		{Label: "tmux", Value: "tmux"},
		{Label: "tmux-256color", Value: "tmux-256color"},
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Select(gui.SelectCfg{
				ID: "bm-term", Label: i18n.T("bookmark.field.term"),
				Selected: []string{b.Term},
				Options:  termOpts,
				Sizing:   gui.FillFit,
				OnSelect: func(vals []string, ctx gui.EventCtx) {
					ctx.Consume()
					s := gui.State[State](ctx.Window)
					if len(vals) > 0 {
						s.Draft.Term = vals[0]
					}
					s.Dirty = true
				},
			}),
			charsetSelect("bm-session-charset", b.Charset, func(v string) {
				b.Charset = v
				st.Dirty = true
			}),
			formRow("bm-startup", i18n.T("bookmark.field.startup_cmd"), b.StartupCmd,
				i18n.T("bookmark.field.startup_cmd_placeholder"), false, func(v string) {
					b.StartupCmd = v
					st.Dirty = true
				}),
			gui.Input(gui.InputCfg{
				ID:          "bm-env",
				Label:       i18n.T("bookmark.field.env"),
				Text:        st.EnvText,
				Placeholder: i18n.T("bookmark.field.env_placeholder"),
				Mode:        gui.InputMultiline,
				Sizing:      gui.FillFit,
				MinHeight:   120,
				OnTextChanged: func(v string, ctx gui.EventCtx) {
					s := gui.State[State](ctx.Window)
					s.EnvText = v
					s.Dirty = true
					ctx.Consume()
				},
			}),
		},
	}))
}

func charsetSelect(id, value string, onChange func(string)) gui.View {
	if value == "" {
		value = "utf-8"
	}
	return gui.Select(gui.SelectCfg{
		ID: id, Label: i18n.T("bookmark.field.term_encoding"),
		Selected: []string{value},
		Options: []gui.SelectOption{
			{Label: "UTF-8", Value: "utf-8"},
			{Label: "GBK", Value: "gbk"},
			{Label: "GB18030", Value: "gb18030"},
		},
		Sizing: gui.FillFit,
		OnSelect: func(vals []string, ctx gui.EventCtx) {
			ctx.Consume()
			if len(vals) > 0 {
				onChange(vals[0])
			}
		},
	})
}

func intOpts(vals ...int) []gui.SelectOption {
	out := make([]gui.SelectOption, 0, len(vals))
	for _, v := range vals {
		s := strconv.Itoa(v)
		out = append(out, gui.SelectOption{Label: s, Value: s})
	}
	return out
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
