package ai

import "github.com/ilaziness/vela/internal/ai/event"

// Re-export stream events for services / Wails bindings stability.
const (
	EventTextDelta           = event.EventTextDelta
	EventReasoningDelta      = event.EventReasoningDelta
	EventStartStep           = event.EventStartStep
	EventFinishStep          = event.EventFinishStep
	EventToolInputStart      = event.EventToolInputStart
	EventToolInputAvailable  = event.EventToolInputAvailable
	EventToolApprovalRequest = event.EventToolApprovalRequest
	EventToolOutputAvailable = event.EventToolOutputAvailable
	EventToolOutputError     = event.EventToolOutputError
	EventToolOutputDenied    = event.EventToolOutputDenied
)

// StreamEvent is a structured agent stream payload for the frontend.
type StreamEvent = event.StreamEvent
