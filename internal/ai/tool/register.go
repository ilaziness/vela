package tool

import (
	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// Set holds Genkit tool refs registered at engine Init.
type Set struct {
	Plan   genkitAI.ToolRef
	SSH    genkitAI.ToolRef
	Read   genkitAI.ToolRef
	Write  genkitAI.ToolRef
	List   genkitAI.ToolRef
	Search genkitAI.ToolRef
}

// Register defines all built-in Genkit tools on g.
func Register(g *genkit.Genkit) Set {
	read, write, list := defineFSTools(g)
	return Set{
		Plan:   definePlanTool(g),
		SSH:    defineSSHTool(g),
		Read:   read,
		Write:  write,
		List:   list,
		Search: defineWebSearchTool(g),
	}
}
