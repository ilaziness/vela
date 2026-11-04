package ai

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	genkitAI "github.com/firebase/genkit/go/ai"
	genkitTool "github.com/firebase/genkit/go/ai/tool"

	"github.com/ilaziness/vela/internal/ai/tool"
)

func mcpWrapToolParams(t *testing.T, callID string) *genkitAI.ToolParams {
	t.Helper()
	tl := genkitAI.NewTool("mcp__srv__t", "test", func(_ *genkitAI.ToolContext, _ map[string]any) (string, error) {
		return "", nil
	})
	return &genkitAI.ToolParams{
		Tool: tl,
		Request: &genkitAI.ToolRequest{
			Ref:   callID,
			Name:  "mcp__srv__t",
			Input: map[string]any{},
		},
	}
}

func TestWrapToolCallFailEmitsOutputAvailable(t *testing.T) {
	t.Parallel()
	var events []StreamEvent
	ctx := tool.WithDeps(context.Background(), tool.Deps{
		Emit: func(ev StreamEvent) { events = append(events, ev) },
	})
	failMsg := "additional input required (not supported)"
	_, err := wrapToolCall(ctx, mcpWrapToolParams(t, "call-1"), func(context.Context, *genkitAI.ToolParams) (*genkitAI.MultipartToolResponse, error) {
		return nil, genkitTool.Fail(ctx, errors.New(failMsg))
	})
	if !genkitAI.IsToolFailError(err) {
		t.Fatalf("want ToolFailError, got %v", err)
	}
	var sawAvailable, sawHardError bool
	for _, ev := range events {
		switch ev.Type {
		case EventToolOutputAvailable:
			sawAvailable = true
			var payload map[string]any
			if err := json.Unmarshal(ev.Output, &payload); err != nil {
				t.Fatalf("output json: %v", err)
			}
			if payload["error"] != failMsg {
				t.Fatalf("payload=%#v", payload)
			}
		case EventToolOutputError:
			sawHardError = true
		}
	}
	if !sawAvailable {
		t.Fatalf("expected tool-output-available, events=%+v", events)
	}
	if sawHardError {
		t.Fatalf("tool.Fail must not emit tool-output-error, events=%+v", events)
	}
}

func TestWrapToolCallHardErrorEmitsOutputError(t *testing.T) {
	t.Parallel()
	var events []StreamEvent
	ctx := tool.WithDeps(context.Background(), tool.Deps{
		Emit: func(ev StreamEvent) { events = append(events, ev) },
	})
	_, err := wrapToolCall(ctx, mcpWrapToolParams(t, "call-2"), func(context.Context, *genkitAI.ToolParams) (*genkitAI.MultipartToolResponse, error) {
		return nil, errors.New("transport down")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var sawHardError bool
	for _, ev := range events {
		if ev.Type == EventToolOutputError {
			sawHardError = true
		}
		if ev.Type == EventToolOutputAvailable {
			t.Fatalf("hard error must not emit available, events=%+v", events)
		}
	}
	if !sawHardError {
		t.Fatalf("expected tool-output-error, events=%+v", events)
	}
}
