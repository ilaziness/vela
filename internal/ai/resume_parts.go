package ai

import (
	"encoding/json"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/middleware"
)

// toolRequestRef is the stable approval / toolCall id (Ref, else Name). Never mint a new id.
func toolRequestRef(tr *genkitAI.ToolRequest) string {
	if tr == nil {
		return ""
	}
	if tr.Ref != "" {
		return tr.Ref
	}
	return tr.Name
}

func denyReason(reason string) string {
	if reason == "" {
		return "user denied"
	}
	return reason
}

// restartApprovalPart answers one ToolApproval hold (typed Restart, or ToToolRestart fallback).
func restartApprovalPart(part *genkitAI.Part, approved bool, reason string) *genkitAI.Part {
	decision := middleware.ToolCallDecision{Approved: approved, Reason: reason}
	if call, ok := middleware.ToolApprovalInterrupted(part); ok {
		return call.Restart(decision)
	}
	// Holds without stage metadata (older snapshots): Genkit documents ToToolRestart + ToolCallDecision.
	restart, err := part.ToToolRestart(decision)
	if err != nil || restart == nil {
		return nil
	}
	return restart
}

func pendingFromInterruptParts(parts []*genkitAI.Part) []PendingApproval {
	out := make([]PendingApproval, 0)
	for _, part := range parts {
		if part == nil || !part.IsInterrupt() || part.ToolRequest == nil {
			continue
		}
		ref := toolRequestRef(part.ToolRequest)
		var inJSON json.RawMessage
		if call, ok := middleware.ToolApprovalInterrupted(part); ok {
			inJSON, _ = json.Marshal(call.Input)
		} else {
			inJSON, _ = json.Marshal(part.ToolRequest.Input)
		}
		out = append(out, PendingApproval{
			ApprovalID: ref,
			ToolCallID: ref,
			ToolName:   part.ToolRequest.Name,
			Input:      inJSON,
		})
	}
	return out
}

// PendingInterruptsFromHistory extracts ToolApproval holds from the trailing model message.
func PendingInterruptsFromHistory(msgs []ChatMessage) []PendingApproval {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != RoleModel {
			continue
		}
		return pendingFromInterruptParts(UnmarshalParts(msgs[i].Parts))
	}
	return nil
}

// BuildResumeParts maps UI decisions onto Genkit ToolApproval restart parts.
func BuildResumeParts(msgs []ChatMessage, decisions []ApprovalDecision) []*genkitAI.Part {
	byID := make(map[string]ApprovalDecision, len(decisions))
	for _, d := range decisions {
		byID[d.ApprovalID] = d
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != RoleModel {
			continue
		}
		var resume []*genkitAI.Part
		for _, part := range UnmarshalParts(msgs[i].Parts) {
			if part == nil || !part.IsInterrupt() || part.ToolRequest == nil {
				continue
			}
			ref := toolRequestRef(part.ToolRequest)
			d, ok := byID[ref]
			if !ok {
				d = ApprovalDecision{ApprovalID: ref, Approved: false, Reason: "stopped"}
			}
			reason := ""
			if !d.Approved {
				reason = denyReason(d.Reason)
			}
			restart := restartApprovalPart(part, d.Approved, reason)
			if restart != nil {
				resume = append(resume, restart)
			}
		}
		return resume
	}
	return nil
}

// TrailingInterruptModelIndex returns the index of the trailing interrupted model message, or -1.
func TrailingInterruptModelIndex(msgs []*genkitAI.Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m == nil {
			continue
		}
		if m.Role == genkitAI.RoleTool {
			continue
		}
		if m.Role != genkitAI.RoleModel {
			return -1
		}
		for _, p := range m.Content {
			if p != nil && p.IsInterrupt() {
				return i
			}
		}
		return -1
	}
	return -1
}
