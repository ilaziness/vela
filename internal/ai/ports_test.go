package ai

import (
	"fmt"
	"strings"
	"testing"

	genkitAI "github.com/firebase/genkit/go/ai"
)

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

func TestCompleteTurnOnCancelSynthesizesDeniedTools(t *testing.T) {
	reqPart := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "c1"})
	turn := []ChatMessage{
		{Role: RoleModel, Content: "ok", Parts: UserTextPartsJSON("ok")},
		{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqPart})},
	}
	got := completeTurnOnCancel(turn)
	if len(got) != 3 {
		t.Fatalf("len=%d got=%+v", len(got), got)
	}
	if got[2].Role != RoleTool || got[2].Status != MessageStatusDenied {
		t.Fatalf("expected denied tool row: %+v", got[2])
	}
	parts := UnmarshalParts(got[2].Parts)
	if len(parts) != 1 || !parts[0].IsToolResponse() {
		t.Fatalf("parts=%+v", parts)
	}
	if denialReasonFromOutput(parts[0].ToolResponse.Output) != "stopped" {
		t.Fatalf("output=%v", parts[0].ToolResponse.Output)
	}
}

func TestCompleteTurnOnCancelKeepsAnsweredAndSynthesizesRest(t *testing.T) {
	reqA := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "a"})
	reqB := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "b"})
	respA := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
		Name: ToolRunSSHCommand, Ref: "a", Output: "ok-a",
	})
	turn := []ChatMessage{
		{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqA, reqB})},
		{Role: RoleTool, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{respA})},
	}
	got := completeTurnOnCancel(turn)
	if len(got) != 2 {
		t.Fatalf("len=%d got=%+v", len(got), got)
	}
	if got[1].Role != RoleTool {
		t.Fatalf("expected merged tool row: %+v", got[1])
	}
	// Mixed success + synthetic denial: do not mark the whole row denied.
	if got[1].Status == MessageStatusDenied {
		t.Fatalf("mixed tool row must not be status=denied: %+v", got[1])
	}
	parts := UnmarshalParts(got[1].Parts)
	if len(parts) != 2 {
		t.Fatalf("parts=%+v", parts)
	}
	if denialReasonFromOutput(parts[0].ToolResponse.Output) != "" {
		t.Fatalf("answered output must stay: %v", parts[0].ToolResponse.Output)
	}
	if denialReasonFromOutput(parts[1].ToolResponse.Output) != "stopped" {
		t.Fatalf("synthetic denial missing: %v", parts[1].ToolResponse.Output)
	}
	replay := sanitizeTranscript(chatMessagesToGenkit(got))
	if len(replay) != 2 {
		t.Fatalf("sanitize replay len=%d", len(replay))
	}
}

func TestSanitizeTranscriptKeepsConsecutiveToolMessages(t *testing.T) {
	reqA := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "a"})
	reqB := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "b"})
	respA := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{Name: ToolRunSSHCommand, Ref: "a", Output: "ok"})
	respB := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
		Name: ToolRunSSHCommand, Ref: "b", Output: "User denied tool execution: stopped",
	})
	msgs := []*genkitAI.Message{
		genkitAI.NewMessage(genkitAI.RoleModel, nil, reqA, reqB),
		genkitAI.NewMessage(genkitAI.RoleTool, nil, respA),
		genkitAI.NewMessage(genkitAI.RoleTool, nil, respB),
	}
	got := sanitizeTranscript(msgs)
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
}

func TestDenialReasonFromOutput(t *testing.T) {
	if got := denialReasonFromOutput("User denied tool execution: stopped"); got != "stopped" {
		t.Fatalf("string=%q", got)
	}
	if got := denialReasonFromOutput(map[string]any{"output": "User denied command execution: no"}); got != "no" {
		t.Fatalf("nested=%q", got)
	}
	if got := denialReasonFromOutput("User denied tool execution: "); got != "user denied" {
		t.Fatalf("empty reason=%q", got)
	}
	if got := denialReasonFromOutput("ok"); got != "" {
		t.Fatalf("success=%q", got)
	}
}

func TestPrepareTurnMarksDeniedToolStatus(t *testing.T) {
	reqPart := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "c1"})
	respPart := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
		Name: ToolRunSSHCommand, Ref: "c1", Output: "User denied command execution: no thanks",
	})
	resp := PrepareTurn(&AgentResponse{Turn: []ChatMessage{
		{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqPart})},
		{Role: RoleTool, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{respPart})},
	}}, false)
	if len(resp.Turn) != 2 || resp.Turn[1].Status != MessageStatusDenied {
		t.Fatalf("got=%+v", resp.Turn)
	}
}

func TestPrepareTurnMixedToolResponsesStayUndeniedStatus(t *testing.T) {
	reqA := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "a"})
	reqB := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{Name: ToolRunSSHCommand, Ref: "b"})
	respA := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{Name: ToolRunSSHCommand, Ref: "a", Output: "ok"})
	respB := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
		Name: ToolRunSSHCommand, Ref: "b", Output: "User denied command execution: no",
	})
	resp := PrepareTurn(&AgentResponse{Turn: []ChatMessage{
		{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{reqA, reqB})},
		{Role: RoleTool, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{respA, respB})},
	}}, false)
	if len(resp.Turn) != 2 || resp.Turn[1].Status != "" {
		t.Fatalf("mixed tool row status=%q turn=%+v", resp.Turn[1].Status, resp.Turn)
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
	parts := UnmarshalParts(got.Turn[0].Parts)
	if !hasToolRequestPart(parts) {
		t.Fatalf("tool request must be preserved: %+v", parts)
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

	partial := &AgentResponse{
		Content: "hello",
		Turn:    []ChatMessage{{Role: RoleModel, Content: "hello", Parts: UserTextPartsJSON("hello")}},
	}
	got = applyStreamedText(partial, "hello world")
	if got.Content != "hello world" || got.Turn[0].Content != "hello world" {
		t.Fatalf("should extend last text model by streamed prefix: %+v", got)
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

func TestBuildGenkitMessagesStripsUpsertPlanKeepsSSH(t *testing.T) {
	planReq := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{
		Name: ToolUpsertPlan, Ref: "p1",
		Input: map[string]any{"title": "t", "tasks": []any{}},
	})
	planResp := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
		Name: ToolUpsertPlan, Ref: "p1", Output: map[string]any{"title": "t", "tasks": []any{}},
	})
	sshReq := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{
		Name: ToolRunSSHCommand, Ref: "s1", Input: map[string]any{"command": "uname"},
	})
	sshResp := genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
		Name: ToolRunSSHCommand, Ref: "s1", Output: "Linux",
	})
	history := []ChatMessage{
		{Role: RoleUser, Content: "fix", Parts: UserTextPartsJSON("fix")},
		{Role: RoleModel, Content: "planning", Parts: MarshalParts([]*genkitAI.Part{
			genkitAI.NewTextPart("ok"), planReq,
		})},
		{Role: RoleTool, Content: "[" + ToolUpsertPlan + "]", Parts: MarshalParts([]*genkitAI.Part{planResp})},
		{Role: RoleModel, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{sshReq})},
		{Role: RoleTool, Content: "[" + ToolRunSSHCommand + "]", Parts: MarshalParts([]*genkitAI.Part{sshResp})},
	}
	msgs := buildGenkitMessages(history, "again", "sys")
	// system + user + model(text only) + model(ssh req) + tool(ssh) + new user
	if len(msgs) != 6 {
		t.Fatalf("len=%d want 6", len(msgs))
	}
	if msgs[2].Role != genkitAI.RoleModel || len(msgs[2].Content) != 1 || !msgs[2].Content[0].IsText() {
		t.Fatalf("model after strip should keep text only: %+v", msgs[2])
	}
	if msgs[3].Role != genkitAI.RoleModel || !msgs[3].Content[0].IsToolRequest() ||
		msgs[3].Content[0].ToolRequest.Name != ToolRunSSHCommand {
		t.Fatalf("ssh request missing: %+v", msgs[3])
	}
	if msgs[4].Role != genkitAI.RoleTool || msgs[4].Content[0].ToolResponse.Name != ToolRunSSHCommand {
		t.Fatalf("ssh response missing: %+v", msgs[4])
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
