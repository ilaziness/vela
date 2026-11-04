package ai

import (
	"context"
	"encoding/json"

	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"

	"github.com/ilaziness/vela/internal/ai/tool"
)

// toolStreamMiddleware emits MUI-aligned tool stream events around actual tool execution.
// Approval gating is handled by Genkit middleware.ToolApproval (interrupt / resume).
type toolStreamMiddleware struct{}

func (t toolStreamMiddleware) Name() string { return "vela/tool-stream" }

func (t toolStreamMiddleware) New(ctx context.Context) (*genkitAI.Hooks, error) {
	_ = ctx
	return &genkitAI.Hooks{
		WrapTool: wrapToolCall,
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
	deps := tool.FromCtx(ctx)
	callID := toolRequestRef(params.Request)
	if callID == "" {
		callID = uuid.New().String()
	}
	inJSON := toolInputJSON(params)

	emitDeps(deps, StreamEvent{Type: EventStartStep})
	emitDeps(deps, StreamEvent{Type: EventToolInputStart, ToolCallID: callID, ToolName: name})
	emitDeps(deps, StreamEvent{Type: EventToolInputAvailable, ToolCallID: callID, ToolName: name, Input: inJSON})

	resp, err := next(ctx, params)
	if err != nil {
		if genkitAI.IsToolFailError(err) {
			outJSON, _ := json.Marshal(map[string]any{"error": err.Error()})
			emitDeps(deps, StreamEvent{Type: EventToolOutputAvailable, ToolCallID: callID, ToolName: name, Output: outJSON})
			emitDeps(deps, StreamEvent{Type: EventFinishStep})
			return resp, err
		}
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

func emitDeps(deps tool.Deps, ev StreamEvent) {
	if deps.Emit != nil {
		deps.Emit(ev)
	}
}
