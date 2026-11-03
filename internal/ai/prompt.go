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
func fallbackSystemPrompt(opts PromptOptions, uiLang string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Current time: %s.\n\n", time.Now().Format("2006-01"))
	b.WriteString("You are Vela's built-in AI assistant (Agent).\n")
	fmt.Fprintf(&b, "Respond in the user's UI language: %s.\n", uiLang)
	if opts.EnableSSH {
		b.WriteString("SSH tools are enabled (run_ssh_command).\n")
	}
	if opts.EnableFS {
		b.WriteString("Local filesystem tools are enabled.\n")
	}
	if opts.EnableWebSearch {
		b.WriteString("web_search is enabled.\n")
	}
	return b.String()
}

func appendAllowDirsSection(b *strings.Builder, opts PromptOptions) {
	if !opts.EnableFS || len(opts.AllowDirs) == 0 {
		return
	}
	b.WriteString("\n\n## Local file allowlist\n")
	for _, d := range opts.AllowDirs {
		fmt.Fprintf(b, "- %s\n", d)
	}
}

func appendSSHContextSection(b *strings.Builder, opts PromptOptions) {
	if !isValidSSHContext(opts.SSH) || !opts.EnableSSH {
		b.WriteString(`

## Current SSH context
The user has no usable SSH connection (not connected, connecting, or disconnected). Do not call run_ssh_command; do not assume hosts mentioned earlier in chat history are still reachable. Provide general Linux/Unix and SSH troubleshooting help. upsert_plan remains available.`)
		return
	}
	b.WriteString(`

## Current SSH context
Connected to a remote host; run_ssh_command is available (user approval required). Treat the user's questions as related to this environment by default.
Use the connection in this system prompt as authoritative; if chat history mentions other hosts, do not assume commands still run there.
`)
	fmt.Fprintf(b, "- Connection: %s@%s:%d\n", opts.SSH.User, opts.SSH.Host, opts.SSH.Port)

	remote := opts.Remote
	if remote != nil && remote.Ready && remote.HasContent() {
		b.WriteString("\n## Remote system info\n")
		if remote.Hostname != "" {
			fmt.Fprintf(b, "- Hostname: %s\n", remote.Hostname)
		}
		if remote.OSPretty != "" || remote.OSID != "" || remote.OSVersion != "" {
			fmt.Fprintf(b, "- OS: %s (%s %s)\n", remote.OSPretty, remote.OSID, remote.OSVersion)
		}
		if remote.Kernel != "" {
			fmt.Fprintf(b, "- Kernel: %s\n", remote.Kernel)
		}
		if remote.Arch != "" {
			fmt.Fprintf(b, "- Arch: %s\n", remote.Arch)
		}
		fmt.Fprintf(b, "\nThe above came from a one-shot probe of host %s; confirm details not listed here with the user.", opts.SSH.Host)
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
	b.WriteString("Authoritative plan snapshot (this section wins). Changes require another upsert_plan call.\n")
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
