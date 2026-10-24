package tool

import (
	"encoding/json"
	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/ilaziness/vela/internal/ai/event"
)

type webSearchInput struct {
	Query string `json:"query" jsonschema_description:"Search query string."`
}

func defineWebSearchTool(g *genkit.Genkit) genkitAI.ToolRef {
	return genkit.DefineTool(g, WebSearch,
		"Search the public web via the user's configured search provider. Returns titles, URLs, and snippets.",
		func(tc *genkitAI.ToolContext, input webSearchInput) (string, error) {
			deps := FromCtx(tc)
			if !deps.EnableWebSearch() {
				return "web search unavailable: provider or API key not configured", nil
			}
			callID := uuid.New().String()
			inJSON, _ := json.Marshal(input)
			emit(deps, event.StreamEvent{Type: event.EventStartStep})
			emit(deps, event.StreamEvent{Type: event.EventToolInputStart, ToolCallID: callID, ToolName: WebSearch})
			emit(deps, event.StreamEvent{Type: event.EventToolInputAvailable, ToolCallID: callID, ToolName: WebSearch, Input: inJSON})

			results, err := Search(tc, SearchConfig{
				Provider: deps.WebSearch.Provider,
				APIKey:   deps.WebSearch.APIKey,
			}, input.Query)
			if err != nil {
				emit(deps, event.StreamEvent{Type: event.EventToolOutputError, ToolCallID: callID, ToolName: WebSearch, ErrorText: err.Error()})
				emit(deps, event.StreamEvent{Type: event.EventFinishStep})
				return "error: " + err.Error(), nil //nolint:nilerr // soft-fail as tool result for the model
			}
			wrapped, _ := json.Marshal(map[string]any{"results": results})
			emit(deps, event.StreamEvent{Type: event.EventToolOutputAvailable, ToolCallID: callID, ToolName: WebSearch, Output: wrapped})
			emit(deps, event.StreamEvent{Type: event.EventFinishStep})
			return string(wrapped), nil
		})
}
