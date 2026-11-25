package tool

import (
	"fmt"
	"strings"
	"unicode/utf8"

	genkitAI "github.com/firebase/genkit/go/ai"
	genkitTool "github.com/firebase/genkit/go/ai/tool"
	"github.com/firebase/genkit/go/genkit"
)

const (
	terminalMaxScrollback = 2000
	terminalMaxOut        = 48 * 1024
)

type terminalInput struct {
	ScrollbackLines int `json:"scrollback_lines" jsonschema_description:"How many scrollback rows above the live screen to include (0 = current screen only, max 2000). Increase only when the visible screen is not enough."`
}

func defineTerminalTool(g *genkit.Genkit) genkitAI.ToolRef {
	return genkit.DefineTool(g, ReadTerminal,
		"Read the text currently shown in the user's bound terminal (plain text, no ANSI). Returns the live screen plus up to scrollback_lines rows of scrollback above it. Use it to see command output, errors, or what the terminal is displaying; call again with a larger scrollback_lines when more history is needed.",
		func(tc *genkitAI.ToolContext, input terminalInput) (string, error) {
			deps := FromCtx(tc)
			if deps.Screen == nil {
				return "", genkitTool.Fail(tc, fmt.Errorf("terminal read unavailable: no bound terminal session"))
			}
			lines := input.ScrollbackLines
			if lines < 0 {
				lines = 0
			}
			if lines > terminalMaxScrollback {
				lines = terminalMaxScrollback
			}
			text, err := deps.Screen(tc, lines)
			if err != nil {
				return "", genkitTool.Fail(tc, err)
			}
			text = strings.TrimRight(text, "\r\n")
			if text == "" {
				return "(terminal screen is empty)", nil
			}
			rows := strings.Count(text, "\n") + 1
			return fmt.Sprintf("[terminal screen: %d rows]\n%s", rows, truncateTail(text, terminalMaxOut)), nil
		})
}

// truncateTail keeps the last maxLen bytes on a UTF-8 boundary — the newest
// screen content matters most.
func truncateTail(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	s = s[len(s)-maxLen:]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[1:]
	}
	return "…[truncated]\n" + s
}
