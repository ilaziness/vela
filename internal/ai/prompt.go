package ai

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/dotprompt/go/dotprompt"

	"github.com/ilaziness/vela/internal/ai/tool"
	"github.com/ilaziness/vela/internal/ssh"
)

//go:embed prompts/*
var promptFS embed.FS

// SSHPromptContext SSH connection context for the system prompt.
type SSHPromptContext struct {
	Host string
	Port int
	User string
}

// PromptOptions controls which capability sections are included in the system prompt.
type PromptOptions struct {
	SSH             *SSHPromptContext
	Remote          *ssh.RemoteSystemInfo
	EnableSSH       bool
	EnableFS        bool
	AllowDirs       []string
	EnableWebSearch bool
	PlanJSON        string // authoritative session plan snapshot; empty skips Current plan
	UILanguage      string // en / zh-CN / zh-TW
}

var (
	agentPromptOnce sync.Once
	agentPromptFn   dotprompt.PromptFunction
	agentPromptErr  error
)

func agentSystemPrompt() (dotprompt.PromptFunction, error) {
	agentPromptOnce.Do(func() {
		src, err := promptFS.ReadFile("prompts/agent_system.prompt")
		if err != nil {
			agentPromptErr = err
			return
		}
		dp := dotprompt.NewDotprompt(nil)
		agentPromptFn, agentPromptErr = dp.Compile(string(src), nil)
	})
	return agentPromptFn, agentPromptErr
}

// BuildSystemPromptOpts assembles the system prompt from the embedded Dotprompt
// static body plus Go-formatted dynamic sections (SSH/remote/dirs/plan).
func BuildSystemPromptOpts(opts PromptOptions) string {
	uiLang := strings.TrimSpace(opts.UILanguage)
	if uiLang == "" {
		uiLang = "en"
	}
	fn, err := agentSystemPrompt()
	var body string
	if err != nil || fn == nil {
		body = fallbackSystemPrompt(opts, uiLang)
	} else {
		rendered, renderErr := fn(&dotprompt.DataArgument{
			Input: map[string]any{
				"now":             time.Now().Format("2006-01"),
				"uiLanguage":      uiLang,
				"enableSSH":       opts.EnableSSH,
				"enableFS":        opts.EnableFS,
				"enableWebSearch": opts.EnableWebSearch,
			},
		}, nil)
		if renderErr != nil {
			body = fallbackSystemPrompt(opts, uiLang)
		} else {
			body = renderedSystemText(rendered)
		}
	}

	var b strings.Builder
	b.WriteString(strings.TrimSpace(body))
	appendAllowDirsSection(&b, opts)
	appendSSHContextSection(&b, opts)
	appendCurrentPlanSection(&b, opts.PlanJSON)
	return b.String()
}

func renderedSystemText(rendered dotprompt.RenderedPrompt) string {
	var parts []string
	for _, msg := range rendered.Messages {
		for _, part := range msg.Content {
			if tp, ok := part.(*dotprompt.TextPart); ok {
				if t := strings.TrimSpace(tp.Text); t != "" {
					parts = append(parts, t)
				}
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// fallbackSystemPrompt keeps Chat working if Dotprompt compile/render fails.
// It is a compact copy of the hard rules in agent_system.prompt (scope, tools, security).
func fallbackSystemPrompt(opts PromptOptions, uiLang string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Current time: %s.\n\n", time.Now().Format("2006-01"))
	b.WriteString("You are Vela's built-in AI assistant (Agent) in an SSH desktop client sidebar.\n")
	fmt.Fprintf(&b, "Respond in the user's UI language: %s (including refusals).\n", uiLang)
	b.WriteString("Only answer operations-related questions (Linux/Unix, SSH, servers, networking, processes, services, logs, files, scripts, configs). If a message mixes topics, answer only the operations part. If unrelated, do not answer it, do not use tools, and reply briefly that you cannot answer.\n")
	b.WriteString("upsert_plan: for ≥2 steps or probe→decide→execute, submit a full snapshot; empty tasks clears the plan. Mark a step running before work and done/failed as soon as that step finishes.\n")
	if opts.EnableSSH {
		b.WriteString("run_ssh_command: user approval required; one short command per call (preferably ≤300 characters, single-line JSON string); prefer read-only probes; call out risk before destructive commands.\n")
	}
	if opts.EnableFS {
		b.WriteString("read_local_file / write_local_file / list_local_directory: stay in the allowlisted directories; each call needs chat approval; writes overwrite — state intent first.\n")
	}
	if opts.EnableWebSearch {
		b.WriteString("web_search: only to support an operations question; cite returned URLs; do not invent links.\n")
	}
	b.WriteString("Do not invent facts. Never request, output, or guess passwords, private keys, API keys, tokens, or credentials; do not read .env or key material to obtain them; do not log into databases or other services for the user.\n")
	return b.String()
}

func appendAllowDirsSection(b *strings.Builder, opts PromptOptions) {
	if !opts.EnableFS || len(opts.AllowDirs) == 0 {
		return
	}
	var dirs []string
	for _, d := range opts.AllowDirs {
		if d = strings.TrimSpace(d); d != "" {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return
	}
	b.WriteString("\n\n## Local file allowlist\n")
	b.WriteString("File tools may only use paths under these directories:\n")
	for _, d := range dirs {
		fmt.Fprintf(b, "- %s\n", d)
	}
}

func appendSSHContextSection(b *strings.Builder, opts PromptOptions) {
	if !isValidSSHContext(opts.SSH) || !opts.EnableSSH {
		b.WriteString(`

## Current SSH context
No usable SSH session. Do not call run_ssh_command. Do not assume hosts from earlier chat are reachable. General Linux/Unix and SSH advice is still in scope; upsert_plan remains available.`)
		return
	}
	b.WriteString(`

## Current SSH context
Connected; run_ssh_command is available (user approval required). Operations questions default to this host. This connection is authoritative — do not assume commands still run on hosts mentioned only in chat history.
`)
	fmt.Fprintf(b, "- Connection: %s@%s:%d\n", opts.SSH.User, opts.SSH.Host, opts.SSH.Port)

	remote := opts.Remote
	if remote != nil && remote.Ready && remote.HasContent() {
		b.WriteString("\n## Remote system info\n")
		if remote.Hostname != "" {
			fmt.Fprintf(b, "- Hostname: %s\n", remote.Hostname)
		}
		if osLine := formatRemoteOS(remote.OSPretty, remote.OSID, remote.OSVersion); osLine != "" {
			fmt.Fprintf(b, "- OS: %s\n", osLine)
		}
		if remote.Kernel != "" {
			fmt.Fprintf(b, "- Kernel: %s\n", remote.Kernel)
		}
		if remote.Arch != "" {
			fmt.Fprintf(b, "- Arch: %s\n", remote.Arch)
		}
		fmt.Fprintf(b, "\nOne-shot probe of %s; confirm anything not listed here.", opts.SSH.Host)
	}
}

func formatRemoteOS(pretty, id, version string) string {
	pretty = strings.TrimSpace(pretty)
	extra := strings.TrimSpace(strings.TrimSpace(id) + " " + strings.TrimSpace(version))
	switch {
	case pretty != "" && extra != "":
		return pretty + " (" + extra + ")"
	case pretty != "":
		return pretty
	default:
		return extra
	}
}

func appendCurrentPlanSection(b *strings.Builder, planJSON string) {
	planJSON = strings.TrimSpace(planJSON)
	if planJSON == "" {
		return
	}
	var snap tool.PlanSnapshot
	if err := json.Unmarshal([]byte(planJSON), &snap); err != nil || len(snap.Tasks) == 0 {
		return
	}
	b.WriteString("\n\n## Current plan\n")
	b.WriteString("Authoritative plan snapshot (this section wins over chat history). Change it only with upsert_plan. If the user's message is unrelated to operations, do not continue this plan.\n")
	fmt.Fprintf(b, "- Title: %s\n", snap.Title)
	b.WriteString("- Tasks:\n")
	for _, t := range snap.Tasks {
		fmt.Fprintf(b, "  - [%s] %s (id=%s)\n", t.Status, t.Title, t.ID)
	}
}

func isValidSSHContext(ctx *SSHPromptContext) bool {
	if ctx == nil {
		return false
	}
	return strings.TrimSpace(ctx.Host) != "" &&
		strings.TrimSpace(ctx.User) != "" &&
		ctx.Port > 0
}
