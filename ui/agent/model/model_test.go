package model

import (
	"testing"

	"github.com/ilaziness/vela/internal/agent"
	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/ai/event"
)

func chunk(typ string, kv func(*agent.AIStreamChunkData)) agent.AIStreamChunkData {
	c := agent.AIStreamChunkData{Type: typ}
	if kv != nil {
		kv(&c)
	}
	return c
}

func TestApplyTextAndReasoning(t *testing.T) {
	c := NewChat("s1")
	c.BeginRun("r1")
	c.Apply(chunk(event.EventReasoningDelta, func(d *agent.AIStreamChunkData) { d.Delta = "hmm" }))
	c.Apply(chunk(event.EventTextDelta, func(d *agent.AIStreamChunkData) { d.Delta = "Hello " }))
	c.Apply(chunk(event.EventTextDelta, func(d *agent.AIStreamChunkData) { d.Delta = "world" }))
	c.EndRun(false)

	if len(c.Messages) != 1 {
		t.Fatalf("messages=%d", len(c.Messages))
	}
	m := c.Messages[0]
	if m.Reasoning() != "hmm" || m.Text() != "Hello world" {
		t.Fatalf("text=%q reasoning=%q", m.Text(), m.Reasoning())
	}
	if m.Status != "" {
		t.Fatalf("status=%q", m.Status)
	}
}

func TestApplyToolLifecycle(t *testing.T) {
	c := NewChat("s1")
	c.BeginRun("r1")
	c.Apply(chunk(event.EventToolInputStart, func(d *agent.AIStreamChunkData) { d.ToolCallID = "call1" }))
	c.Apply(chunk(event.EventToolInputAvailable, func(d *agent.AIStreamChunkData) {
		d.ToolCallID, d.ToolName, d.Input = "call1", "run_ssh_command", map[string]any{"command": "ls"}
	}))
	c.Apply(chunk(event.EventToolApprovalRequest, func(d *agent.AIStreamChunkData) {
		d.ToolCallID, d.ToolName, d.ApprovalID, d.Input = "call1", "run_ssh_command", "ap1", map[string]any{"command": "ls"}
	}))

	a := c.Approval("ap1")
	if a == nil || a.ToolCallID != "call1" {
		t.Fatalf("approval=%+v", a)
	}
	if tc := c.findTool("call1"); tc == nil || tc.Status != ToolAwaiting {
		t.Fatalf("tool=%+v", tc)
	}

	c.Apply(chunk(event.EventToolOutputAvailable, func(d *agent.AIStreamChunkData) {
		d.ToolCallID, d.Output = "call1", "file.txt"
	}))
	if tc := c.findTool("call1"); tc.Status != ToolSuccess {
		t.Fatalf("status=%q", tc.Status)
	}
	c.RemoveApproval("ap1")
	if c.Approval("ap1") != nil {
		t.Fatal("approval not removed")
	}
}

func TestApplyDenied(t *testing.T) {
	c := NewChat("s1")
	c.BeginRun("r1")
	c.Apply(chunk(event.EventToolInputAvailable, func(d *agent.AIStreamChunkData) {
		d.ToolCallID, d.ToolName = "c9", "run_ssh_command"
	}))
	c.Apply(chunk(event.EventToolApprovalRequest, func(d *agent.AIStreamChunkData) {
		d.ToolCallID, d.ToolName, d.ApprovalID = "c9", "run_ssh_command", "ap9"
	}))
	c.Apply(chunk(event.EventToolOutputDenied, func(d *agent.AIStreamChunkData) {
		d.ToolCallID, d.Reason = "c9", "user denied"
	}))
	// The denied status chip carries the outcome; the engine's reason is an
	// internal token ("user denied" / "stopped"), so it must not become the
	// card's error text (that renders with the error style).
	if tc := c.findTool("c9"); tc.Status != ToolDenied || tc.ErrorText != "" {
		t.Fatalf("tool=%+v", tc)
	}
	// The approval bound to that call is resolved with it.
	if len(c.Approvals) != 0 {
		t.Fatalf("approvals=%d", len(c.Approvals))
	}
}

func TestEndRunCancelled(t *testing.T) {
	c := NewChat("s1")
	c.BeginRun("r1")
	c.Apply(chunk(event.EventTextDelta, func(d *agent.AIStreamChunkData) { d.Delta = "partial" }))
	c.EndRun(true)
	if c.Messages[0].Status != MsgCancelled {
		t.Fatalf("status=%q", c.Messages[0].Status)
	}
	if c.Running {
		t.Fatal("still running")
	}
}

func TestLoadHistory(t *testing.T) {
	c := NewChat("s1")
	userParts := ai.MarshalParts(nil)
	_ = userParts
	msgs := []*agent.AIMessage{
		{ID: "m1", Role: ai.RoleUser, Content: "hi", Timestamp: 1},
		{ID: "m2", Role: ai.RoleModel, Parts: `[{"kind":0,"text":"Hello "},{"kind":0,"text":"there"}]`, Timestamp: 2},
	}
	c.Load(msgs)
	if len(c.Messages) != 2 {
		t.Fatalf("messages=%d", len(c.Messages))
	}
	if c.Messages[0].Role != RoleUser || c.Messages[0].Text() != "hi" {
		t.Fatalf("user=%+v", c.Messages[0])
	}
	if c.Messages[1].Text() != "Hello there" {
		t.Fatalf("assistant=%q", c.Messages[1].Text())
	}
}

func TestLoadPendingApproval(t *testing.T) {
	c := NewChat("s1")
	// A tool request part with an unresolved interrupt replays as awaiting.
	msgs := []*agent.AIMessage{
		{ID: "m1", Role: ai.RoleUser, Content: "run ls", Timestamp: 1},
		{ID: "m2", Role: ai.RoleModel, Timestamp: 2, Parts: toolRequestPartsJSON("call1", "run_ssh_command", `{"command":"ls"}`)},
	}
	c.Load(msgs)
	found := false
	for _, m := range c.Messages {
		for _, b := range m.Blocks {
			if b.Tool != nil && b.Tool.ID == "call1" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("tool card missing")
	}
}

func toolRequestPartsJSON(callID, name, input string) string {
	return `[{"kind":2,"toolRequest":{"name":"` + name + `","ref":"` + callID + `","input":` + input + `},"metadata":{"interrupt":{"id":"ap-` + callID + `","toolCallId":"` + callID + `","toolName":"` + name + `","input":` + input + `}}}]`
}
