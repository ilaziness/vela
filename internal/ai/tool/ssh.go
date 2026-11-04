package tool

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	genkitAI "github.com/firebase/genkit/go/ai"
	genkitTool "github.com/firebase/genkit/go/ai/tool"
	"github.com/firebase/genkit/go/genkit"
)

const (
	msgUserStoppedCommand = "user stopped command execution"
	msgGenerationStopped  = "generation stopped"
	echoMaxOut            = 16 * 1024
)

type sshCommandInput struct {
	Command string `json:"command" jsonschema_description:"A single short shell command (prefer under 300 characters, one line, no unescaped quotes or newlines). Do not pack multiple probes into one command."`
}

func defineSSHTool(g *genkit.Genkit) genkitAI.ToolRef {
	return genkit.DefineTool(g, RunSSHCommand,
		"Run a shell command on the user's currently connected SSH host. Requires user approval before execution. Output is captured and returned; it does not use the interactive terminal PTY.",
		func(tc *genkitAI.ToolContext, input sshCommandInput) (string, error) {
			deps := FromCtx(tc)
			if !deps.EnableSSH() {
				return "", genkitTool.Fail(tc, errors.New("SSH tools unavailable: no active SSH session"))
			}
			cmd := strings.TrimSpace(input.Command)
			if cmd == "" {
				return "", genkitTool.Fail(tc, errors.New("empty command"))
			}
			if len(cmd) > 800 {
				return "", genkitTool.Fail(tc, errors.New("command too long; call run_ssh_command once per short command"))
			}

			if deps.ArmSSHExecCancel != nil {
				deps.ArmSSHExecCancel()
			}
			echoCommand(tc, deps, cmd)
			out, execErr := deps.SSH.Exec(tc, deps.LinkID, cmd)
			if errors.Is(execErr, context.Canceled) {
				return sshCanceledResult(tc, deps, out), nil
			}
			echoResult(tc, deps, out, execErr)
			if execErr != nil {
				msg := strings.TrimRight(out, "\r\n")
				if msg != "" {
					msg += "\n"
				}
				msg += "exit/error: " + execErr.Error()
				return "", genkitTool.Fail(tc, errors.New(msg))
			}
			if out == "" {
				out = "(no output)"
			}
			return out, nil
		})
}

func sshCanceledResult(tc *genkitAI.ToolContext, deps Deps, out string) string {
	if tc.Err() != nil {
		echoResult(tc, deps, out, errors.New(msgGenerationStopped))
		return msgGenerationStopped
	}
	echoResult(tc, deps, out, errors.New(msgUserStoppedCommand))
	return msgUserStoppedCommand
}

func echoCommand(tc *genkitAI.ToolContext, deps Deps, cmd string) {
	if !deps.Echo || deps.SSH == nil {
		return
	}
	_ = deps.SSH.Annotate(tc, deps.LinkID, "\r\n\x1b[90m[AI]\x1b[0m $ "+cmd+"\r\n")
}

func echoResult(tc *genkitAI.ToolContext, deps Deps, out string, execErr error) {
	if !deps.Echo || deps.SSH == nil {
		return
	}
	var b strings.Builder
	body := strings.TrimRight(out, "\r\n")
	if body == "" {
		b.WriteString("\x1b[90m(no output)\x1b[0m\r\n")
	} else {
		b.WriteString("\x1b[90m")
		b.WriteString(toCRLF(truncateEcho(body, echoMaxOut)))
		b.WriteString("\x1b[0m\r\n")
	}
	if execErr != nil {
		b.WriteString("\x1b[31m[AI] exit/error: ")
		b.WriteString(execErr.Error())
		b.WriteString("\x1b[0m\r\n")
	}
	_ = deps.SSH.Annotate(tc, deps.LinkID, b.String())
}

func truncateEcho(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	s = s[:maxLen]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s + "\n…[truncated]"
}

func toCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
