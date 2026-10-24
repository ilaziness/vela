package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/ilaziness/vela/internal/ai/event"
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
				return "SSH tools unavailable: no active SSH session", nil
			}
			cmd := strings.TrimSpace(input.Command)
			if cmd == "" {
				return "error: empty command", nil
			}
			if len(cmd) > 800 {
				return "error: command too long; call run_ssh_command once per short command", nil
			}
			callID := uuid.New().String()
			approvalID := callID
			inJSON, _ := json.Marshal(sshCommandInput{Command: cmd})

			emit(deps, event.StreamEvent{Type: event.EventStartStep})
			emit(deps, event.StreamEvent{Type: event.EventToolInputStart, ToolCallID: callID, ToolName: RunSSHCommand})
			emit(deps, event.StreamEvent{Type: event.EventToolInputAvailable, ToolCallID: callID, ToolName: RunSSHCommand, Input: inJSON})

			deps.Approval.Open(approvalID)
			if deps.MarkSSHApproval != nil {
				deps.MarkSSHApproval(approvalID)
			}
			emit(deps, event.StreamEvent{
				Type: event.EventToolApprovalRequest, ToolCallID: callID, ToolName: RunSSHCommand,
				ApprovalID: approvalID, Input: inJSON,
			})

			approved, reason, err := deps.Approval.Wait(tc, approvalID)
			if err != nil && !errors.Is(err, context.Canceled) {
				emit(deps, event.StreamEvent{Type: event.EventToolOutputError, ToolCallID: callID, ToolName: RunSSHCommand, ErrorText: err.Error()})
				emit(deps, event.StreamEvent{Type: event.EventFinishStep})
				return fmt.Sprintf("approval wait failed: %v", err), nil
			}
			if !approved || errors.Is(err, context.Canceled) {
				if reason == "" {
					if errors.Is(err, context.Canceled) {
						reason = "stopped"
					} else {
						reason = "user denied"
					}
				}
				emit(deps, event.StreamEvent{
					Type: event.EventToolOutputDenied, ToolCallID: callID, ToolName: RunSSHCommand,
					ApprovalID: approvalID, Reason: reason,
				})
				emit(deps, event.StreamEvent{Type: event.EventFinishStep})
				return fmt.Sprintf("User denied command execution: %s", reason), nil
			}

			if deps.ArmSSHExecCancel != nil {
				deps.ArmSSHExecCancel()
			}
			echoCommand(tc, deps, cmd)
			out, execErr := deps.SSH.Exec(tc, deps.LinkID, cmd)
			if errors.Is(execErr, context.Canceled) {
				return emitSSHCanceled(tc, deps, callID, out), nil
			}
			echoResult(tc, deps, out, execErr)
			return finishSSH(deps, callID, out, execErr), nil
		})
}

func emitSSHCanceled(tc *genkitAI.ToolContext, deps Deps, callID, out string) string {
	if tc.Err() != nil {
		echoResult(tc, deps, out, errors.New(msgGenerationStopped))
		emit(deps, event.StreamEvent{Type: event.EventToolOutputError, ToolCallID: callID, ToolName: RunSSHCommand, ErrorText: msgGenerationStopped})
		emit(deps, event.StreamEvent{Type: event.EventFinishStep})
		return msgGenerationStopped
	}
	echoResult(tc, deps, out, errors.New(msgUserStoppedCommand))
	outJSON, _ := json.Marshal(map[string]string{"output": msgUserStoppedCommand})
	emit(deps, event.StreamEvent{Type: event.EventToolOutputAvailable, ToolCallID: callID, ToolName: RunSSHCommand, Output: outJSON})
	emit(deps, event.StreamEvent{Type: event.EventFinishStep})
	return msgUserStoppedCommand
}

func finishSSH(deps Deps, callID, out string, execErr error) string {
	if execErr != nil {
		msg := strings.TrimRight(out, "\r\n")
		if msg != "" {
			msg += "\n"
		}
		msg += "exit/error: " + execErr.Error()
		emit(deps, event.StreamEvent{Type: event.EventToolOutputError, ToolCallID: callID, ToolName: RunSSHCommand, ErrorText: msg})
		emit(deps, event.StreamEvent{Type: event.EventFinishStep})
		return msg
	}
	if out == "" {
		out = "(no output)"
	}
	outJSON, _ := json.Marshal(map[string]string{"output": out})
	emit(deps, event.StreamEvent{Type: event.EventToolOutputAvailable, ToolCallID: callID, ToolName: RunSSHCommand, Output: outJSON})
	emit(deps, event.StreamEvent{Type: event.EventFinishStep})
	return out
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
