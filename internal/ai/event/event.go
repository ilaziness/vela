package event

import "encoding/json"

// Stream event kinds aligned with MUI X Chat chunk types (live UI only).
const (
	EventTextDelta           = "text-delta"
	EventReasoningDelta      = "reasoning-delta"
	EventStartStep           = "start-step"
	EventFinishStep          = "finish-step"
	EventToolInputStart      = "tool-input-start"
	EventToolInputAvailable  = "tool-input-available"
	EventToolApprovalRequest = "tool-approval-request"
	EventToolOutputAvailable = "tool-output-available"
	EventToolOutputError     = "tool-output-error"
	EventToolOutputDenied    = "tool-output-denied"
)

// StreamEvent is a structured agent stream payload for the frontend.
type StreamEvent struct {
	Type       string          `json:"type"`
	ID         string          `json:"id,omitempty"`
	Delta      string          `json:"delta,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	ToolName   string          `json:"toolName,omitempty"`
	ApprovalID string          `json:"approvalId,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	ErrorText  string          `json:"errorText,omitempty"`
	Reason     string          `json:"reason,omitempty"`
}
