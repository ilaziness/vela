package ai

import (
	"testing"

	genkitAI "github.com/firebase/genkit/go/ai"
)

func TestBuildResumePartsBareInterrupt(t *testing.T) {
	t.Parallel()
	part := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{
		Name: ToolRunSSHCommand, Ref: "c1", Input: map[string]any{"command": "ls"},
	})
	part.Interrupt = &genkitAI.ToolInterrupt{}
	msgs := []ChatMessage{{
		Role:  RoleModel,
		Parts: MarshalParts([]*genkitAI.Part{part}),
	}}
	got := BuildResumeParts(msgs, []ApprovalDecision{{ApprovalID: "c1", Approved: true}})
	if len(got) != 1 || got[0] == nil || !got[0].IsRestart() {
		t.Fatalf("want one restart part, got %#v", got)
	}
}

func TestPendingInterruptsFromHistoryBareInterrupt(t *testing.T) {
	t.Parallel()
	part := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{
		Name: ToolRunSSHCommand, Ref: "c1", Input: map[string]any{"command": "ls"},
	})
	part.Interrupt = &genkitAI.ToolInterrupt{}
	msgs := []ChatMessage{{
		Role:  RoleModel,
		Parts: MarshalParts([]*genkitAI.Part{part}),
	}}
	holds := PendingInterruptsFromHistory(msgs)
	if len(holds) != 1 || holds[0].ApprovalID != "c1" {
		t.Fatalf("got %#v", holds)
	}
}
