package ai

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
)

const (
	ToolRunSSHCommand = "run_ssh_command"
	ToolUpsertPlan    = "upsert_plan"

	msgUserStoppedCommand = "user stopped command execution"
	msgGenerationStopped  = "generation stopped"
)

type sshCommandInput struct {
	Command string `json:"command" jsonschema_description:"A single short shell command (prefer under 300 characters, one line, no unescaped quotes or newlines). Do not pack multiple probes into one command."`
}

type planTask struct {
	ID     string `json:"id" jsonschema_description:"Stable task id within the plan."`
	Title  string `json:"title" jsonschema_description:"Short task title."`
	Status string `json:"status" jsonschema_description:"One of: pending, running, done, failed."`
}

type upsertPlanInput struct {
	Title string     `json:"title" jsonschema_description:"Plan title."`
	Tasks []planTask `json:"tasks" jsonschema_description:"Full snapshot of tasks (replace previous plan)."`
}

type agentTools struct {
	plan genkitAI.ToolRef
	ssh  genkitAI.ToolRef
}

func registerTools(g *genkit.Genkit) agentTools {
	upsert := genkit.DefineTool(g, ToolUpsertPlan,
		"Create or replace the current multi-step plan shown in the chat. Always send the full task list snapshot.",
		func(tc *genkitAI.ToolContext, input upsertPlanInput) (upsertPlanInput, error) {
			normalizePlan(&input)
			callID := uuid.New().String()
			inJSON, _ := json.Marshal(input)
			emit(tc, StreamEvent{Type: EventStartStep})
			emit(tc, StreamEvent{Type: EventToolInputStart, ToolCallID: callID, ToolName: ToolUpsertPlan})
			emit(tc, StreamEvent{Type: EventToolInputAvailable, ToolCallID: callID, ToolName: ToolUpsertPlan, Input: inJSON})
			emit(tc, StreamEvent{Type: EventToolOutputAvailable, ToolCallID: callID, ToolName: ToolUpsertPlan, Output: inJSON})
			emit(tc, StreamEvent{Type: EventFinishStep})
			return input, nil
		})

	runSSH := genkit.DefineTool(g, ToolRunSSHCommand,
		"Run a shell command on the user's currently connected SSH host. Requires user approval before execution. Output is captured and returned; it does not use the interactive terminal PTY.",
		func(tc *genkitAI.ToolContext, input sshCommandInput) (string, error) {
			deps := depsFromCtx(tc)
			if !deps.EnableSSHTool() {
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

			emit(tc, StreamEvent{Type: EventStartStep})
			emit(tc, StreamEvent{Type: EventToolInputStart, ToolCallID: callID, ToolName: ToolRunSSHCommand})
			emit(tc, StreamEvent{Type: EventToolInputAvailable, ToolCallID: callID, ToolName: ToolRunSSHCommand, Input: inJSON})

			deps.Approval.Open(approvalID)
			if deps.MarkSSHApproval != nil {
				deps.MarkSSHApproval(approvalID)
			}
			emit(tc, StreamEvent{
				Type: EventToolApprovalRequest, ToolCallID: callID, ToolName: ToolRunSSHCommand,
				ApprovalID: approvalID, Input: inJSON,
			})

			approved, reason, err := deps.Approval.Wait(tc, approvalID)
			if err != nil && !errors.Is(err, context.Canceled) {
				emit(tc, StreamEvent{Type: EventToolOutputError, ToolCallID: callID, ToolName: ToolRunSSHCommand, ErrorText: err.Error()})
				emit(tc, StreamEvent{Type: EventFinishStep})
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
				emit(tc, StreamEvent{
					Type: EventToolOutputDenied, ToolCallID: callID, ToolName: ToolRunSSHCommand,
					ApprovalID: approvalID, Reason: reason,
				})
				emit(tc, StreamEvent{Type: EventFinishStep})
				// Keep "User denied command execution:" prefix for history denial detection.
				return fmt.Sprintf("User denied command execution: %s", reason), nil
			}

			if deps.ArmSSHExecCancel != nil {
				deps.ArmSSHExecCancel()
			}
			echoCommand(tc, deps, cmd)
			out, execErr := deps.SSH.Exec(tc, deps.LinkID, cmd)
			if errors.Is(execErr, context.Canceled) {
				// Intentional: cancel is a tool result string for Genkit, not a tool error.
				return emitSSHCanceled(tc, deps, callID, out), nil
			}
			echoResult(tc, deps, out, execErr)
			return finishSSH(tc, callID, out, execErr), nil
		})

	return agentTools{plan: upsert, ssh: runSSH}
}

const echoMaxOut = 16 * 1024

func emitSSHCanceled(tc *genkitAI.ToolContext, deps ToolDeps, callID, out string) string {
	// Per-command Stop: keep agent alive so the model can continue (UI ≠ 失败).
	// Generation Stop: tc is also cancelled — still return a clean string.
	if tc.Err() != nil {
		echoResult(tc, deps, out, errors.New(msgGenerationStopped))
		emit(tc, StreamEvent{Type: EventToolOutputError, ToolCallID: callID, ToolName: ToolRunSSHCommand, ErrorText: msgGenerationStopped})
		emit(tc, StreamEvent{Type: EventFinishStep})
		return msgGenerationStopped
	}
	echoResult(tc, deps, out, errors.New(msgUserStoppedCommand))
	outJSON, _ := json.Marshal(map[string]string{"output": msgUserStoppedCommand})
	emit(tc, StreamEvent{Type: EventToolOutputAvailable, ToolCallID: callID, ToolName: ToolRunSSHCommand, Output: outJSON})
	emit(tc, StreamEvent{Type: EventFinishStep})
	return msgUserStoppedCommand
}

func finishSSH(tc *genkitAI.ToolContext, callID, out string, execErr error) string {
	if execErr != nil {
		msg := strings.TrimRight(out, "\r\n")
		if msg != "" {
			msg += "\n"
		}
		msg += "exit/error: " + execErr.Error()
		emit(tc, StreamEvent{Type: EventToolOutputError, ToolCallID: callID, ToolName: ToolRunSSHCommand, ErrorText: msg})
		emit(tc, StreamEvent{Type: EventFinishStep})
		return msg
	}
	if out == "" {
		out = "(no output)"
	}
	outJSON, _ := json.Marshal(map[string]string{"output": out})
	emit(tc, StreamEvent{Type: EventToolOutputAvailable, ToolCallID: callID, ToolName: ToolRunSSHCommand, Output: outJSON})
	emit(tc, StreamEvent{Type: EventFinishStep})
	return out
}

func echoCommand(tc *genkitAI.ToolContext, deps ToolDeps, cmd string) {
	if !deps.Echo || deps.SSH == nil {
		return
	}
	_ = deps.SSH.Annotate(tc, deps.LinkID, "\r\n\x1b[90m[AI]\x1b[0m $ "+cmd+"\r\n")
}

func echoResult(tc *genkitAI.ToolContext, deps ToolDeps, out string, execErr error) {
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

func emit(tc *genkitAI.ToolContext, ev StreamEvent) {
	deps := depsFromCtx(tc)
	if deps.Emit != nil {
		deps.Emit(ev)
	}
}

func normalizePlan(p *upsertPlanInput) {
	p.Title = strings.TrimSpace(p.Title)
	for i := range p.Tasks {
		p.Tasks[i].ID = strings.TrimSpace(p.Tasks[i].ID)
		p.Tasks[i].Title = strings.TrimSpace(p.Tasks[i].Title)
		st := strings.ToLower(strings.TrimSpace(p.Tasks[i].Status))
		switch st {
		case "pending", "running", "done", "failed":
			p.Tasks[i].Status = st
		default:
			p.Tasks[i].Status = "pending"
		}
		if p.Tasks[i].ID == "" {
			p.Tasks[i].ID = uuid.New().String()
		}
	}
}
