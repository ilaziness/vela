package ai

import (
	"encoding/json"

	genkitAI "github.com/firebase/genkit/go/ai"
)

const (
	RoleUser   = string(genkitAI.RoleUser)
	RoleModel  = string(genkitAI.RoleModel)
	RoleTool   = string(genkitAI.RoleTool)
	RoleSystem = string(genkitAI.RoleSystem)
)

// ChatMessage is one persisted Genkit message (role = user|model|tool|system).
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Parts   string `json:"parts,omitempty"` // JSON []*genkitAI.Part
}

// AgentRequest is the input for a single agent Run.
type AgentRequest struct {
	Messages     []ChatMessage
	NewMessage   string
	SystemPrompt string
	EnableSSH    bool
	LinkID       string
}

// AgentResponse is the new model/tool messages produced by this Run.
type AgentResponse struct {
	Content string
	Turn    []ChatMessage
}

// buildGenkitMessages rebuilds a native Genkit transcript plus the new user turn.
// System prompt is always injected fresh (not taken from stored history).
func buildGenkitMessages(history []ChatMessage, newMsg, systemPrompt string) []*genkitAI.Message {
	var msgs []*genkitAI.Message
	if systemPrompt != "" {
		msgs = append(msgs, genkitAI.NewSystemTextMessage(systemPrompt))
	}
	for _, m := range history {
		role, ok := genkitRole(m.Role)
		if !ok || role == genkitAI.RoleSystem {
			continue
		}
		parts := UnmarshalParts(m.Parts)
		if len(parts) == 0 && m.Content != "" {
			parts = []*genkitAI.Part{genkitAI.NewTextPart(m.Content)}
		}
		if len(parts) == 0 {
			continue
		}
		msgs = append(msgs, genkitAI.NewMessage(role, nil, parts...))
	}
	msgs = sanitizeTranscript(msgs)
	msgs = append(msgs, genkitAI.NewUserTextMessage(newMsg))
	return msgs
}

func genkitRole(s string) (genkitAI.Role, bool) {
	r := genkitAI.Role(s)
	switch r {
	case genkitAI.RoleUser, genkitAI.RoleModel, genkitAI.RoleTool, genkitAI.RoleSystem:
		return r, true
	default:
		return "", false
	}
}

// sanitizeTranscript drops orphan tool messages and a trailing model
// turn that still has unanswered tool requests. Providers reject both.
func sanitizeTranscript(msgs []*genkitAI.Message) []*genkitAI.Message {
	out := make([]*genkitAI.Message, 0, len(msgs))
	for _, m := range msgs {
		if m == nil {
			continue
		}
		if m.Role == genkitAI.RoleTool {
			if len(out) == 0 || out[len(out)-1].Role != genkitAI.RoleModel || !hasToolRequest(out[len(out)-1]) {
				continue
			}
		}
		out = append(out, m)
	}
	return trimIncompleteToolRound(out)
}

// trimIncompleteToolRound drops a trailing model message that still has
// unanswered tool requests.
func trimIncompleteToolRound(msgs []*genkitAI.Message) []*genkitAI.Message {
	for len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		if last == nil {
			msgs = msgs[:len(msgs)-1]
			continue
		}
		if last.Role != genkitAI.RoleModel || !hasToolRequest(last) {
			break
		}
		msgs = msgs[:len(msgs)-1]
	}
	return msgs
}

func hasToolRequest(m *genkitAI.Message) bool {
	if m == nil {
		return false
	}
	for _, p := range m.Content {
		if p != nil && p.IsToolRequest() {
			return true
		}
	}
	return false
}

func storedTurn(resp *genkitAI.ModelResponse, promptLen int) *AgentResponse {
	out := &AgentResponse{}
	if resp == nil {
		return out
	}
	hist := resp.History()
	if promptLen < 0 {
		promptLen = 0
	}
	if promptLen > len(hist) {
		promptLen = len(hist)
	}
	var turn []ChatMessage
	for _, m := range hist[promptLen:] {
		if m == nil {
			continue
		}
		turn = append(turn, ChatMessage{
			Role:    string(m.Role),
			Content: messageContent(m),
			Parts:   MarshalParts(m.Content),
		})
	}
	out.Turn = completeTurn(turn)
	for i := len(out.Turn) - 1; i >= 0; i-- {
		if out.Turn[i].Role == RoleModel && out.Turn[i].Content != "" {
			out.Content = out.Turn[i].Content
			break
		}
	}
	if out.Content == "" && resp.Message != nil {
		out.Content = resp.Message.Text()
	}
	return out
}

// applyStreamedText fills in model text that was emitted as stream deltas but may be
// missing from ModelResponse when generation is cancelled mid-stream.
// Only used when the stored turn has no model text yet; streamed spans the whole run
// and must not overwrite multi-turn model messages.
func applyStreamedText(out *AgentResponse, streamed string) *AgentResponse {
	if out == nil {
		out = &AgentResponse{}
	}
	if hasModelText(out) {
		return out
	}
	// Prefer streamed deltas; fall back to Content (e.g. Message.Text with empty Turn).
	text := streamed
	if text == "" {
		text = out.Content
	}
	if text == "" {
		return out
	}
	msg := ChatMessage{
		Role:    RoleModel,
		Content: text,
		Parts:   UserTextPartsJSON(text),
	}
	if n := len(out.Turn); n > 0 && out.Turn[n-1].Role == RoleModel {
		out.Turn[n-1] = msg
	} else {
		out.Turn = append(out.Turn, msg)
	}
	out.Content = text
	return out
}

func hasModelText(out *AgentResponse) bool {
	if out == nil {
		return false
	}
	for _, m := range out.Turn {
		if messageHasTextPart(m) {
			return true
		}
	}
	return false
}

func messageHasTextPart(m ChatMessage) bool {
	if m.Role != RoleModel {
		return false
	}
	for _, p := range UnmarshalParts(m.Parts) {
		if p != nil && p.IsText() && p.Text != "" {
			return true
		}
	}
	return false
}

// completeTurn drops orphan tool messages and a trailing unanswered tool request
// so a failed generate cannot poison the next prompt.
func completeTurn(turn []ChatMessage) []ChatMessage {
	if len(turn) == 0 {
		return turn
	}
	msgs := make([]*genkitAI.Message, 0, len(turn))
	for _, m := range turn {
		role, ok := genkitRole(m.Role)
		if !ok || role == genkitAI.RoleSystem {
			continue
		}
		parts := UnmarshalParts(m.Parts)
		if len(parts) == 0 && m.Content != "" {
			parts = []*genkitAI.Part{genkitAI.NewTextPart(m.Content)}
		}
		if len(parts) == 0 {
			continue
		}
		msgs = append(msgs, genkitAI.NewMessage(role, nil, parts...))
	}
	msgs = sanitizeTranscript(msgs)
	out := make([]ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ChatMessage{
			Role:    string(m.Role),
			Content: messageContent(m),
			Parts:   MarshalParts(m.Content),
		})
	}
	return out
}

func messageContent(m *genkitAI.Message) string {
	if m == nil {
		return ""
	}
	if t := m.Text(); t != "" {
		return t
	}
	for _, p := range m.Content {
		if p == nil {
			continue
		}
		if p.IsToolRequest() && p.ToolRequest != nil {
			return "[" + p.ToolRequest.Name + "]"
		}
		if p.IsToolResponse() && p.ToolResponse != nil {
			return "[" + p.ToolResponse.Name + "]"
		}
	}
	return ""
}

// MarshalParts encodes Genkit parts for SQLite.
func MarshalParts(parts []*genkitAI.Part) string {
	if len(parts) == 0 {
		return "[]"
	}
	b, err := json.Marshal(parts)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// UnmarshalParts decodes Genkit parts JSON.
func UnmarshalParts(partsJSON string) []*genkitAI.Part {
	if partsJSON == "" || partsJSON == "[]" {
		return nil
	}
	var parts []*genkitAI.Part
	if err := json.Unmarshal([]byte(partsJSON), &parts); err != nil {
		return nil
	}
	return parts
}

// UserTextPartsJSON is a Genkit text-part array for a user message.
func UserTextPartsJSON(text string) string {
	return MarshalParts([]*genkitAI.Part{genkitAI.NewTextPart(text)})
}
