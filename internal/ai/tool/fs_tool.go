package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/ilaziness/vela/internal/ai/event"
)

type readLocalInput struct {
	Path string `json:"path" jsonschema_description:"Absolute or relative path to a file under an allowed directory."`
}

type writeLocalInput struct {
	Path    string `json:"path" jsonschema_description:"Absolute or relative path to write under an allowed directory."`
	Content string `json:"content" jsonschema_description:"Full file contents to write (overwrite)."`
}

type listLocalInput struct {
	Path string `json:"path" jsonschema_description:"Absolute or relative path to a directory under an allowed directory."`
}

type fsApprovalPayload struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

func defineFSTools(g *genkit.Genkit) (read, write, list genkitAI.ToolRef) {
	read = genkit.DefineTool(g, ReadLocalFile,
		"Read a local file from the user's machine. Path must be under a configured allowed directory. Requires user approval.",
		func(tc *genkitAI.ToolContext, input readLocalInput) (string, error) {
			return runFSApproved(tc, ReadLocalFile, input.Path, "", func(deps Deps, abs string) (string, error) {
				return ReadFile(abs, deps.AllowedDirs, 0)
			})
		})

	write = genkit.DefineTool(g, WriteLocalFile,
		"Write (overwrite/create) a local file on the user's machine under an allowed directory. Requires user approval.",
		func(tc *genkitAI.ToolContext, input writeLocalInput) (string, error) {
			if len(input.Content) > defaultWriteMaxBytes {
				return fmt.Sprintf("error: content too large: %d bytes (max %d)", len(input.Content), defaultWriteMaxBytes), nil
			}
			return runFSApproved(tc, WriteLocalFile, input.Path, input.Content, func(deps Deps, abs string) (string, error) {
				return WriteFile(abs, input.Content, deps.AllowedDirs)
			})
		})

	list = genkit.DefineTool(g, ListLocalDirectory,
		"List entries in a local directory under an allowed directory. Requires user approval.",
		func(tc *genkitAI.ToolContext, input listLocalInput) (string, error) {
			return runFSApproved(tc, ListLocalDirectory, input.Path, "", func(deps Deps, abs string) (string, error) {
				res, err := ListDir(abs, deps.AllowedDirs, 0)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(res)
				return string(b), nil
			})
		})
	return read, write, list
}

func runFSApproved(
	tc *genkitAI.ToolContext,
	toolName, path, content string,
	exec func(Deps, string) (string, error),
) (string, error) {
	deps := FromCtx(tc)
	if !deps.EnableFS() {
		return "local filesystem tools unavailable: no allowed directories configured", nil
	}
	abs, err := ResolveUnderAllowlist(path, deps.AllowedDirs)
	if err != nil {
		return "error: " + err.Error(), nil //nolint:nilerr // soft-fail as tool result for the model
	}

	payload := fsApprovalPayload{Path: abs}
	if toolName == WriteLocalFile {
		payload.Content = content
	}
	inJSON, _ := json.Marshal(payload)
	callID := uuid.New().String()
	approvalID := callID

	emit(deps, event.StreamEvent{Type: event.EventStartStep})
	emit(deps, event.StreamEvent{Type: event.EventToolInputStart, ToolCallID: callID, ToolName: toolName})
	emit(deps, event.StreamEvent{Type: event.EventToolInputAvailable, ToolCallID: callID, ToolName: toolName, Input: inJSON})

	deps.Approval.Open(approvalID)
	emit(deps, event.StreamEvent{
		Type: event.EventToolApprovalRequest, ToolCallID: callID, ToolName: toolName,
		ApprovalID: approvalID, Input: inJSON,
	})

	approved, reason, waitErr := deps.Approval.Wait(tc, approvalID)
	if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
		emit(deps, event.StreamEvent{Type: event.EventToolOutputError, ToolCallID: callID, ToolName: toolName, ErrorText: waitErr.Error()})
		emit(deps, event.StreamEvent{Type: event.EventFinishStep})
		return fmt.Sprintf("approval wait failed: %v", waitErr), nil
	}
	if !approved || errors.Is(waitErr, context.Canceled) {
		if reason == "" {
			if errors.Is(waitErr, context.Canceled) {
				reason = "stopped"
			} else {
				reason = "user denied"
			}
		}
		emit(deps, event.StreamEvent{
			Type: event.EventToolOutputDenied, ToolCallID: callID, ToolName: toolName,
			ApprovalID: approvalID, Reason: reason,
		})
		emit(deps, event.StreamEvent{Type: event.EventFinishStep})
		return fmt.Sprintf("User denied tool execution: %s", reason), nil
	}

	out, execErr := exec(deps, abs)
	if execErr != nil {
		msg := execErr.Error()
		emit(deps, event.StreamEvent{Type: event.EventToolOutputError, ToolCallID: callID, ToolName: toolName, ErrorText: msg})
		emit(deps, event.StreamEvent{Type: event.EventFinishStep})
		return "error: " + msg, nil //nolint:nilerr // soft-fail as tool result for the model
	}
	outJSON, _ := json.Marshal(map[string]string{"output": out})
	emit(deps, event.StreamEvent{Type: event.EventToolOutputAvailable, ToolCallID: callID, ToolName: toolName, Output: outJSON})
	emit(deps, event.StreamEvent{Type: event.EventFinishStep})
	return out, nil
}
