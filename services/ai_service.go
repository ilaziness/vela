package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/ssh"
)

const EventAIStreamChunk = "eventAIStreamChunk"

// AIStreamChunkData is the Wails event payload for agent streaming (MUI-aligned).
type AIStreamChunkData struct {
	SessionID  string `json:"sessionId"`
	Type       string `json:"type"`
	ID         string `json:"id,omitempty"`
	Delta      string `json:"delta,omitempty"`
	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	ApprovalID string `json:"approvalId,omitempty"`
	Input      any    `json:"input,omitempty"`
	Output     any    `json:"output,omitempty"`
	ErrorText  string `json:"errorText,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

func init() {
	application.RegisterEvent[AIStreamChunkData](EventAIStreamChunk)
}

var ErrAINotEnabled = errors.New("AI 助手尚未启用，请前往「设置 → AI」完成配置并启用后再试")

type AIMessage struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Parts     string `json:"parts"`
	Timestamp int64  `json:"timestamp"`
}

type AISession struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type SSHContext struct {
	LinkID string `json:"link_id,omitempty"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	User   string `json:"user"`
}

type ChatRequest struct {
	SessionID  string      `json:"session_id"`
	NewMessage string      `json:"new_message"`
	SSHContext *SSHContext `json:"ssh_context,omitempty"`
}

type ChatResponse struct {
	Message AIMessage `json:"message"`
}

type ToolApprovalRequest struct {
	SessionID  string `json:"session_id"`
	ApprovalID string `json:"approval_id"`
	Approved   bool   `json:"approved"`
	Reason     string `json:"reason,omitempty"`
}

type approvalDecision struct {
	Approved bool
	Reason   string
}

type agentRun struct {
	cancel    context.CancelFunc
	approvals map[string]chan approvalDecision
	mu        sync.Mutex
}

type AIService struct {
	app           *application.App
	logger        *zap.Logger
	configService *ConfigService
	sshService    *SSHService
	engine        *ai.AIEngine
	sessionRepo   database.AISessionRepository

	engineMu sync.Mutex
	runsMu   sync.Mutex
	runs     map[string]*agentRun
}

func NewAIService(app *application.App, logger *zap.Logger, configService *ConfigService, sshService *SSHService, db *database.Database) *AIService {
	// Do not decrypt API keys or init Genkit here: the password prompt needs a
	// live UI, and a 60s timeout would both delay startup and leave engine nil.
	return &AIService{
		app: app, logger: logger, configService: configService, sshService: sshService,
		sessionRepo: db.AISessionRepo, runs: make(map[string]*agentRun),
	}
}

func (s *AIService) GetConfig() (*AIConfig, error) {
	if s.configService == nil {
		return nil, fmt.Errorf("config service not initialized")
	}
	cfg := s.configService.aiConfig()
	return &cfg, nil
}

func (s *AIService) engineConfig(cfg *AIConfig) (*ai.Config, error) {
	out := *cfg
	if !cfg.Provider.NeedsAPIKey() {
		out.APIKey = ""
		return &out, nil
	}
	apiKey := cfg.APIKey
	if apiKey == "" && cfg.APIKeyEncrypted != "" {
		decrypted, err := s.configService.decryptAPIKey(cfg.APIKeyEncrypted)
		if err != nil {
			return nil, err
		}
		apiKey = decrypted
	}
	out.APIKey = apiKey
	return &out, nil
}

func (s *AIService) engineReady() bool {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	return s.engine != nil && s.engine.IsEnabled()
}

func (s *AIService) ensureEngine(cfg *AIConfig) error {
	if cfg == nil || !cfg.Enabled {
		return ErrAINotEnabled
	}
	if s.engineReady() {
		return nil
	}
	return s.reloadEngine(cfg)
}

func (s *AIService) reloadEngine(cfg *AIConfig) error {
	if cfg == nil || !cfg.Enabled {
		s.engineMu.Lock()
		defer s.engineMu.Unlock()
		if s.engine != nil {
			_ = s.engine.SetConfig(&ai.Config{Enabled: false})
		}
		return nil
	}
	engineCfg, err := s.engineConfig(cfg)
	if err != nil {
		return err
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if s.engine == nil {
		s.engine = ai.NewAIEngine(s.logger)
	}
	if err := s.engine.SetConfig(engineCfg); err != nil {
		return fmt.Errorf("set ai engine config failed: %w", err)
	}
	return nil
}

func (s *AIService) SaveConfig(cfg AIConfig) error {
	runtimeAPIKey := cfg.APIKey
	if cfg.APIKey != "" && cfg.APIKeyEncrypted == "" {
		encrypted, err := s.configService.encryptAPIKey(cfg.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt api key failed: %w", err)
		}
		cfg.APIKeyEncrypted = encrypted
		cfg.APIKey = ""
	}
	if err := s.configService.saveAI(cfg); err != nil {
		return err
	}
	if runtimeAPIKey != "" {
		cfg.APIKey = runtimeAPIKey
	}
	return s.reloadEngine(&cfg)
}

func (s *AIService) ResetConfig() error {
	return s.SaveConfig(AIConfig{
		Enabled: false, Provider: ai.ProviderOllama, Model: "llama3.2",
		Endpoint: "http://localhost:11434", Temperature: 0.7, MaxTokens: 2048,
		EchoSSHCommands: false, MaxAgentTurns: ai.DefaultMaxAgentTurns, ExecTimeoutSec: ai.DefaultExecTimeoutSec,
	})
}

func (s *AIService) GetProviders() []ai.ProviderInfo { return ai.GetAllProviders() }

func (s *AIService) CreateSession() (*AISession, error) {
	session := &database.AISession{Title: "新会话"}
	if err := s.sessionRepo.CreateSession(context.Background(), session); err != nil {
		return nil, err
	}
	return toAISession(session), nil
}
func (s *AIService) GetSession(id string) (*AISession, error) {
	session, err := s.sessionRepo.GetSession(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return toAISession(session), nil
}
func (s *AIService) ListSessions(limit int) ([]*AISession, error) {
	sessions, err := s.sessionRepo.ListSessions(context.Background(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]*AISession, len(sessions))
	for i, sess := range sessions {
		out[i] = toAISession(sess)
	}
	return out, nil
}
func (s *AIService) DeleteSession(id string) error {
	_ = s.StopGeneration(id)
	return s.sessionRepo.DeleteSession(context.Background(), id)
}
func (s *AIService) ListMessages(sessionID string) ([]*AIMessage, error) {
	messages, err := s.sessionRepo.ListMessages(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]*AIMessage, len(messages))
	for i, m := range messages {
		out[i] = toAIMessage(m)
	}
	return out, nil
}
func (s *AIService) UpdateSession(session *AISession) error {
	return s.sessionRepo.UpdateSession(context.Background(), &database.AISession{
		ID: session.ID, Title: session.Title,
		CreatedAt: time.Unix(session.CreatedAt, 0), UpdatedAt: time.Unix(session.UpdatedAt, 0),
	})
}

func (s *AIService) Chat(req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("chat request is nil")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	newMessage := strings.TrimSpace(req.NewMessage)
	if sessionID == "" || newMessage == "" {
		return nil, fmt.Errorf("session id and message are required")
	}
	cfg, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrAINotEnabled
	}
	if err := s.ensureEngine(cfg); err != nil {
		s.logger.Error("AI engine not ready",
			zap.String("sessionId", sessionID),
			zap.String("detail", err.Error()),
		)
		return nil, err
	}

	_ = s.StopGeneration(sessionID)

	history, err := s.sessionRepo.ListMessages(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]ai.ChatMessage, len(history))
	for i, msg := range history {
		messages[i] = ai.ChatMessage{Role: msg.Role, Content: msg.Content, Parts: msg.Parts}
	}
	userMsg := &database.AIMessage{
		SessionID: sessionID,
		Role:      ai.RoleUser,
		Content:   newMessage,
		Parts:     ai.UserTextPartsJSON(newMessage),
		Timestamp: time.Now(),
	}
	if err := s.sessionRepo.CreateMessage(context.Background(), userMsg); err != nil {
		return nil, err
	}

	linkID, sshCtx, remoteInfo := s.resolveSSHContext(req.SSHContext)
	enableSSH := linkID != ""
	aiReq := &ai.AgentRequest{
		Messages:     messages,
		NewMessage:   newMessage,
		SystemPrompt: ai.BuildSystemPrompt(sshCtx, remoteInfo, enableSSH),
		EnableSSH:    enableSSH,
		LinkID:       linkID,
	}
	if session, err := s.sessionRepo.GetSession(context.Background(), sessionID); err == nil && len(history) == 0 && (session.Title == "" || session.Title == "新会话") {
		title := []rune(newMessage)
		if len(title) > 20 {
			title = title[:20]
		}
		session.Title = string(title)
		if session.Title == "" {
			session.Title = "新会话"
		}
		_ = s.sessionRepo.UpdateSession(context.Background(), session)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	run := &agentRun{cancel: cancel, approvals: make(map[string]chan approvalDecision)}
	s.runsMu.Lock()
	s.runs[sessionID] = run
	s.runsMu.Unlock()
	defer func() {
		cancel()
		s.runsMu.Lock()
		if s.runs[sessionID] == run {
			delete(s.runs, sessionID)
		}
		s.runsMu.Unlock()
		run.mu.Lock()
		for id, ch := range run.approvals {
			close(ch)
			delete(run.approvals, id)
		}
		run.mu.Unlock()
	}()

	host := &aiHost{svc: s, run: run, cfg: cfg}
	deps := ai.ToolDeps{
		LinkID:   linkID,
		SSH:      host,
		Approval: host,
	}

	aiResp, err := s.engine.Run(ctx, aiReq, deps, func(ev ai.StreamEvent) error {
		if ev.Type != ai.EventTextDelta && ev.Type != ai.EventReasoningDelta {
			s.logger.Info("ai stream",
				zap.String("sessionId", sessionID),
				zap.String("type", ev.Type),
				zap.String("tool", ev.ToolName),
				zap.String("approvalId", ev.ApprovalID),
			)
		}
		s.app.Event.Emit(EventAIStreamChunk, AIStreamChunkData{
			SessionID:  sessionID,
			Type:       ev.Type,
			ID:         ev.ID,
			Delta:      ev.Delta,
			ToolCallID: ev.ToolCallID,
			ToolName:   ev.ToolName,
			ApprovalID: ev.ApprovalID,
			Input:      jsonValue(ev.Input),
			Output:     jsonValue(ev.Output),
			ErrorText:  ev.ErrorText,
			Reason:     ev.Reason,
		})
		return nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			s.logger.Info("AI generation stopped", zap.String("sessionId", sessionID))
			if s.isCurrentRun(sessionID, run) {
				_, _ = s.persistTurn(sessionID, aiResp)
			}
			return nil, fmt.Errorf("generation stopped")
		}
		msg := ai.PublicGenerateError(err)
		s.logger.Error("AI chat failed",
			zap.String("sessionId", sessionID),
			zap.String("error", msg),
			zap.String("detail", err.Error()),
		)
		s.app.Event.Emit(EventAIStreamChunk, AIStreamChunkData{
			SessionID: sessionID,
			Type:      ai.EventTextDelta,
			Delta:     msg,
		})
		if s.isCurrentRun(sessionID, run) {
			_, _ = s.persistTurn(sessionID, aiResp)
			last := &database.AIMessage{
				SessionID: sessionID,
				Role:      ai.RoleModel,
				Content:   msg,
				Parts:     ai.UserTextPartsJSON(msg),
				Timestamp: time.Now(),
			}
			_ = s.sessionRepo.CreateMessage(context.Background(), last)
			return &ChatResponse{Message: *toAIMessage(last)}, nil
		}
		return &ChatResponse{Message: AIMessage{SessionID: sessionID, Role: ai.RoleModel, Content: msg, Parts: "[]"}}, nil
	}

	last, persistErr := s.persistIfCurrent(sessionID, run, aiResp)
	if persistErr != nil {
		return nil, persistErr
	}
	if last == nil {
		content := ""
		if aiResp != nil {
			content = aiResp.Content
		}
		last = &database.AIMessage{SessionID: sessionID, Role: ai.RoleModel, Content: content, Parts: "[]", Timestamp: time.Now()}
	}
	return &ChatResponse{Message: *toAIMessage(last)}, nil
}

func (s *AIService) isCurrentRun(sessionID string, run *agentRun) bool {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	return run != nil && s.runs[sessionID] == run
}

func (s *AIService) persistIfCurrent(sessionID string, run *agentRun, resp *ai.AgentResponse) (*database.AIMessage, error) {
	if !s.isCurrentRun(sessionID, run) {
		return nil, nil
	}
	return s.persistTurn(sessionID, resp)
}

func (s *AIService) persistTurn(sessionID string, resp *ai.AgentResponse) (*database.AIMessage, error) {
	if resp == nil || len(resp.Turn) == 0 {
		return nil, nil
	}
	ts := time.Now()
	msgs := make([]*database.AIMessage, 0, len(resp.Turn))
	for _, m := range resp.Turn {
		msgs = append(msgs, &database.AIMessage{
			SessionID: sessionID,
			Role:      m.Role,
			Content:   m.Content,
			Parts:     m.Parts,
			Timestamp: ts,
		})
	}
	if err := s.sessionRepo.CreateMessages(context.Background(), msgs); err != nil {
		return nil, err
	}
	if session, err := s.sessionRepo.GetSession(context.Background(), sessionID); err == nil {
		session.UpdatedAt = time.Now()
		_ = s.sessionRepo.UpdateSession(context.Background(), session)
	}
	return msgs[len(msgs)-1], nil
}

// RespondToolApproval unblocks a pending tool approval for the session.
func (s *AIService) RespondToolApproval(req *ToolApprovalRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	approvalID := strings.TrimSpace(req.ApprovalID)
	if sessionID == "" || approvalID == "" {
		return fmt.Errorf("session_id and approval_id are required")
	}
	s.runsMu.Lock()
	run := s.runs[sessionID]
	s.runsMu.Unlock()
	if run == nil {
		return fmt.Errorf("no active generation for session")
	}
	run.mu.Lock()
	ch := run.approvals[approvalID]
	run.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("unknown or expired approval id")
	}
	select {
	case ch <- approvalDecision{Approved: req.Approved, Reason: req.Reason}:
		return nil
	default:
		return fmt.Errorf("approval already resolved")
	}
}

// StopGeneration cancels the active agent run for a session.
func (s *AIService) StopGeneration(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	s.runsMu.Lock()
	run := s.runs[sessionID]
	s.runsMu.Unlock()
	if run == nil {
		return nil
	}
	run.cancel()
	run.mu.Lock()
	for _, ch := range run.approvals {
		select {
		case ch <- approvalDecision{Approved: false, Reason: "stopped"}:
		default:
		}
	}
	run.mu.Unlock()
	return nil
}

// aiHost adapts SSH + approval for internal/ai ports.
type aiHost struct {
	svc *AIService
	run *agentRun
	cfg *AIConfig
}

func (h *aiHost) Exec(ctx context.Context, linkID, command string) (string, error) {
	timeout := time.Duration(h.cfg.EffectiveExecTimeoutSec()) * time.Second
	return h.svc.sshService.exec(ctx, linkID, command, timeout, ssh.DefaultExecMaxOut)
}

func (h *aiHost) Annotate(ctx context.Context, linkID, notice string) error {
	_ = ctx
	return h.svc.sshService.annotateSession(linkID, notice)
}

func (h *aiHost) Open(approvalID string) {
	if approvalID == "" || h.run == nil {
		return
	}
	h.run.mu.Lock()
	defer h.run.mu.Unlock()
	if _, ok := h.run.approvals[approvalID]; ok {
		return
	}
	h.run.approvals[approvalID] = make(chan approvalDecision, 1)
}

func (h *aiHost) Wait(ctx context.Context, approvalID string) (bool, string, error) {
	if h.run == nil {
		return false, "cancelled", fmt.Errorf("no active generation")
	}
	h.run.mu.Lock()
	ch := h.run.approvals[approvalID]
	if ch == nil {
		ch = make(chan approvalDecision, 1)
		h.run.approvals[approvalID] = ch
	}
	h.run.mu.Unlock()
	defer func() {
		h.run.mu.Lock()
		delete(h.run.approvals, approvalID)
		h.run.mu.Unlock()
	}()

	select {
	case d, ok := <-ch:
		if !ok {
			return false, "cancelled", context.Canceled
		}
		return d.Approved, d.Reason, nil
	case <-ctx.Done():
		select {
		case d, ok := <-ch:
			if !ok {
				return false, "cancelled", context.Canceled
			}
			return d.Approved, d.Reason, nil
		default:
			return false, "cancelled", ctx.Err()
		}
	}
}

func (s *AIService) resolveSSHContext(ctx *SSHContext) (linkID string, prompt *ai.SSHPromptContext, info *ssh.RemoteSystemInfo) {
	if ctx == nil || s.sshService == nil {
		return "", nil, nil
	}
	linkID = strings.TrimSpace(ctx.LinkID)
	host := strings.TrimSpace(ctx.Host)
	user := strings.TrimSpace(ctx.User)
	if linkID == "" || host == "" || user == "" || ctx.Port <= 0 {
		return "", nil, nil
	}
	if !s.sshService.hasSession(linkID) {
		return "", nil, nil
	}
	return linkID, &ai.SSHPromptContext{Host: host, Port: ctx.Port, User: user}, s.sshService.GetOrFetchRemoteSystemInfo(linkID, host)
}

func toAISession(s *database.AISession) *AISession {
	return &AISession{ID: s.ID, Title: s.Title, CreatedAt: s.CreatedAt.Unix(), UpdatedAt: s.UpdatedAt.Unix()}
}
func toAIMessage(m *database.AIMessage) *AIMessage {
	return &AIMessage{ID: m.ID, SessionID: m.SessionID, Role: m.Role, Content: m.Content, Parts: m.Parts, Timestamp: m.Timestamp.Unix()}
}

func jsonValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}
