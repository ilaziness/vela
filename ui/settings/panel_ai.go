package settings

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/ai/tool"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/mcpserver"
	"github.com/ilaziness/vela/ui/layout"
)

func panelAI(d *Deps, st *State) gui.View {
	tabs := []gui.SegmentOption{
		{Label: i18n.T("settings.ai.tab_general"), Value: aiTabGeneral},
		{Label: i18n.T("settings.ai.tab_tools"), Value: aiTabTools},
		{Label: i18n.T("settings.ai.open_mcp"), Value: aiTabMCP},
		{Label: i18n.T("settings.ai.tab_mcp_server"), Value: aiTabMCPServer},
		{Label: i18n.T("settings.ai.open_skills"), Value: aiTabSkills},
	}
	var body gui.View
	switch st.AITab {
	case aiTabTools:
		body = aiToolsBody(d, st)
	case aiTabMCP:
		body = aiMCPBody(d, st)
	case aiTabSkills:
		body = aiSkillsBody(d, st)
	case aiTabMCPServer:
		body = aiMCPServerBody(d, st)
	default:
		body = aiGeneralBody(d, st)
	}
	return panelColumn(i18n.T("settings.ai.title"), []gui.View{
		gui.SegmentedControl(gui.SegmentedControlCfg{
			ID:      "settings-ai-tabs",
			Value:   st.AITab,
			Options: tabs,
			Sizing:  gui.FitFit,
			OnSelect: func(v string, ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				s.AITab = v
				if v == aiTabMCP || v == aiTabSkills {
					refreshLists(s, d)
				}
				if v == aiTabMCPServer && d.Agent != nil {
					s.MCPStatus = d.Agent.MCPServerStatus()
				}
			},
		}),
		body,
	})
}

func aiGeneralBody(d *Deps, st *State) gui.View {
	c := &st.AI
	providers := ai.GetAllProviders()
	opts := make([]gui.SelectOption, 0, len(providers))
	for _, p := range providers {
		opts = append(opts, gui.SelectOption{Label: p.Label, Value: p.Name})
	}
	prov := string(c.Provider)
	rows := []gui.View{
		switchRow("settings-ai-enable", i18n.T("settings.ai.enable"), c.Enabled, func(gui.EventCtx) { c.Enabled = !c.Enabled }),
		gui.Select(gui.SelectCfg{
			ID:       "settings-ai-provider",
			Label:    i18n.T("settings.ai.provider"),
			Selected: []string{prov},
			Options:  opts,
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).AI.Provider = ai.Provider(vals[0])
				}
			},
		}),
		formRow("settings-ai-model", i18n.T("settings.ai.model"), c.Model,
			i18n.T("settings.ai.model_placeholder"), false, func(v string) { c.Model = v }),
		formRow("settings-ai-endpoint", i18n.T("settings.ai.endpoint"), c.Endpoint,
			i18n.T("settings.ai.endpoint_placeholder"), false, func(v string) { c.Endpoint = v }),
		formRow("settings-ai-key", i18n.T("settings.ai.api_key"), c.APIKey,
			secretPlaceholder(c.APIKey, c.APIKeyEncrypted, i18n.T("settings.ai.api_key")),
			true, func(v string) { c.APIKey = v }),
		formRowHelp("settings-ai-temp", i18n.T("settings.ai.temperature"),
			strconv.FormatFloat(c.Temperature, 'f', -1, 64), "",
			i18n.T("settings.ai.temperature_help"), false, func(v string) {
				if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					c.Temperature = n
				}
			}),
		formRowHelp("settings-ai-tokens", i18n.T("settings.ai.max_tokens"),
			strconv.Itoa(c.MaxTokens), "",
			i18n.T("settings.ai.max_tokens_help"), false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					c.MaxTokens = n
				}
			}),
		switchRow("settings-ai-echo", i18n.T("settings.ai.echo_commands"), c.EchoSSHCommands, func(gui.EventCtx) {
			c.EchoSSHCommands = !c.EchoSSHCommands
		}),
		saveBtn("settings-ai-save", i18n.T("common.save"), st.Busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			saveAI(ctx.Window, d)
		}),
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: rows,
	}))
}

func aiToolsBody(d *Deps, st *State) gui.View {
	c := &st.AI
	th := gui.CurrentTheme()
	searchOpts := []gui.SelectOption{{Label: i18n.T("settings.ai.search_none"), Value: ""}}
	for _, p := range tool.GetAllSearchProviders() {
		searchOpts = append(searchOpts, gui.SelectOption{Label: p.Label, Value: string(p.Name)})
	}

	dirRows := []gui.View{
		gui.Text(gui.TextCfg{Text: i18n.T("settings.ai.filesystem_title"), TextStyle: th.TextStyleBody.Bold()}),
		gui.Label(i18n.T("settings.ai.filesystem_help"), th.TextStyleSecondary),
	}
	if len(c.Filesystem.AllowedDirs) == 0 {
		dirRows = append(dirRows, gui.Label(i18n.T("settings.ai.filesystem_empty"), th.TextStyleSecondary))
	}
	for i, dir := range c.Filesystem.AllowedDirs {
		path := dir
		idx := i
		dirRows = append(dirRows, gui.Row(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.SpacingSmall,
			VAlign:  gui.VAlignMiddle,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      path,
					TextStyle: th.TextStyleBody,
					Sizing:    gui.FillFit,
				}),
				gui.Button(gui.ButtonCfg{
					ID:      "settings-ai-dir-del-" + strconv.Itoa(idx),
					Variant: gui.ButtonGhost,
					A11YCfg: gui.A11YCfg{A11YLabel: i18n.T("common.delete")},
					Content: []gui.View{
						gui.Text(gui.TextCfg{
							Text:      gui.IconClose,
							TextStyle: th.TextStyleIconSmall,
						}),
					},
					OnClick: func(ctx gui.EventCtx) {
						ctx.Consume()
						removeAllowedDir(gui.State[State](ctx.Window), path)
					},
				}),
			},
		})))
	}
	dirRows = append(dirRows, gui.Button(gui.ButtonCfg{
		ID:      "settings-ai-add-dir",
		Variant: gui.ButtonSecondary,
		Label:   i18n.T("settings.ai.filesystem_add"),
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			ctx.Window.NativeFolderDialog(gui.NativeFolderDialogCfg{
				Title:    i18n.T("settings.ai.filesystem_title"),
				StartDir: d.Store.LastDialogDir(),
				OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
					if res.Status != gui.DialogOK || len(res.Paths) == 0 {
						return
					}
					d.Store.RememberDialogPath(res.Paths[0].Path)
					if err := addAllowedDir(gui.State[State](win), res.Paths[0].Path); err != nil {
						toastErr(win, err)
					}
				},
			})
		},
	}))

	rows := []gui.View{
		gui.Column(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.SpacingSmall,
			Content: dirRows,
		})),
		gui.Select(gui.SelectCfg{
			ID:       "settings-ai-search",
			Label:    i18n.T("settings.ai.search_provider"),
			Selected: []string{string(c.WebSearch.Provider)},
			Options:  searchOpts,
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).AI.WebSearch.Provider = tool.SearchProvider(vals[0])
				}
			},
		}),
		formRow("settings-ai-search-key", i18n.T("settings.ai.api_key"), c.WebSearch.APIKey,
			secretPlaceholder(c.WebSearch.APIKey, c.WebSearch.APIKeyEncrypted, i18n.T("settings.ai.api_key")),
			true, func(v string) {
				c.WebSearch.APIKey = v
			}),
		saveBtn("settings-ai-tools-save", i18n.T("common.save"), st.Busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			saveAI(ctx.Window, d)
		}),
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: rows,
	}))
}

func addAllowedDir(st *State, path string) error {
	abs, err := tool.ValidateAllowedDir(path)
	if err != nil {
		return err
	}
	for _, existing := range st.AI.Filesystem.AllowedDirs {
		if sameDirPath(existing, abs) {
			return nil
		}
	}
	st.AI.Filesystem.AllowedDirs = append(st.AI.Filesystem.AllowedDirs, abs)
	return nil
}

func removeAllowedDir(st *State, path string) {
	out := st.AI.Filesystem.AllowedDirs[:0]
	for _, d := range st.AI.Filesystem.AllowedDirs {
		if !sameDirPath(d, path) {
			out = append(out, d)
		}
	}
	st.AI.Filesystem.AllowedDirs = out
}

func sameDirPath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func aiMCPBody(d *Deps, st *State) gui.View {
	rows := []gui.View{aiMCPActions(d)}
	if len(st.MCPServers) == 0 {
		rows = append(rows, gui.Label(i18n.T("settings.ai.mcp_empty"), gui.CurrentTheme().TextStyleSecondary))
	}
	for i := range st.MCPServers {
		srv := st.MCPServers[i]
		name := srv.Name
		enabled := srv.IsEnabled()
		label := name
		if enabled {
			label += " — " + i18n.T("settings.ai.mcp_on")
		} else {
			label += " — " + i18n.T("settings.ai.mcp_off")
		}
		rows = append(rows, switchListRow("settings-mcp-"+name, label, enabled, func(ctx gui.EventCtx) {
			next := !enabled
			if err := ai.SaveMCPEnabled(d.Store.UserDataDir(), name, next); err != nil {
				toastErr(ctx.Window, err)
				return
			}
			refreshLists(gui.State[State](ctx.Window), d)
		}))
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		Content: rows,
	}))
}

// aiMCPServerBody edits the built-in MCP server (streamable HTTP on
// loopback): enable switch, port, optional bearer access key, live status.
func aiMCPServerBody(d *Deps, st *State) gui.View {
	c := &st.AI.MCPServer
	th := gui.CurrentTheme()
	port := c.Port
	if port <= 0 {
		port = ai.DefaultMCPServerPort
	}
	addr := "http://127.0.0.1:" + strconv.Itoa(port) + "/mcp"

	statusText := i18n.T("settings.ai.mcp_server_status_stopped")
	switch {
	case st.MCPStatus.Err != "":
		statusText = i18n.T("settings.ai.mcp_server_status_error") + ": " + st.MCPStatus.Err
	case st.MCPStatus.Running:
		statusText = i18n.T("settings.ai.mcp_server_status_running") + " — " + st.MCPStatus.Addr
	}

	keySet := strings.TrimSpace(c.AccessKey) != "" || strings.TrimSpace(c.AccessKeyHash) != ""
	keyRows := []gui.View{
		formRow("settings-ai-mcpserver-key", i18n.T("settings.ai.mcp_server_access_key"), c.AccessKey,
			secretPlaceholder(c.AccessKey, c.AccessKeyHash, i18n.T("settings.ai.mcp_server_access_key_ph")),
			true, func(v string) {
				c.AccessKey = v
				if strings.TrimSpace(v) != "" {
					st.MCPKeyCleared = false
				}
			}),
	}
	if strings.TrimSpace(c.AccessKeyHash) != "" || st.MCPKeyCleared {
		keyRows = append(keyRows, aiLinkRow(gui.Button(gui.ButtonCfg{
			ID:      "settings-ai-mcpserver-clear-key",
			Variant: gui.ButtonGhost,
			Label:   i18n.T("settings.ai.mcp_server_clear_key"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				s.MCPKeyCleared = true
				s.AI.MCPServer.AccessKey = ""
				s.AI.MCPServer.AccessKeyHash = ""
			},
		})))
	}
	if c.Enabled && !keySet {
		keyRows = append(keyRows, gui.Text(gui.TextCfg{
			Text:      i18n.T("settings.ai.mcp_server_risk"),
			TextStyle: th.TextStyleWarning,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}))
	}

	// Build config example JSON
	exampleJSON := buildMCPServerExampleJSON(port, keySet)
	exampleLines := strings.Split(exampleJSON, "\n")
	exampleViews := make([]gui.View, 0, len(exampleLines))
	for _, line := range exampleLines {
		exampleViews = append(exampleViews, gui.Label(line, th.TextStyleBody))
	}
	keyRows = append(keyRows,
		gui.Label(i18n.T("settings.ai.mcp_server_example"), th.TextStyleSecondary),
		gui.Column(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.SpacingTight,
			Content: exampleViews,
		})),
		aiLinkRow(gui.Button(gui.ButtonCfg{
			ID:      "settings-ai-mcpserver-copy-example",
			Variant: gui.ButtonGhost,
			Label:   i18n.T("settings.ai.mcp_server_copy_example"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				ctx.Window.SetClipboard(exampleJSON)
				toastOK(ctx.Window, i18n.T("common.copied"))
			},
		})),
	)

	portText := ""
	if c.Port > 0 {
		portText = strconv.Itoa(c.Port)
	}
	rows := []gui.View{
		gui.Label(i18n.T("settings.ai.mcp_server_endpoint")+": "+addr, th.TextStyleBody),
		gui.Label(statusText, th.TextStyleSecondary),
		switchRow("settings-ai-mcpserver-enable", i18n.T("settings.ai.mcp_server_enable"), c.Enabled, func(gui.EventCtx) {
			c.Enabled = !c.Enabled
		}),
		formRow("settings-ai-mcpserver-port", i18n.T("settings.ai.mcp_server_port"), portText,
			strconv.Itoa(ai.DefaultMCPServerPort), false, func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					c.Port = n
				}
			}),
		gui.Column(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.SpacingSmall,
			Content: keyRows,
		})),
		saveBtn("settings-ai-mcpserver-save", i18n.T("common.save"), st.Busy, func(ctx gui.EventCtx) {
			ctx.Consume()
			saveAI(ctx.Window, d)
		}),
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: rows,
	}))
}

func aiSkillsBody(d *Deps, st *State) gui.View {
	rows := []gui.View{aiSkillsActions(d)}
	if len(st.Skills) == 0 {
		rows = append(rows, gui.Label(i18n.T("settings.ai.skills_empty"), gui.CurrentTheme().TextStyleSecondary))
	}
	for i := range st.Skills {
		sk := st.Skills[i]
		name := sk.Name
		enabled := sk.Enabled
		label := name
		if enabled {
			label += " — " + i18n.T("settings.ai.skill_on")
		} else {
			label += " — " + i18n.T("settings.ai.skill_off")
		}
		th := gui.CurrentTheme()
		rows = append(rows, switchListRow("settings-skill-"+name, label, enabled, func(ctx gui.EventCtx) {
			next := !enabled
			if err := ai.SetSkillEnabled(d.Store.UserDataDir(), name, next); err != nil {
				toastErr(ctx.Window, err)
				return
			}
			refreshLists(gui.State[State](ctx.Window), d)
		}, layout.IconTipButton(layout.IconTipButtonCfg{
			ID:        "settings-skill-open-" + name,
			Glyph:     gui.IconFolder,
			Tip:       i18n.T("settings.ai.open_skill_dir"),
			Th:        th,
			Variant:   gui.ButtonGhost,
			Icon:      th.TextStyleIconSmall,
			Padding:   gui.PaddingNone,
			Placement: layout.TipBelow,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				if err := d.Agent.OpenSkillDir(name); err != nil {
					toastErr(ctx.Window, err)
				}
			},
		})))
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		Content: rows,
	}))
}

func buildMCPServerExampleJSON(port int, hasKey bool) string {
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port) + "/mcp"
	if hasKey {
		return "{\n" +
			"  \"mcpServers\": {\n" +
			"    \"vela\": {\n" +
			"      \"url\": \"" + endpoint + "\",\n" +
			"      \"headers\": {\n" +
			"        \"Authorization\": \"Bearer <your-key>\"\n" +
			"      }\n" +
			"    }\n" +
			"  }\n" +
			"}"
	}
	return "{\n" +
		"  \"mcpServers\": {\n" +
		"    \"vela\": {\n" +
		"      \"url\": \"" + endpoint + "\"\n" +
		"    }\n" +
		"  }\n" +
		"}"
}

func aiLinkRow(btns ...gui.View) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		Content: btns,
	}))
}

func aiMCPActions(d *Deps) gui.View {
	return aiLinkRow(
		gui.Button(gui.ButtonCfg{
			ID:      "settings-ai-mcp-open-config",
			Variant: gui.ButtonGhost,
			Label:   i18n.T("settings.ai.open_config"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				if err := d.Agent.OpenMCPConfig(); err != nil {
					toastErr(ctx.Window, err)
				}
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID:      "settings-ai-mcp-refresh",
			Variant: gui.ButtonGhost,
			Label:   i18n.T("common.refresh"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				w := ctx.Window
				refreshLists(gui.State[State](w), d)
				go func() {
					_, err := d.Agent.ReloadMCPServers()
					w.QueueCommand(func(win *gui.Window) {
						if err != nil {
							toastErr(win, err)
						}
					})
				}()
			},
		}),
	)
}

func aiSkillsActions(d *Deps) gui.View {
	return aiLinkRow(
		gui.Button(gui.ButtonCfg{
			ID:      "settings-ai-skills-open-dir",
			Variant: gui.ButtonGhost,
			Label:   i18n.T("settings.ai.open_dir"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				if err := d.Agent.OpenSkillsDir(); err != nil {
					toastErr(ctx.Window, err)
				}
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID:      "settings-ai-skills-refresh",
			Variant: gui.ButtonGhost,
			Label:   i18n.T("common.refresh"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				w := ctx.Window
				refreshLists(gui.State[State](w), d)
				go func() {
					err := d.Agent.ReloadSkills()
					w.QueueCommand(func(win *gui.Window) {
						if err != nil {
							toastErr(win, err)
						}
					})
				}()
			},
		}),
	)
}

func saveAI(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	incoming := st.AI
	mcpKeyCleared := st.MCPKeyCleared
	needsSecret := strings.TrimSpace(incoming.APIKey) != "" || strings.TrimSpace(incoming.WebSearch.APIKey) != ""
	run := func() {
		st.Busy = true
		draft := incoming
		prev := d.Store.Config.AI
		go func() {
			cfg, err := applyAISecrets(d, draft, prev)
			if err == nil {
				cfg.Filesystem.AllowedDirs = tool.SanitizeAllowedDirs(cfg.Filesystem.AllowedDirs)
				if !tool.ValidSearchProvider(cfg.WebSearch.Provider) {
					cfg.WebSearch.Provider = ""
					cfg.WebSearch.APIKey = ""
					cfg.WebSearch.APIKeyEncrypted = ""
				}
				// MCP access key is a bearer token: persist only its hash.
				switch {
				case strings.TrimSpace(cfg.MCPServer.AccessKey) != "":
					cfg.MCPServer.AccessKeyHash = mcpserver.HashAccessKey(cfg.MCPServer.AccessKey)
				case mcpKeyCleared:
					cfg.MCPServer.AccessKeyHash = ""
				case cfg.MCPServer.AccessKeyHash == "":
					cfg.MCPServer.AccessKeyHash = prev.MCPServer.AccessKeyHash
				}
				cfg.MCPServer.AccessKey = ""
				cfg.MCPServer.Normalize()
				d.Store.Config.AI = cfg
				err = d.Store.Save()
			}
			w.QueueCommand(func(win *gui.Window) {
				s := gui.State[State](win)
				s.Busy = false
				if err != nil {
					toastErr(win, err)
					return
				}
				cfg.APIKey = ""
				cfg.WebSearch.APIKey = ""
				s.MCPKeyCleared = false
				s.AI = cfg
				if d.Ctrl != nil {
					d.Ctrl.AgentReloadConfig()
				}
				if d.Agent != nil {
					go func() {
						d.Agent.SyncMCPServer()
						win.QueueCommand(func(w2 *gui.Window) {
							gui.State[State](w2).MCPStatus = d.Agent.MCPServerStatus()
						})
					}()
				}
				toastOK(win, i18n.T("settings.ai.save_success"))
			})
		}()
	}
	if needsSecret {
		withVault(w, d, i18n.T("password.encrypt_api_key"), run)
		return
	}
	run()
}

func applyAISecrets(d *Deps, cfg, prev ai.Config) (ai.Config, error) {
	if strings.TrimSpace(cfg.APIKey) != "" {
		enc, err := encryptOrKeep(d, cfg.APIKey, "", prev.APIKeyEncrypted)
		if err != nil {
			return cfg, err
		}
		cfg.APIKeyEncrypted = enc
		cfg.APIKey = ""
	} else if cfg.APIKeyEncrypted == "" {
		cfg.APIKeyEncrypted = prev.APIKeyEncrypted
	}
	if strings.TrimSpace(cfg.WebSearch.APIKey) != "" {
		enc, err := encryptOrKeep(d, cfg.WebSearch.APIKey, "", prev.WebSearch.APIKeyEncrypted)
		if err != nil {
			return cfg, err
		}
		cfg.WebSearch.APIKeyEncrypted = enc
		cfg.WebSearch.APIKey = ""
	} else if cfg.WebSearch.APIKeyEncrypted == "" && cfg.WebSearch.Provider == prev.WebSearch.Provider {
		cfg.WebSearch.APIKeyEncrypted = prev.WebSearch.APIKeyEncrypted
	}
	return cfg, nil
}
