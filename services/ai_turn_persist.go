package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"uuid"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/database"
)

const (
	streamFlushInterval = 200 * time.Millisecond
	streamFlushMinBytes = 512
)

// turnPersister owns mid-run and finalize writes for one Chat run_id.
//
// streamBuf holds text deltas since the last Checkpoint (a segment), not the
// full run transcript. Flushing extends the trailing text model or creates a
// provisional trailing model after tool rounds.
type turnPersister struct {
	repo      database.AISessionRepository
	logger    *zap.Logger
	sessionID string
	runID     string
	userMsgID string

	mu            sync.Mutex
	turnIDs       []string
	lastTurn      []ai.ChatMessage
	streamBuf     string
	provisionalID string
	lastFlush     time.Time
	sealed        bool
}

func newTurnPersister(repo database.AISessionRepository, logger *zap.Logger, sessionID, runID, userMsgID string) *turnPersister {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &turnPersister{
		repo:      repo,
		logger:    logger,
		sessionID: sessionID,
		runID:     runID,
		userMsgID: userMsgID,
	}
}

// UpsertStreamingText appends a text delta and throttles flush of durable model text.
func (p *turnPersister) UpsertStreamingText(delta string) {
	if p == nil || delta == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sealed {
		return
	}
	p.streamBuf += delta
	if p.shouldFlushStreamLocked() {
		if err := p.flushStreamLocked(); err != nil {
			p.logger.Error("flush streaming AI text failed",
				zap.String("sessionId", p.sessionID),
				zap.String("runId", p.runID),
				zap.Error(err),
			)
		}
	}
}

func (p *turnPersister) shouldFlushStreamLocked() bool {
	if p.streamBuf == "" {
		return false
	}
	if p.provisionalID == "" {
		return true
	}
	if len(p.streamBuf) >= streamFlushMinBytes {
		return true
	}
	return time.Since(p.lastFlush) >= streamFlushInterval
}

// Checkpoint replaces model/tool rows with the Genkit History turn slice (no PrepareTurn).
func (p *turnPersister) Checkpoint(turn []ai.ChatMessage) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sealed {
		return
	}
	turn = modelToolTurn(turn)
	if len(turn) == 0 {
		return
	}
	savedIDs := append([]string(nil), p.turnIDs...)
	// Reuse the trailing provisional model id at the stream slot (len(lastTurn)).
	if p.provisionalID != "" {
		idx := len(p.lastTurn)
		if idx < len(turn) && turn[idx].Role == ai.RoleModel {
			p.ensureTurnIDsLocked(idx + 1)
			p.turnIDs[idx] = p.provisionalID
		}
	}
	p.ensureTurnIDsLocked(len(turn))
	msgs := p.messagesFromTurnLocked(turn, false)
	if err := p.repo.ReplaceRunMessages(context.Background(), p.sessionID, p.runID, msgs); err != nil {
		p.turnIDs = savedIDs
		p.logger.Error("checkpoint AI turn failed",
			zap.String("sessionId", p.sessionID),
			zap.String("runId", p.runID),
			zap.Error(err),
		)
		return
	}
	p.streamBuf = ""
	p.provisionalID = ""
	p.lastTurn = append([]ai.ChatMessage(nil), turn...)
}

// Finalize writes PrepareTurn output, seals run_id, and marks user cancelled when needed.
func (p *turnPersister) Finalize(resp *ai.AgentResponse, cancelled bool) (*database.AIMessage, error) {
	if p == nil {
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sealed {
		return nil, nil
	}
	merged := cloneAgentResponse(resp)
	if p.streamBuf != "" {
		merged = mergeStreamSegment(merged, p.streamBuf)
	}
	prepared := ai.PrepareTurn(merged, cancelled)
	last, err := p.finalizePreparedLocked(prepared, cancelled)
	if err != nil {
		return last, err
	}
	p.streamBuf = ""
	p.provisionalID = ""
	return last, nil
}

// ForceFinalize seals using lastCheckpoint + streamBuf when Run never returned a response.
func (p *turnPersister) ForceFinalize() (*database.AIMessage, error) {
	if p == nil {
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sealed {
		return nil, nil
	}
	merged := mergeStreamSegment(&ai.AgentResponse{Turn: append([]ai.ChatMessage(nil), p.lastTurn...)}, p.streamBuf)
	prepared := ai.PrepareTurn(merged, true)
	last, err := p.finalizePreparedLocked(prepared, true)
	if err != nil {
		return last, err
	}
	p.streamBuf = ""
	p.provisionalID = ""
	return last, nil
}

func cloneAgentResponse(resp *ai.AgentResponse) *ai.AgentResponse {
	if resp == nil {
		return nil
	}
	return &ai.AgentResponse{
		Content: resp.Content,
		Turn:    append([]ai.ChatMessage(nil), resp.Turn...),
	}
}

// mergeStreamSegment applies a post-checkpoint text segment onto resp.
// segment is NOT the full transcript — it is appended to a trailing text model,
// or used as-is when it already contains the trailing content (dedupe).
func mergeStreamSegment(resp *ai.AgentResponse, segment string) *ai.AgentResponse {
	if segment == "" {
		return resp
	}
	if resp == nil {
		resp = &ai.AgentResponse{}
	}
	turn := append([]ai.ChatMessage(nil), resp.Turn...)
	if n := len(turn); n > 0 && turn[n-1].Role == ai.RoleModel && !chatHasToolRequest(turn[n-1]) {
		cur := turn[n-1].Content
		switch {
		case strings.HasSuffix(cur, segment):
			// Engine already applied the same bytes (or segment already merged).
		case strings.HasPrefix(segment, cur):
			turn[n-1].Content = segment
			turn[n-1].Parts = ai.UserTextPartsJSON(segment)
		default:
			combined := cur + segment
			turn[n-1].Content = combined
			turn[n-1].Parts = ai.UserTextPartsJSON(combined)
		}
		resp.Turn = turn
		resp.Content = turn[n-1].Content
		return resp
	}
	turn = append(turn, ai.ChatMessage{
		Role:    ai.RoleModel,
		Content: segment,
		Parts:   ai.UserTextPartsJSON(segment),
	})
	resp.Turn = turn
	if resp.Content == "" {
		resp.Content = segment
	}
	return resp
}

func (p *turnPersister) finalizePreparedLocked(prepared *ai.AgentResponse, cancelled bool) (*database.AIMessage, error) {
	var last *database.AIMessage
	if prepared != nil && len(prepared.Turn) > 0 {
		p.ensureTurnIDsLocked(len(prepared.Turn))
		p.lastTurn = append([]ai.ChatMessage(nil), prepared.Turn...)
		msgs := p.messagesFromTurnLocked(prepared.Turn, cancelled)
		if err := p.repo.ReplaceRunMessages(context.Background(), p.sessionID, p.runID, msgs); err != nil {
			return nil, err
		}
		p.lastTurn = append([]ai.ChatMessage(nil), prepared.Turn...)
		last = msgs[len(msgs)-1]
	}
	if err := p.repo.SealRunMessages(context.Background(), p.sessionID, p.runID); err != nil {
		return last, err
	}
	p.sealed = true
	if cancelled && p.userMsgID != "" {
		if err := p.repo.UpdateMessageStatus(context.Background(), p.userMsgID, database.MessageStatusCancelled); err != nil {
			p.logger.Error("mark cancelled user message failed",
				zap.String("sessionId", p.sessionID),
				zap.Error(err),
			)
		}
	}
	if session, err := p.repo.GetSession(context.Background(), p.sessionID); err == nil {
		session.UpdatedAt = time.Now()
		_ = p.repo.UpdateSession(context.Background(), session)
	}
	return last, nil
}

func (p *turnPersister) flushStreamLocked() error {
	if p.streamBuf == "" {
		return nil
	}
	segment := p.streamBuf

	// Extend trailing text-only model from the last checkpoint (same Genkit message).
	if n := len(p.lastTurn); n > 0 && p.lastTurn[n-1].Role == ai.RoleModel && !chatHasToolRequest(p.lastTurn[n-1]) {
		p.ensureTurnIDsLocked(n)
		id := p.turnIDs[n-1]
		combined := p.lastTurn[n-1].Content + segment
		parts := ai.UserTextPartsJSON(combined)
		if err := p.repo.UpdateMessageContent(context.Background(), id, combined, parts); err != nil {
			return err
		}
		p.provisionalID = id
		p.lastFlush = time.Now()
		return nil
	}

	// Empty turn or after tool round: durable trailing provisional model = segment only.
	if p.provisionalID != "" {
		parts := ai.UserTextPartsJSON(segment)
		if err := p.repo.UpdateMessageContent(context.Background(), p.provisionalID, segment, parts); err != nil {
			return err
		}
		p.lastFlush = time.Now()
		return nil
	}
	trailIdx := len(p.lastTurn)
	p.ensureTurnIDsLocked(trailIdx + 1)
	id := p.turnIDs[trailIdx]
	msgs := p.messagesFromTurnLocked(p.lastTurn, false)
	msgs = append(msgs, &database.AIMessage{
		ID:        id,
		SessionID: p.sessionID,
		Role:      ai.RoleModel,
		Content:   segment,
		Parts:     ai.UserTextPartsJSON(segment),
		RunID:     p.runID,
		Timestamp: time.Now(),
	})
	if err := p.repo.ReplaceRunMessages(context.Background(), p.sessionID, p.runID, msgs); err != nil {
		return err
	}
	p.provisionalID = id
	p.lastFlush = time.Now()
	return nil
}

func (p *turnPersister) ensureTurnIDsLocked(n int) {
	for len(p.turnIDs) < n {
		p.turnIDs = append(p.turnIDs, uuid.New().String())
	}
}

func (p *turnPersister) messagesFromTurnLocked(turn []ai.ChatMessage, cancelled bool) []*database.AIMessage {
	turn = modelToolTurn(turn)
	p.ensureTurnIDsLocked(len(turn))
	now := time.Now()
	msgs := make([]*database.AIMessage, 0, len(turn))
	for i, m := range turn {
		status := m.Status
		if cancelled && m.Role != ai.RoleTool {
			status = database.MessageStatusCancelled
		}
		msgs = append(msgs, &database.AIMessage{
			ID:        p.turnIDs[i],
			SessionID: p.sessionID,
			Role:      m.Role,
			Content:   m.Content,
			Parts:     m.Parts,
			Status:    status,
			RunID:     p.runID,
			Timestamp: now.Add(time.Duration(i) * time.Millisecond),
		})
	}
	return msgs
}

func modelToolTurn(turn []ai.ChatMessage) []ai.ChatMessage {
	out := make([]ai.ChatMessage, 0, len(turn))
	for _, m := range turn {
		if m.Role != ai.RoleModel && m.Role != ai.RoleTool {
			continue
		}
		out = append(out, m)
	}
	return out
}

func chatHasToolRequest(m ai.ChatMessage) bool {
	for _, part := range ai.UnmarshalParts(m.Parts) {
		if part != nil && part.IsToolRequest() {
			return true
		}
	}
	return false
}

// persistCancelledRunTurn writes PrepareTurn(cancelled) model/tool rows, seals run_id, marks users.
// Used by orphan recovery (no live turnPersister).
func persistCancelledRunTurn(
	repo database.AISessionRepository,
	logger *zap.Logger,
	sessionID, runID string,
	turn []ai.ChatMessage,
	modelToolIDs, userIDs []string,
) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	prepared := ai.PrepareTurn(&ai.AgentResponse{Turn: modelToolTurn(turn)}, true)
	if prepared != nil && len(prepared.Turn) > 0 {
		now := time.Now()
		out := make([]*database.AIMessage, 0, len(prepared.Turn))
		for i, m := range prepared.Turn {
			status := m.Status
			if m.Role != ai.RoleTool {
				status = database.MessageStatusCancelled
			}
			id := ""
			if i < len(modelToolIDs) {
				id = modelToolIDs[i]
			}
			if id == "" {
				id = uuid.New().String()
			}
			out = append(out, &database.AIMessage{
				ID:        id,
				SessionID: sessionID,
				Role:      m.Role,
				Content:   m.Content,
				Parts:     m.Parts,
				Status:    status,
				RunID:     runID,
				Timestamp: now.Add(time.Duration(i) * time.Millisecond),
			})
		}
		if err := repo.ReplaceRunMessages(context.Background(), sessionID, runID, out); err != nil {
			return err
		}
	}
	if err := repo.SealRunMessages(context.Background(), sessionID, runID); err != nil {
		return err
	}
	if len(userIDs) > 0 {
		if err := repo.UpdateMessageStatuses(context.Background(), userIDs, database.MessageStatusCancelled); err != nil {
			logger.Error("mark orphan user cancelled failed",
				zap.String("sessionId", sessionID),
				zap.String("runId", runID),
				zap.Error(err),
			)
		}
	}
	return nil
}
