package tool

import (
	"encoding/json"
	"strings"
	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/ilaziness/vela/internal/ai/event"
)

type planTask struct {
	ID     string `json:"id" jsonschema_description:"Stable task id within the plan."`
	Title  string `json:"title" jsonschema_description:"Short task title."`
	Status string `json:"status" jsonschema_description:"One of: pending, running, done, failed."`
}

type upsertPlanInput struct {
	Title string     `json:"title" jsonschema_description:"Plan title."`
	Tasks []planTask `json:"tasks" jsonschema_description:"Full snapshot of tasks (replace previous plan)."`
}

func definePlanTool(g *genkit.Genkit) genkitAI.ToolRef {
	return genkit.DefineTool(g, UpsertPlan,
		"Create or replace the current multi-step plan shown in the chat. Always send the full task list snapshot.",
		func(tc *genkitAI.ToolContext, input upsertPlanInput) (upsertPlanInput, error) {
			normalizePlan(&input)
			callID := uuid.New().String()
			inJSON, _ := json.Marshal(input)
			deps := FromCtx(tc)
			emit(deps, event.StreamEvent{Type: event.EventStartStep})
			emit(deps, event.StreamEvent{Type: event.EventToolInputStart, ToolCallID: callID, ToolName: UpsertPlan})
			emit(deps, event.StreamEvent{Type: event.EventToolInputAvailable, ToolCallID: callID, ToolName: UpsertPlan, Input: inJSON})
			emit(deps, event.StreamEvent{Type: event.EventToolOutputAvailable, ToolCallID: callID, ToolName: UpsertPlan, Output: inJSON})
			emit(deps, event.StreamEvent{Type: event.EventFinishStep})
			return input, nil
		})
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
