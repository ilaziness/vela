package tool

import (
	"encoding/json"
	"strings"
	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/ilaziness/vela/internal/ai/event"
)

// PlanTask is one step in the authoritative session plan snapshot.
type PlanTask struct {
	ID     string `json:"id" jsonschema_description:"Stable task id within the plan."`
	Title  string `json:"title" jsonschema_description:"Short task title."`
	Status string `json:"status" jsonschema_description:"One of: pending, running, done, failed."`
}

// PlanSnapshot is the full replace payload for upsert_plan (and session plan_json).
type PlanSnapshot struct {
	Title string     `json:"title" jsonschema_description:"Plan title."`
	Tasks []PlanTask `json:"tasks" jsonschema_description:"Full snapshot of tasks (replace previous plan)."`
}

func definePlanTool(g *genkit.Genkit) genkitAI.ToolRef {
	return genkit.DefineTool(g, UpsertPlan,
		"Create or replace the current multi-step plan (full snapshot). "+
			"Mark a step running before starting it; mark done/failed immediately after that step finishes. "+
			"Do not batch status updates until the end. Empty tasks clears the plan.",
		func(tc *genkitAI.ToolContext, input PlanSnapshot) (PlanSnapshot, error) {
			normalizePlan(&input)
			inJSON, err := json.Marshal(input)
			if err != nil {
				return input, err
			}
			deps := FromCtx(tc)
			store := string(inJSON)
			if len(input.Tasks) == 0 {
				store = ""
			}
			if deps.OnPlanUpsert != nil {
				if err := deps.OnPlanUpsert(store); err != nil {
					return input, err
				}
			}
			callID := uuid.New().String()
			emit(deps, event.StreamEvent{Type: event.EventStartStep})
			emit(deps, event.StreamEvent{Type: event.EventToolInputStart, ToolCallID: callID, ToolName: UpsertPlan})
			emit(deps, event.StreamEvent{Type: event.EventToolInputAvailable, ToolCallID: callID, ToolName: UpsertPlan, Input: inJSON})
			emit(deps, event.StreamEvent{Type: event.EventToolOutputAvailable, ToolCallID: callID, ToolName: UpsertPlan, Output: inJSON})
			emit(deps, event.StreamEvent{Type: event.EventFinishStep})
			return input, nil
		})
}

func normalizePlan(p *PlanSnapshot) {
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
