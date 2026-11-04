package tool

import (
	"encoding/json"
	"errors"

	genkitAI "github.com/firebase/genkit/go/ai"
	genkitTool "github.com/firebase/genkit/go/ai/tool"
	"github.com/firebase/genkit/go/genkit"
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
				return "", genkitTool.Fail(tc, errors.New("web search unavailable: provider or API key not configured"))
			}
			results, err := Search(tc, SearchConfig{
				Provider: deps.WebSearch.Provider,
				APIKey:   deps.WebSearch.APIKey,
			}, input.Query)
			if err != nil {
				return "", genkitTool.Fail(tc, err)
			}
			wrapped, err := json.Marshal(map[string]any{"results": results})
			if err != nil {
				return "", genkitTool.Fail(tc, err)
			}
			return string(wrapped), nil
		})
}
