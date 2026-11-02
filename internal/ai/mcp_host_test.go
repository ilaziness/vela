package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPToolCallOutputText(t *testing.T) {
	t.Parallel()
	out, err := mcpToolCallOutput(context.Background(), &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent("hello")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("got %#v", out)
	}
}

func TestMCPToolCallOutputStructuredPreferred(t *testing.T) {
	t.Parallel()
	out, err := mcpToolCallOutput(context.Background(), &mcp.CallToolResult{
		Content:           []mcp.Content{mcp.NewTextContent("fallback")},
		StructuredContent: map[string]any{"ok": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok || m["ok"] != true {
		t.Fatalf("got %#v", out)
	}
}

func TestMCPToolCallOutputIsError(t *testing.T) {
	t.Parallel()
	_, err := mcpToolCallOutput(context.Background(), &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent("boom")},
		IsError: true,
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("got err=%v", err)
	}
	if !genkitAI.IsToolFailError(err) {
		t.Fatalf("expected ToolFailError, got %T %v", err, err)
	}
}

func TestMCPToolCallOutputNeedsInput(t *testing.T) {
	t.Parallel()
	result := &mcp.CallToolResult{}
	result.ResultType = mcp.ResultTypeInputRequired
	_, err := mcpToolCallOutput(context.Background(), result)
	if err == nil {
		t.Fatal("expected error")
	}
	if !genkitAI.IsToolFailError(err) {
		t.Fatalf("expected ToolFailError, got %T %v", err, err)
	}
}

func TestMCPInputSchemaMapRawPreferred(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"q": map[string]any{"type": "string"},
		},
		"required": []any{"q"},
	})
	got := mcpInputSchemaMap(mcp.Tool{
		InputSchema:    mcp.ToolInputSchema{Type: "object"},
		RawInputSchema: raw,
	})
	req, _ := got["required"].([]any)
	if len(req) != 1 || req[0] != "q" {
		t.Fatalf("got %#v", got)
	}
}

func TestMCPInputSchemaMapNormalizesEmptyType(t *testing.T) {
	t.Parallel()
	got := mcpInputSchemaMap(mcp.Tool{})
	if got["type"] != "object" {
		t.Fatalf("type=%#v", got["type"])
	}
	if got["properties"] == nil {
		t.Fatal("properties missing")
	}
}

func TestMCPNamespacedToolNameTruncatesForGemini(t *testing.T) {
	t.Parallel()
	longServer := strings.Repeat("s", 40)
	longTool := strings.Repeat("t", 40)
	got := mcpNamespacedToolName(longServer, longTool)
	if len(got) > maxMCPToolNameLen {
		t.Fatalf("len=%d name=%q", len(got), got)
	}
	if !strings.HasPrefix(got, "mcp__") {
		t.Fatalf("prefix: %q", got)
	}
	// Stable across calls.
	if again := mcpNamespacedToolName(longServer, longTool); again != got {
		t.Fatalf("unstable: %q vs %q", got, again)
	}
}
