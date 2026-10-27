package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	genkitAI "github.com/firebase/genkit/go/ai"
)

const (
	RoleUser   = string(genkitAI.RoleUser)
	RoleModel  = string(genkitAI.RoleModel)
	RoleTool   = string(genkitAI.RoleTool)
	RoleSystem = string(genkitAI.RoleSystem)

	// MessageStatusDenied is UI-only on tool rows; Genkit replay ignores Status.
	// Cancelled is applied at persist time in services (not set on ChatMessage here).
	MessageStatusDenied = "denied"
)

// ChatMessage is one persisted Genkit message (role = user|model|tool|system).
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Parts   string `json:"parts,omitempty"`  // JSON []*genkitAI.Part
	Status  string `json:"status,omitempty"` // "" | denied (UI only; cancelled stamped at DB persist)
}

// AgentRequest is the input for a single agent Run.
type AgentRequest struct {
	Messages     []ChatMessage
	NewParts     []*genkitAI.Part // user turn (text + attachment refs); resolved via engine userDataDir
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
// upsert_plan tool parts are stripped from sealed history (authoritative plan is in system).
// Attachment refs (vela-att:// and vela-path:) are resolved using userDataDir.
func buildGenkitMessages(history []ChatMessage, newParts []*genkitAI.Part, systemPrompt, userDataDir string) ([]*genkitAI.Message, error) {
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
		parts = stripUpsertPlanParts(parts)
		if len(parts) == 0 {
			continue
		}
		resolved, err := ResolvePartsForModel(userDataDir, parts)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, genkitAI.NewMessage(role, nil, resolved...))
	}
	msgs = sanitizeTranscript(msgs)
	if len(newParts) == 0 {
		return msgs, fmt.Errorf("new user parts are required")
	}
	resolvedNew, err := ResolvePartsForModel(userDataDir, newParts)
	if err != nil {
		return nil, err
	}
	msgs = append(msgs, genkitAI.NewUserMessage(resolvedNew...))
	return msgs, nil
}

// stripUpsertPlanParts drops upsert_plan tool request/response parts from prompt history.
func stripUpsertPlanParts(parts []*genkitAI.Part) []*genkitAI.Part {
	out := make([]*genkitAI.Part, 0, len(parts))
	for _, p := range parts {
		if p == nil {
			continue
		}
		if toolPartName(p) == ToolUpsertPlan {
			continue
		}
		out = append(out, p)
	}
	return out
}

func toolPartName(p *genkitAI.Part) string {
	if p.IsToolRequest() && p.ToolRequest != nil {
		return p.ToolRequest.Name
	}
	if p.IsToolResponse() && p.ToolResponse != nil {
		return p.ToolResponse.Name
	}
	return ""
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
		if m.Role == genkitAI.RoleTool && !toolFollowsModelRequest(out) {
			continue
		}
		out = append(out, m)
	}
	return trimIncompleteToolRound(out)
}

// toolFollowsModelRequest reports whether a tool message may follow out:
// immediately after a model tool-request turn, or after other tool messages
// in that same round (cancel may persist answered + synthetic denial rows).
func toolFollowsModelRequest(out []*genkitAI.Message) bool {
	for i := len(out) - 1; i >= 0; i-- {
		m := out[i]
		if m == nil {
			continue
		}
		switch m.Role {
		case genkitAI.RoleTool:
			continue
		case genkitAI.RoleModel:
			return hasToolRequest(m)
		default:
			return false
		}
	}
	return false
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

// TurnFromHistory returns Genkit messages after promptLen as ChatMessage (raw turn, no PrepareTurn).
func TurnFromHistory(resp *genkitAI.ModelResponse, promptLen int) []ChatMessage {
	if resp == nil {
		return nil
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
		role := string(m.Role)
		// Prompt / injected system must never land in the persisted turn.
		// If promptLen is off (e.g. skills prepended a system message), skip
		// leaked user/system rows so ReplaceRunMessages is not given invalid roles.
		if role == RoleSystem || role == RoleUser {
			continue
		}
		turn = append(turn, ChatMessage{
			Role:    role,
			Content: messageContent(m),
			Parts:   MarshalParts(m.Content),
		})
	}
	return turn
}

func storedTurn(resp *genkitAI.ModelResponse, promptLen int) *AgentResponse {
	out := &AgentResponse{}
	if resp == nil {
		return out
	}
	// Leave turn raw; PrepareTurn finalizes for persist (drop vs synthesize denials).
	out.Turn = TurnFromHistory(resp, promptLen)
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

// PrepareTurn finalizes a run's messages for persistence.
// On cancel, unanswered tool requests get synthetic denied responses; otherwise incomplete tools are dropped.
func PrepareTurn(resp *AgentResponse, cancelled bool) *AgentResponse {
	if resp == nil {
		return nil
	}
	if cancelled {
		resp.Turn = completeTurnOnCancel(resp.Turn)
	} else {
		resp.Turn = completeTurn(resp.Turn)
	}
	annotateDeniedToolStatuses(resp.Turn)
	return resp
}

// applyStreamedText fills in model text that was emitted as stream deltas but may be
// missing from ModelResponse when generation is cancelled mid-stream.
// Only used when the stored turn has no model text yet; streamed spans the whole run
// and must not overwrite multi-turn model messages.
// When the last model message is tool-request-only, text is prepended so tool parts stay.
func applyStreamedText(out *AgentResponse, streamed string) *AgentResponse {
	if out == nil {
		out = &AgentResponse{}
	}
	if streamed != "" {
		if n := len(out.Turn); n > 0 && out.Turn[n-1].Role == RoleModel {
			parts := UnmarshalParts(out.Turn[n-1].Parts)
			if !hasToolRequestPart(parts) && strings.HasPrefix(streamed, out.Turn[n-1].Content) && streamed != out.Turn[n-1].Content {
				out.Turn[n-1].Content = streamed
				out.Turn[n-1].Parts = UserTextPartsJSON(streamed)
				out.Content = streamed
				return out
			}
		}
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
		parts := UnmarshalParts(out.Turn[n-1].Parts)
		if hasToolRequestPart(parts) {
			merged := append([]*genkitAI.Part{genkitAI.NewTextPart(text)}, parts...)
			out.Turn[n-1].Parts = MarshalParts(merged)
			out.Turn[n-1].Content = text
		} else {
			out.Turn[n-1] = msg
		}
	} else {
		out.Turn = append(out.Turn, msg)
	}
	out.Content = text
	return out
}

func hasToolRequestPart(parts []*genkitAI.Part) bool {
	for _, p := range parts {
		if p != nil && p.IsToolRequest() {
			return true
		}
	}
	return false
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
	msgs := chatMessagesToGenkit(turn)
	msgs = sanitizeTranscript(msgs)
	return genkitToChatMessages(msgs, nil)
}

// completeTurnOnCancel keeps unanswered tool requests by synthesizing denied tool responses.
func completeTurnOnCancel(turn []ChatMessage) []ChatMessage {
	if len(turn) == 0 {
		return turn
	}
	msgs := dropOrphanTools(chatMessagesToGenkit(turn))
	msgs, deniedIdx := fillUnansweredToolRequests(msgs, "stopped")
	return genkitToChatMessages(msgs, deniedIdx)
}

func chatMessagesToGenkit(turn []ChatMessage) []*genkitAI.Message {
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
	return msgs
}

func genkitToChatMessages(msgs []*genkitAI.Message, deniedIdx map[int]bool) []ChatMessage {
	out := make([]ChatMessage, 0, len(msgs))
	for i, m := range msgs {
		cm := ChatMessage{
			Role:    string(m.Role),
			Content: messageContent(m),
			Parts:   MarshalParts(m.Content),
		}
		if deniedIdx[i] {
			cm.Status = MessageStatusDenied
		}
		out = append(out, cm)
	}
	return out
}

func dropOrphanTools(msgs []*genkitAI.Message) []*genkitAI.Message {
	out := make([]*genkitAI.Message, 0, len(msgs))
	for _, m := range msgs {
		if m == nil {
			continue
		}
		if m.Role == genkitAI.RoleTool && !toolFollowsModelRequest(out) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func fillUnansweredToolRequests(msgs []*genkitAI.Message, reason string) ([]*genkitAI.Message, map[int]bool) {
	answered := make(map[string]bool)
	for _, m := range msgs {
		if m == nil || m.Role != genkitAI.RoleTool {
			continue
		}
		for _, p := range m.Content {
			if p != nil && p.IsToolResponse() && p.ToolResponse != nil {
				answered[toolRefKey(p.ToolResponse.Name, p.ToolResponse.Ref)] = true
			}
		}
	}
	denyReason := reason
	if denyReason == "" {
		denyReason = "user denied"
	}
	var synthetic []*genkitAI.Part
	for _, m := range msgs {
		if m == nil || m.Role != genkitAI.RoleModel {
			continue
		}
		for _, p := range m.Content {
			if p == nil || !p.IsToolRequest() || p.ToolRequest == nil {
				continue
			}
			key := toolRefKey(p.ToolRequest.Name, p.ToolRequest.Ref)
			if answered[key] {
				continue
			}
			synthetic = append(synthetic, genkitAI.NewToolResponsePart(&genkitAI.ToolResponse{
				Name:   p.ToolRequest.Name,
				Ref:    p.ToolRequest.Ref,
				Output: fmt.Sprintf("User denied tool execution: %s", denyReason),
			}))
			answered[key] = true
		}
	}
	deniedIdx := map[int]bool{}
	if len(synthetic) == 0 {
		return msgs, deniedIdx
	}
	// Prefer merging into the trailing tool message so Genkit replay stays one
	// tool turn (answered outputs + synthetic denials). Frontend maps denial per part.
	if n := len(msgs); n > 0 && msgs[n-1] != nil && msgs[n-1].Role == genkitAI.RoleTool {
		msgs[n-1].Content = append(msgs[n-1].Content, synthetic...)
		if toolGenkitMessageAllDenied(msgs[n-1]) {
			deniedIdx[n-1] = true
		}
		return msgs, deniedIdx
	}
	msgs = append(msgs, genkitAI.NewMessage(genkitAI.RoleTool, nil, synthetic...))
	deniedIdx[len(msgs)-1] = true
	return msgs, deniedIdx
}

func toolGenkitMessageAllDenied(m *genkitAI.Message) bool {
	if m == nil {
		return false
	}
	saw := false
	for _, p := range m.Content {
		if p == nil || !p.IsToolResponse() || p.ToolResponse == nil {
			continue
		}
		saw = true
		if denialReasonFromOutput(p.ToolResponse.Output) == "" {
			return false
		}
	}
	return saw
}

func toolRefKey(name, ref string) string {
	if ref != "" {
		return ref
	}
	return name
}

func annotateDeniedToolStatuses(turn []ChatMessage) {
	for i := range turn {
		if turn[i].Role != RoleTool {
			continue
		}
		if turn[i].Status == MessageStatusDenied {
			continue
		}
		// Only mark the row denied when every toolResponse is a denial; mixed
		// success+deny rows keep status empty and rely on per-part UI mapping.
		if toolMessageAllDenied(turn[i]) {
			turn[i].Status = MessageStatusDenied
		}
	}
}

func toolMessageAllDenied(m ChatMessage) bool {
	saw := false
	for _, p := range UnmarshalParts(m.Parts) {
		if p == nil || !p.IsToolResponse() || p.ToolResponse == nil {
			continue
		}
		saw = true
		if denialReasonFromOutput(p.ToolResponse.Output) == "" {
			return false
		}
	}
	return saw
}

func denialReasonFromOutput(output any) string {
	s := strings.TrimSpace(denialOutputText(output))
	if s == "" || s == "<nil>" {
		return ""
	}
	// Prefixes omit the trailing space so "…execution:" and "…execution: reason" both match after TrimSpace.
	for _, prefix := range []string{
		"User denied command execution:",
		"User denied tool execution:",
	} {
		if strings.HasPrefix(s, prefix) {
			reason := strings.TrimSpace(strings.TrimPrefix(s, prefix))
			if reason == "" {
				return "user denied"
			}
			return reason
		}
	}
	return ""
}

// denialOutputText unwraps string / {output: string} the same way as the frontend mapper.
func denialOutputText(output any) string {
	switch v := output.(type) {
	case nil:
		return ""
	case string:
		return v
	case map[string]any:
		if s, ok := v["output"].(string); ok {
			return s
		}
		return ""
	default:
		return fmt.Sprint(output)
	}
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
