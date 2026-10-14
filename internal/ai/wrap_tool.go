package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
)

// toolStreamMiddleware emits stream events and requires approval for MCP tools
// (everything except SSH/plan/use_skill, which handle themselves or need no approval).
type toolStreamMiddleware struct{}

func (t toolStreamMiddleware) Name() string { return "vela/tool-stream" }

func (t toolStreamMiddleware) New(ctx context.Context) (*genkitAI.Hooks, error) {
	_ = ctx
	return &genkitAI.Hooks{
		WrapTool: func(ctx context.Context, params *genkitAI.ToolParams, next genkitAI.ToolNext) (*genkitAI.MultipartToolResponse, error) {
			return wrapToolCall(ctx, params, next)
		},
	}, nil
}

func wrapToolCall(
	ctx context.Context,
	params *genkitAI.ToolParams,
	next genkitAI.ToolNext,
) (*genkitAI.MultipartToolResponse, error) {
	if params == nil || params.Tool == nil {
		return next(ctx, params)
	}
	name := params.Tool.Name()
	// Built-in tools: SSH/plan emit their own stream events; use_skill needs no approval UI.
	if name == ToolRunSSHCommand || name == ToolUpsertPlan || name == useSkillToolName {
		return next(ctx, params)
	}

	deps := depsFromCtx(ctx)
	callID := uuid.New().String()
	if params.Request != nil && params.Request.Ref != "" {
		callID = params.Request.Ref
	}
	inJSON := toolInputJSON(params)

	emitDeps(deps, StreamEvent{Type: EventStartStep})
	emitDeps(deps, StreamEvent{Type: EventToolInputStart, ToolCallID: callID, ToolName: name})
	emitDeps(deps, StreamEvent{Type: EventToolInputAvailable, ToolCallID: callID, ToolName: name, Input: inJSON})

	// MCP (and any other non-builtin) tools always require user approval.
	if deps.Approval == nil {
		msg := "MCP tool approval unavailable"
		emitDeps(deps, StreamEvent{Type: EventToolOutputError, ToolCallID: callID, ToolName: name, ErrorText: msg})
		emitDeps(deps, StreamEvent{Type: EventFinishStep})
		return &genkitAI.MultipartToolResponse{Output: msg}, nil
	}
	approvalID := callID
	deps.Approval.Open(approvalID)
	emitDeps(deps, StreamEvent{
		Type: EventToolApprovalRequest, ToolCallID: callID, ToolName: name,
		ApprovalID: approvalID, Input: inJSON,
	})
	approved, reason, err := deps.Approval.Wait(ctx, approvalID)
	if err != nil {
		emitDeps(deps, StreamEvent{Type: EventToolOutputError, ToolCallID: callID, ToolName: name, ErrorText: err.Error()})
		emitDeps(deps, StreamEvent{Type: EventFinishStep})
		return &genkitAI.MultipartToolResponse{Output: fmt.Sprintf("approval wait failed: %v", err)}, nil
	}
	if !approved {
		if reason == "" {
			reason = "user denied"
		}
		emitDeps(deps, StreamEvent{
			Type: EventToolOutputDenied, ToolCallID: callID, ToolName: name,
			ApprovalID: approvalID, Reason: reason,
		})
		emitDeps(deps, StreamEvent{Type: EventFinishStep})
		return &genkitAI.MultipartToolResponse{Output: fmt.Sprintf("User denied tool execution: %s", reason)}, nil
	}

	resp, err := next(ctx, params)
	if err != nil {
		emitDeps(deps, StreamEvent{Type: EventToolOutputError, ToolCallID: callID, ToolName: name, ErrorText: err.Error()})
		emitDeps(deps, StreamEvent{Type: EventFinishStep})
		return resp, err
	}
	outJSON, _ := json.Marshal(map[string]any{"output": multipartOutput(resp)})
	emitDeps(deps, StreamEvent{Type: EventToolOutputAvailable, ToolCallID: callID, ToolName: name, Output: outJSON})
	emitDeps(deps, StreamEvent{Type: EventFinishStep})
	return resp, nil
}

func toolInputJSON(params *genkitAI.ToolParams) json.RawMessage {
	if params == nil || params.Request == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(params.Request.Input)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func multipartOutput(resp *genkitAI.MultipartToolResponse) any {
	if resp == nil {
		return nil
	}
	if resp.Output != nil {
		return resp.Output
	}
	return resp
}

func emitDeps(deps ToolDeps, ev StreamEvent) {
	if deps.Emit != nil {
		deps.Emit(ev)
	}
}
