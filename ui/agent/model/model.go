// Package model holds the agent chat transcript: pure data plus stream-event
// reduction and history replay. It has no go-gui or app dependencies so it can
// be unit-tested and shared by the view layer.
package model

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	genkitAI "github.com/firebase/genkit/go/ai"

	"github.com/ilaziness/vela/internal/agent"
	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/ai/event"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ToolStatus string

const (
	ToolRunning  ToolStatus = "running"
	ToolAwaiting ToolStatus = "awaiting"
	ToolSuccess  ToolStatus = "success"
	ToolError    ToolStatus = "error"
	ToolDenied   ToolStatus = "denied"
)

type MessageStatus string

const (
	MsgRunning   MessageStatus = "running"
	MsgCancelled MessageStatus = "cancelled"
	MsgDenied    MessageStatus = "denied"
	MsgError     MessageStatus = "error"
)

type BlockKind string

const (
	BlockText      BlockKind = "text"
	BlockReasoning BlockKind = "reasoning"
	BlockTool      BlockKind = "tool"
)

// ToolCall is one tool invocation inside an assistant turn.
type ToolCall struct {
	ID        string
	Name      string
	Input     json.RawMessage
	Output    json.RawMessage
	Status    ToolStatus
	ErrorText string
}

// Block is an ordered piece of an assistant message: streamed text, reasoning,
// or a tool card.
type Block struct {
	Kind BlockKind
	Text string
	Tool *ToolCall
}

// Message is one chat bubble. Assistant messages accumulate ordered blocks so
// tool cards appear where the model invoked them.
type Message struct {
	ID     string
	Role   Role
	Blocks []*Block
	Status MessageStatus
	RunID  string
	// Time is the unix timestamp shown under the bubble.
	Time int64
	// Attachments are local files recorded on a user message. Path is the
	// original absolute path (not a copy in the user data dir).
	Attachments []Attachment
}

// Attachment is one composer or user-message file.
type Attachment struct {
	Path     string
	Filename string
	MIME     string
}

// Approval is a pending tool-approval request shown as a card.
type Approval struct {
	ID         string
	ToolCallID string
	ToolName   string
	Input      json.RawMessage
}

// Chat is the whole sidebar transcript for one agent session.
type Chat struct {
	SessionID    string
	SessionTitle string
	Messages     []*Message
	Approvals    []*Approval
	Running      bool
	RunID        string
	// Rev increments on every transcript mutation; views compare it to
	// detect new content (auto-scroll, relayout).
	Rev int

	seq int
}

// NewChat returns an empty transcript bound to sessionID.
func NewChat(sessionID string) *Chat {
	return &Chat{SessionID: sessionID}
}

// Reset clears transcript state for a different session.
func (c *Chat) Reset(sessionID, title string) {
	*c = Chat{SessionID: sessionID, SessionTitle: title}
}

func (c *Chat) nextID() string {
	c.seq++
	return strconv.Itoa(c.seq)
}

// AppendUser adds a user bubble. atts is copied so the composer can clear its own slice.
func (c *Chat) AppendUser(text string, atts []Attachment) *Message {
	m := &Message{
		ID:          "u" + c.nextID(),
		Role:        RoleUser,
		Blocks:      []*Block{{Kind: BlockText, Text: text}},
		Time:        time.Now().Unix(),
		Attachments: cloneAttachments(atts),
	}
	c.Messages = append(c.Messages, m)
	c.Rev++
	return m
}

func (c *Chat) cur() *Message {
	if n := len(c.Messages); n > 0 {
		m := c.Messages[n-1]
		if m.Role == RoleAssistant && m.Status == MsgRunning {
			return m
		}
	}
	m := &Message{ID: "a" + c.nextID(), Role: RoleAssistant, Status: MsgRunning, RunID: c.RunID, Time: time.Now().Unix()}
	c.Messages = append(c.Messages, m)
	return m
}

func (c *Chat) ensureBlock(m *Message, kind BlockKind) *Block {
	if n := len(m.Blocks); n > 0 && m.Blocks[n-1].Kind == kind {
		return m.Blocks[n-1]
	}
	b := &Block{Kind: kind}
	m.Blocks = append(m.Blocks, b)
	return b
}

func (c *Chat) tool(callID string) *ToolCall {
	if callID == "" {
		return nil
	}
	m := c.cur()
	for _, b := range m.Blocks {
		if b.Kind == BlockTool && b.Tool != nil && b.Tool.ID == callID {
			return b.Tool
		}
	}
	t := &ToolCall{ID: callID, Status: ToolRunning}
	m.Blocks = append(m.Blocks, &Block{Kind: BlockTool, Tool: t})
	return t
}

func (c *Chat) findTool(callID string) *ToolCall {
	for _, m := range c.Messages {
		for _, b := range m.Blocks {
			if b.Kind == BlockTool && b.Tool != nil && b.Tool.ID == callID {
				return b.Tool
			}
		}
	}
	return nil
}

// Apply folds one stream chunk into the transcript.
func (c *Chat) Apply(ch agent.AIStreamChunkData) {
	c.Rev++
	switch ch.Type {
	case event.EventTextDelta:
		if ch.Delta == "" {
			return
		}
		b := c.ensureBlock(c.cur(), BlockText)
		b.Text += ch.Delta
	case event.EventReasoningDelta:
		if ch.Delta == "" {
			return
		}
		b := c.ensureBlock(c.cur(), BlockReasoning)
		b.Text += ch.Delta
	case event.EventToolInputStart:
		c.tool(ch.ToolCallID)
	case event.EventToolInputAvailable:
		t := c.tool(ch.ToolCallID)
		if t == nil {
			return
		}
		t.Name = ch.ToolName
		if raw := rawJSON(ch.Input); len(raw) > 0 {
			t.Input = raw
		}
	case event.EventToolApprovalRequest:
		c.addApproval(Approval{
			ID:         ch.ApprovalID,
			ToolCallID: ch.ToolCallID,
			ToolName:   ch.ToolName,
			Input:      rawJSON(ch.Input),
		})
		// The engine emits the tool input events before the approval, so the
		// block exists. Clearing ErrorText covers a re-request after a failed
		// attempt. A block that is genuinely missing still reaches the user:
		// the transcript renders unmatched approvals at its tail.
		if t := c.findTool(ch.ToolCallID); t != nil {
			t.Status = ToolAwaiting
			t.ErrorText = ""
		}
	case event.EventToolOutputAvailable:
		c.removeApprovalsByToolCall(ch.ToolCallID)
		if t := c.findTool(ch.ToolCallID); t != nil {
			t.Status = ToolSuccess
			t.Output = rawJSON(ch.Output)
			t.ErrorText = ""
		}
	case event.EventToolOutputError:
		c.removeApprovalsByToolCall(ch.ToolCallID)
		if t := c.findTool(ch.ToolCallID); t != nil {
			t.Status = ToolError
			t.ErrorText = ch.ErrorText
		}
	case event.EventToolOutputDenied:
		c.removeApprovalsByToolCall(ch.ToolCallID)
		if t := c.findTool(ch.ToolCallID); t != nil {
			t.Status = ToolDenied
			t.ErrorText = ""
		}
	case event.EventDone:
		c.EndRun(ch.Reason == "cancelled")
		return
	}
}

func (c *Chat) addApproval(a Approval) {
	for _, p := range c.Approvals {
		if p.ID == a.ID {
			return
		}
	}
	c.Approvals = append(c.Approvals, &a)
	c.Rev++
}

// RemoveApproval drops a resolved approval card.
func (c *Chat) RemoveApproval(id string) {
	for i, p := range c.Approvals {
		if p.ID == id {
			c.Approvals = append(c.Approvals[:i], c.Approvals[i+1:]...)
			c.Rev++
			return
		}
	}
}

// removeApprovalsByToolCall drops the pending approval bound to a tool call
// once that tool has a result or was denied. Its inline slot is the awaiting
// tool block, so once the block moves on the leftover approval would render
// as a stale card at the transcript tail.
func (c *Chat) removeApprovalsByToolCall(callID string) {
	if callID == "" || len(c.Approvals) == 0 {
		return
	}
	kept := c.Approvals[:0]
	for _, p := range c.Approvals {
		if p.ToolCallID != callID {
			kept = append(kept, p)
		}
	}
	if len(kept) != len(c.Approvals) {
		c.Approvals = kept
		c.Rev++
	}
}

// SetToolStatus updates a tool card's status by call id (e.g. denied when
// the user rejects its approval).
func (c *Chat) SetToolStatus(callID string, status ToolStatus) {
	if t := c.findTool(callID); t != nil {
		t.Status = status
		c.Rev++
	}
}

// Approval returns the pending approval with the given id, or nil.
func (c *Chat) Approval(id string) *Approval {
	for _, p := range c.Approvals {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// BeginRun marks the transcript as streaming a new assistant turn.
func (c *Chat) BeginRun(runID string) {
	c.Running = true
	c.RunID = runID
	c.cur()
	c.Rev++
}

// EndRun closes the streaming turn. cancelled stamps the bubble so the UI can
// show a stopped marker instead of an endless "running" state.
func (c *Chat) EndRun(cancelled bool) {
	c.Running = false
	// A finished run cannot still hold an approval; anything left here is a
	// stale card that would render at the transcript tail.
	c.Approvals = nil
	if n := len(c.Messages); n > 0 {
		m := c.Messages[n-1]
		if m.Role == RoleAssistant && m.Status == MsgRunning {
			if cancelled {
				m.Status = MsgCancelled
			} else {
				m.Status = ""
			}
			// A run that produced nothing leaves no empty bubble.
			if len(m.Blocks) == 0 {
				c.Messages = c.Messages[:n-1]
			}
		}
	}
	c.Rev++
}

// Text returns the concatenated text of a message's text blocks.
func (m *Message) Text() string {
	var sb strings.Builder
	for _, b := range m.Blocks {
		if b.Kind == BlockText {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

// Reasoning returns the concatenated reasoning text.
func (m *Message) Reasoning() string {
	var sb strings.Builder
	for _, b := range m.Blocks {
		if b.Kind == BlockReasoning {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

func rawJSON(v any) json.RawMessage {
	switch t := v.(type) {
	case nil:
		return nil
	case json.RawMessage:
		return t
	case string:
		if json.Valid([]byte(t)) {
			return json.RawMessage(t)
		}
		b, _ := json.Marshal(t)
		return b
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return nil
		}
		return b
	}
}

// Load rebuilds the transcript from persisted messages. msgs must be ordered
// by sequence (repo order). Consecutive model/tool rows fold into one
// assistant message; user rows become bubbles.
func (c *Chat) Load(msgs []*agent.AIMessage) {
	c.Messages = nil
	c.Approvals = nil
	var cur *Message

	chatMsgs := make([]ai.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		chatMsgs = append(chatMsgs, ai.ChatMessage{Role: m.Role, Content: m.Content, Parts: m.Parts, Status: m.Status})
	}
	pending := ai.PendingInterruptsFromHistory(chatMsgs)

	for _, m := range msgs {
		if m.Role == ai.RoleUser {
			cur = nil
			text := m.Content
			if text == "" {
				text = partsText(m.Parts)
			}
			c.Messages = append(c.Messages, &Message{
				ID:          m.ID,
				Role:        RoleUser,
				Blocks:      []*Block{{Kind: BlockText, Text: text}},
				Time:        m.Timestamp,
				Attachments: attachmentsFromParts(m.Parts),
			})
			continue
		}
		if cur == nil || cur.Role != RoleAssistant {
			cur = &Message{ID: "a" + m.ID, Role: RoleAssistant, RunID: m.RunID, Time: m.Timestamp}
			c.Messages = append(c.Messages, cur)
		}
		loadParts(cur, m)
		switch m.Status {
		case "cancelled":
			cur.Status = MsgCancelled
		case "denied":
			cur.Status = MsgDenied
		}
	}

	c.Rev++
	for _, p := range pending {
		if t := c.findTool(p.ToolCallID); t != nil {
			t.Status = ToolAwaiting
		}
		c.addApproval(Approval{ID: p.ApprovalID, ToolCallID: p.ToolCallID, ToolName: p.ToolName, Input: p.Input})
	}
}

func loadParts(m *Message, am *agent.AIMessage) {
	parts := ai.UnmarshalParts(am.Parts)
	if len(parts) == 0 && am.Content != "" {
		parts = []*genkitAI.Part{genkitAI.NewTextPart(am.Content)}
	}
	for _, p := range parts {
		if p == nil {
			continue
		}
		switch p.Kind {
		case genkitAI.PartText:
			if p.Text == "" {
				continue
			}
			appendText(m, BlockText, p.Text)
		case genkitAI.PartReasoning:
			if p.Text == "" {
				continue
			}
			appendText(m, BlockReasoning, p.Text)
		case genkitAI.PartToolRequest:
			if p.ToolRequest == nil {
				continue
			}
			t := &ToolCall{ID: p.ToolRequest.Ref, Name: p.ToolRequest.Name, Status: ToolRunning}
			if p.ToolRequest.Input != nil {
				t.Input = rawJSON(p.ToolRequest.Input)
			}
			m.Blocks = append(m.Blocks, &Block{Kind: BlockTool, Tool: t})
		case genkitAI.PartToolResponse:
			if p.ToolResponse == nil {
				continue
			}
			if t := m.findLocalTool(p.ToolResponse.Ref); t != nil {
				t.Output = rawJSON(p.ToolResponse.Output)
				t.Status = ToolSuccess
			}
		}
	}
}

func appendText(m *Message, kind BlockKind, text string) {
	last := len(m.Blocks) - 1
	if last >= 0 && m.Blocks[last].Kind == kind {
		m.Blocks[last].Text += text
		return
	}
	m.Blocks = append(m.Blocks, &Block{Kind: kind, Text: text})
}

func (m *Message) findLocalTool(callID string) *ToolCall {
	for _, b := range m.Blocks {
		if b.Kind == BlockTool && b.Tool != nil && b.Tool.ID == callID {
			return b.Tool
		}
	}
	return nil
}

func cloneAttachments(atts []Attachment) []Attachment {
	if len(atts) == 0 {
		return nil
	}
	return append([]Attachment(nil), atts...)
}

// attachmentsFromParts reads vela-path: media refs. Filename comes from
// part metadata; MIME from the part content type.
func attachmentsFromParts(partsJSON string) []Attachment {
	var out []Attachment
	for _, p := range ai.UnmarshalParts(partsJSON) {
		if p == nil || !p.IsMedia() || !strings.HasPrefix(p.Text, ai.PathURIPrefix) {
			continue
		}
		raw, err := url.QueryUnescape(strings.TrimPrefix(p.Text, ai.PathURIPrefix))
		if err != nil || strings.TrimSpace(raw) == "" {
			continue
		}
		abs := filepath.Clean(raw)
		if !filepath.IsAbs(abs) {
			continue
		}
		name := ""
		if p.Metadata != nil {
			if v, ok := p.Metadata["filename"]; ok {
				name = strings.TrimSpace(fmt.Sprint(v))
			}
		}
		if name == "" || name == "." || name == ".." {
			name = filepath.Base(abs)
		}
		out = append(out, Attachment{Path: abs, Filename: name, MIME: p.ContentType})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func partsText(partsJSON string) string {
	for _, p := range ai.UnmarshalParts(partsJSON) {
		if p != nil && p.Kind == genkitAI.PartText {
			return p.Text
		}
	}
	return ""
}
