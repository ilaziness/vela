package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"

	genkitAI "github.com/firebase/genkit/go/ai"
)

func TestDepsFromCtxIsolatesRuns(t *testing.T) {
	a := ToolDeps{LinkID: "sess-a"}
	b := ToolDeps{LinkID: "sess-b"}
	ctxA := withToolDeps(context.Background(), a)
	ctxB := withToolDeps(context.Background(), b)
	if got := depsFromCtx(ctxA).LinkID; got != "sess-a" {
		t.Fatalf("ctxA LinkID=%q", got)
	}
	if got := depsFromCtx(ctxB).LinkID; got != "sess-b" {
		t.Fatalf("ctxB LinkID=%q", got)
	}
	if got := depsFromCtx(context.Background()).LinkID; got != "" {
		t.Fatalf("empty ctx LinkID=%q", got)
	}
}

func TestNormalizePlan(t *testing.T) {
	p := upsertPlanInput{Title: "  x  ", Tasks: []planTask{
		{ID: "", Title: "a", Status: "RUNNING"},
		{ID: "t2", Title: "b", Status: "nope"},
	}}
	normalizePlan(&p)
	if p.Title != "x" {
		t.Fatalf("title=%q", p.Title)
	}
	if p.Tasks[0].ID == "" || p.Tasks[0].Status != "running" {
		t.Fatalf("task0=%+v", p.Tasks[0])
	}
	if p.Tasks[1].Status != "pending" {
		t.Fatalf("task1 status=%q", p.Tasks[1].Status)
	}
}

func TestBuildGenkitMessagesKeepsToolTranscript(t *testing.T) {
	reqPart := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{
		Name:  ToolRunSSHCommand,
		Ref:   "c1",
		Input: map[string]any{"command": "uname"},
	})
	respPart := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
		Name:   ToolRunSSHCommand,
		Ref:    "c1",
		Output: "Linux",
	})
	history := []ChatMessage{
		{Role: string(genkitAI.RoleUser), Content: "os?", Parts: UserTextPartsJSON("os?")},
		{Role: string(genkitAI.RoleModel), Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqPart})},
		{Role: string(genkitAI.RoleTool), Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{respPart})},
	}
	msgs := buildGenkitMessages(history, "again", "sys")
	if len(msgs) != 5 {
		t.Fatalf("len=%d", len(msgs))
	}
	if msgs[0].Role != genkitAI.RoleSystem || msgs[1].Role != genkitAI.RoleUser {
		t.Fatalf("prefix roles %q %q", msgs[0].Role, msgs[1].Role)
	}
	if msgs[2].Role != genkitAI.RoleModel || len(msgs[2].Content) == 0 || !msgs[2].Content[0].IsToolRequest() {
		t.Fatalf("model tool request missing: %+v", msgs[2])
	}
	if msgs[3].Role != genkitAI.RoleTool || len(msgs[3].Content) == 0 || !msgs[3].Content[0].IsToolResponse() {
		t.Fatalf("tool response missing: %+v", msgs[3])
	}
	if got := msgs[3].Content[0].ToolResponse.Output; got != "Linux" {
		t.Fatalf("output=%v", got)
	}
	if msgs[4].Role != genkitAI.RoleUser {
		t.Fatalf("new user role=%q", msgs[4].Role)
	}
}

func TestIsNoToolSupport(t *testing.T) {
	if !isNoToolSupport(fmt.Errorf(`model "ollama/gemma4:e2b" does not support tool use, but tools were provided`)) {
		t.Fatal("expected tool-support error")
	}
	if isNoToolSupport(fmt.Errorf("timeout")) {
		t.Fatal("timeout is not a tool-support error")
	}
}

func TestCompleteTurnDropsTrailingToolRequest(t *testing.T) {
	reqPart := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "c1"})
	turn := []ChatMessage{
		{Role: RoleModel, Content: "ok", Parts: UserTextPartsJSON("ok")},
		{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqPart})},
	}
	got := completeTurn(turn)
	if len(got) != 1 || got[0].Content != "ok" {
		t.Fatalf("got=%+v", got)
	}
}

func TestApplyStreamedTextOnCancel(t *testing.T) {
	got := applyStreamedText(&AgentResponse{}, "partial answer")
	if got.Content != "partial answer" || len(got.Turn) != 1 || got.Turn[0].Role != RoleModel {
		t.Fatalf("got=%+v", got)
	}

	reqPart := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "c1"})
	toolOnly := &AgentResponse{
		Content: "[" + ToolRunSSHCommand + "]",
		Turn: []ChatMessage{
			{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqPart})},
		},
	}
	got = applyStreamedText(toolOnly, "before tool")
	if got.Content != "before tool" || !messageHasTextPart(got.Turn[0]) {
		t.Fatalf("tool-only should be filled with streamed text: %+v", got)
	}

	contentOnly := &AgentResponse{Content: "from message text"}
	got = applyStreamedText(contentOnly, "")
	if got.Content != "from message text" || len(got.Turn) != 1 || !messageHasTextPart(got.Turn[0]) {
		t.Fatalf("Content without Turn should become persistable: %+v", got)
	}

	full := &AgentResponse{
		Content: "phase2",
		Turn: []ChatMessage{
			{Role: RoleModel, Content: "phase1", Parts: UserTextPartsJSON("phase1")},
			{Role: RoleModel, Content: "phase2", Parts: UserTextPartsJSON("phase2")},
		},
	}
	got = applyStreamedText(full, "phase1phase2extra")
	if got.Content != "phase2" || len(got.Turn) != 2 || got.Turn[0].Content != "phase1" {
		t.Fatalf("must not overwrite multi-turn model text with full stream: %+v", got)
	}
}

func TestTruncateEcho(t *testing.T) {
	if got := truncateEcho("abc", 10); got != "abc" {
		t.Fatalf("short=%q", got)
	}
	got := truncateEcho("你好世界", 4)
	if !strings.HasSuffix(got, "…[truncated]") {
		t.Fatalf("got %q", got)
	}
}

func TestToCRLF(t *testing.T) {
	if got := toCRLF("a\nb\r\nc\rd"); got != "a\r\nb\r\nc\r\nd" {
		t.Fatalf("got %q", got)
	}
}

func TestPublicGenerateErrorStripsHugeJSON(t *testing.T) {
	err := fmt.Errorf(`could not parse tool args: unmarshal failed to parse json string {"command": "echo \"=== 网关连通性 ===\"; timeout 5 bash -c 'cat': unexpected end of JSON input`)
	got := PublicGenerateError(err)
	if strings.Contains(got, "echo") || strings.Contains(got, "{") {
		t.Fatalf("leaked raw JSON: %q", got)
	}
	if !strings.Contains(got, "JSON") && !strings.Contains(got, "命令") {
		t.Fatalf("got=%q", got)
	}
}

func TestTrimIncompleteToolRound(t *testing.T) {
	reqPart := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "c1"})
	history := []ChatMessage{
		{Role: RoleUser, Content: "run", Parts: UserTextPartsJSON("run")},
		{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqPart})},
	}
	msgs := buildGenkitMessages(history, "next", "sys")
	if len(msgs) != 3 {
		t.Fatalf("len=%d want system+user+newUser (dangling tool request dropped)", len(msgs))
	}
	if msgs[1].Role != genkitAI.RoleUser || msgs[2].Role != genkitAI.RoleUser {
		t.Fatalf("roles=%q %q", msgs[1].Role, msgs[2].Role)
	}
}

func TestSanitizeDropsOrphanToolMessage(t *testing.T) {
	respPart := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{Name: ToolRunSSHCommand, Ref: "c1", Output: "x"})
	history := []ChatMessage{
		{Role: RoleUser, Content: "hi", Parts: UserTextPartsJSON("hi")},
		{Role: RoleTool, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{respPart})},
	}
	msgs := buildGenkitMessages(history, "next", "sys")
	if len(msgs) != 3 {
		t.Fatalf("len=%d want system+user+newUser", len(msgs))
	}
	for _, m := range msgs {
		if m.Role == genkitAI.RoleTool {
			t.Fatalf("orphan tool survived: %+v", m)
		}
	}
}

func TestUnmarshalPartsRoundTrip(t *testing.T) {
	p := genkitAI.NewTextPart("hi")
	raw := MarshalParts([]*genkitAI.Part{p})
	got := UnmarshalParts(raw)
	if len(got) != 1 || !got[0].IsText() || got[0].Text != "hi" {
		t.Fatalf("roundtrip=%s parts=%+v", raw, got)
	}
}
