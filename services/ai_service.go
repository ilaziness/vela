package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"uuid"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/ai/tool"
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
var ErrGenerationStopped = errors.New("generation stopped")

type AIMessage struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Parts     string `json:"parts"`
	Status    string `json:"status"` // "" | cancelled | denied
	RunID     string `json:"run_id,omitempty"`
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
	RunID      string      `json:"run_id,omitempty"`
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
	runID        string
	cancel       context.CancelFunc
	approvals    map[string]chan approvalDecision
	sshApprovals map[string]struct{} // approvalIDs that will SSH-Exec after approve
	persister    *turnPersister
	execCancel   context.CancelFunc // cancels the in-flight run_ssh_command Exec only
	execPending  bool               // approved, Exec not registered yet (approve→Exec race)
	execAbort    bool               // CancelSSHExec while execPending
	mu           sync.Mutex
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
	// cancelled records run IDs cancelled via CancelChat before (or while) Chat registers.
	cancelled map[string]struct{}
}

func NewAIService(app *application.App, logger *zap.Logger, configService *ConfigService, sshService *SSHService, db *database.Database) *AIService {
	// Do not decrypt API keys or init Genkit here: the password prompt needs a
	// live UI, and a 60s timeout would both delay startup and leave engine nil.
	return &AIService{
		app: app, logger: logger, configService: configService, sshService: sshService,
		sessionRepo: db.AISessionRepo, runs: make(map[string]*agentRun), cancelled: make(map[string]struct{}),
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
	s.engine.SetUserDataDir(s.userDataDir())
	if err := s.engine.SetConfig(engineCfg); err != nil {
		return fmt.Errorf("set ai engine config failed: %w", err)
	}
	return nil
}

func (s *AIService) SaveConfig(cfg AIConfig) error {
	return s.saveConfig(cfg, false)
}

func (s *AIService) saveConfig(cfg AIConfig, reset bool) error {
	runtimeAPIKey := cfg.APIKey

	// Always re-encrypt when plaintext is present so stale ciphertext cannot leave
	// a new key persisted in cleartext beside the old encrypted value.
	if cfg.APIKey != "" {
		encrypted, err := s.configService.encryptAPIKey(cfg.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt api key failed: %w", err)
		}
		cfg.APIKeyEncrypted = encrypted
		cfg.APIKey = ""
	}
	if cfg.WebSearch.APIKey != "" {
		encrypted, err := s.configService.encryptAPIKey(cfg.WebSearch.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt web search api key failed: %w", err)
		}
		cfg.WebSearch.APIKeyEncrypted = encrypted
		cfg.WebSearch.APIKey = ""
	}
	if !reset {
		prev, prevErr := s.GetConfig()
		// Preserve existing ciphertext when the UI leaves the key blank.
		if cfg.APIKey == "" && cfg.APIKeyEncrypted == "" && prevErr == nil && prev != nil {
			cfg.APIKeyEncrypted = prev.APIKeyEncrypted
		}
		// Search keys are provider-scoped: never reuse another vendor's ciphertext.
		if cfg.WebSearch.APIKey == "" && cfg.WebSearch.APIKeyEncrypted == "" &&
			prevErr == nil && prev != nil &&
			cfg.WebSearch.Provider == prev.WebSearch.Provider &&
			tool.ValidSearchProvider(cfg.WebSearch.Provider) {
			cfg.WebSearch.APIKeyEncrypted = prev.WebSearch.APIKeyEncrypted
		}
	}

	// Unconfigured provider must not keep a usable encrypted key on disk.
	if !tool.ValidSearchProvider(cfg.WebSearch.Provider) {
		cfg.WebSearch.Provider = ""
		cfg.WebSearch.APIKey = ""
		cfg.WebSearch.APIKeyEncrypted = ""
	}

	cfg.Filesystem.AllowedDirs = tool.SanitizeAllowedDirs(cfg.Filesystem.AllowedDirs)

	if err := s.configService.saveAI(cfg); err != nil {
		return err
	}
	if runtimeAPIKey != "" {
		cfg.APIKey = runtimeAPIKey
	}
	// Web search keys stay on the Chat RunDeps path only — not in Genkit engine config.
	return s.reloadEngine(&cfg)
}

func (s *AIService) ResetConfig() error {
	return s.saveConfig(AIConfig{
		Enabled: false, Provider: ai.ProviderOllama, Model: "llama3.2",
		Endpoint: "http://localhost:11434", Temperature: 0.7, MaxTokens: 2048,
		EchoSSHCommands: false, MaxAgentTurns: ai.DefaultMaxAgentTurns,
		Filesystem: ai.FileSystemConfig{AllowedDirs: nil},
		WebSearch:  ai.WebSearchConfig{},
	}, true)
}

// ResetToolsConfig clears filesystem allowlist and web search settings only.
func (s *AIService) ResetToolsConfig() error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.APIKey = ""
	out.Filesystem = ai.FileSystemConfig{}
	out.WebSearch = ai.WebSearchConfig{}
	// reset=true skips key-preserve so cleared WebSearch stays empty; LLM ciphertext
	// remains on out from GetConfig.
	return s.saveConfig(out, true)
}

func (s *AIService) GetProviders() []ai.ProviderInfo { return ai.GetAllProviders() }

func (s *AIService) GetWebSearchProviders() []tool.SearchProviderInfo {
	return tool.GetAllSearchProviders()
}

// ValidateAllowedDir checks that path is an existing local directory and returns its absolute form.
func (s *AIService) ValidateAllowedDir(path string) (string, error) {
	return tool.ValidateAllowedDir(path)
}

func (s *AIService) resolveWebSearchRuntime(cfg *AIConfig) (tool.WebSearchRuntime, error) {
	if cfg == nil || !tool.ValidSearchProvider(cfg.WebSearch.Provider) {
		return tool.WebSearchRuntime{}, nil
	}
	key := strings.TrimSpace(cfg.WebSearch.APIKey)
	if key == "" && cfg.WebSearch.APIKeyEncrypted != "" {
		decrypted, err := s.configService.decryptAPIKey(cfg.WebSearch.APIKeyEncrypted)
		if err != nil {
			return tool.WebSearchRuntime{}, err
		}
		key = decrypted
	}
	if key == "" {
		return tool.WebSearchRuntime{}, nil
	}
	return tool.WebSearchRuntime{
		Provider: cfg.WebSearch.Provider,
		APIKey:   key,
	}, nil
}

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
	runID := strings.TrimSpace(req.RunID)
	if sessionID == "" || newMessage == "" {
		return nil, fmt.Errorf("session id and message are required")
	}
	if runID == "" {
		runID = uuid.New().String()
	}
	// Clear pre-register cancel marks even when Chat fails before run registration.
	defer s.clearCancelledRun(runID)
	if err := s.errIfRunCancelled(sessionID, runID); err != nil {
		return nil, err
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
	if err := s.errIfRunCancelled(sessionID, runID); err != nil {
		return nil, err
	}

	_ = s.StopGeneration(sessionID)
	// Let the previous run Finalize/seal before we build the next prompt, so history
	// does not include an in-flight incomplete tool round from the superseded Chat.
	// If it hangs past the wait, steal it: ForceFinalize + drop from map so orphan
	// recovery cannot race a still-running Finalize on the same run_id.
	s.waitSessionRunIdle(sessionID, 2*time.Second)
	s.forceFinalizeSessionRun(sessionID)
	s.sealOrphanRuns(sessionID)

	history, err := s.sessionRepo.ListMessages(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]ai.ChatMessage, 0, len(history))
	for _, msg := range history {
		// Skip still-unsealed rows (defensive if a prior run hung past the wait).
		if msg.RunID != "" {
			continue
		}
		// Status is UI-only; Genkit replay uses role/content/parts.
		messages = append(messages, ai.ChatMessage{Role: msg.Role, Content: msg.Content, Parts: msg.Parts})
	}
	userMsg := &database.AIMessage{
		SessionID: sessionID,
		Role:      ai.RoleUser,
		Content:   newMessage,
		Parts:     ai.UserTextPartsJSON(newMessage),
		RunID:     runID,
		Timestamp: time.Now(),
	}
	if err := s.sessionRepo.CreateMessage(context.Background(), userMsg); err != nil {
		return nil, err
	}

	linkID, sshCtx, remoteInfo := s.resolveSSHContext(req.SSHContext)
	enableSSH := linkID != ""
	allowDirs := cfg.EffectiveAllowedDirs()
	webSearch, err := s.resolveWebSearchRuntime(cfg)
	if err != nil {
		return nil, err
	}
	enableFS := len(allowDirs) > 0
	enableWebSearch := webSearch.APIKey != "" && tool.ValidSearchProvider(webSearch.Provider)
	aiReq := &ai.AgentRequest{
		Messages:   messages,
		NewMessage: newMessage,
		SystemPrompt: ai.BuildSystemPromptOpts(ai.PromptOptions{
			SSH:             sshCtx,
			Remote:          remoteInfo,
			EnableSSH:       enableSSH,
			EnableFS:        enableFS,
			AllowDirs:       allowDirs,
			EnableWebSearch: enableWebSearch,
		}),
		EnableSSH: enableSSH,
		LinkID:    linkID,
	}
	if session, err := s.sessionRepo.GetSession(context.Background(), sessionID); err == nil && len(messages) == 0 && (session.Title == "" || session.Title == "新会话") {
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

	ctx, cancel := context.WithCancel(context.Background())
	persister := newTurnPersister(s.sessionRepo, s.logger, sessionID, runID, userMsg.ID)
	run := &agentRun{
		runID:        runID,
		cancel:       cancel,
		approvals:    make(map[string]chan approvalDecision),
		sshApprovals: make(map[string]struct{}),
		persister:    persister,
	}
	s.runsMu.Lock()
	if _, ok := s.cancelled[runID]; ok {
		s.runsMu.Unlock()
		cancel()
		s.logger.Info("AI generation stopped before start",
			zap.String("sessionId", sessionID),
			zap.String("runId", runID),
		)
		s.markMessageCancelled(sessionID, userMsg.ID)
		_ = s.sessionRepo.SealRunMessages(context.Background(), sessionID, runID)
		return nil, ErrGenerationStopped
	}
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

	host := &aiHost{svc: s, run: run}
	rd := ai.RunDeps{
		Tools: tool.Deps{
			LinkID:           linkID,
			SSH:              host,
			Approval:         host,
			MarkSSHApproval:  host.MarkSSHApproval,
			ArmSSHExecCancel: host.ArmSSHExecCancel,
			AllowedDirs:      allowDirs,
			WebSearch:        webSearch,
		},
		OnCheckpoint: persister.Checkpoint,
	}

	aiResp, err := s.engine.Run(ctx, aiReq, rd, func(ev ai.StreamEvent) error {
		if ev.Type == ai.EventTextDelta && ev.Delta != "" {
			persister.UpsertStreamingText(ev.Delta)
		}
		if !s.isCurrentRun(sessionID, run) {
			return nil
		}
		if ev.Type != ai.EventTextDelta && ev.Type != ai.EventReasoningDelta {
			s.logger.Info("ai stream",
				zap.String("sessionId", sessionID),
				zap.String("type", ev.Type),
				zap.String("tool", ev.ToolName),
				zap.String("approvalId", ev.ApprovalID),
			)
		}
		s.emitAIStream(AIStreamChunkData{
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
		if isChatStopped(err, ctx) {
			s.logger.Info("AI generation stopped",
				zap.String("sessionId", sessionID),
				zap.String("runId", runID),
			)
			if _, perr := persister.Finalize(aiResp, true); perr != nil {
				s.logger.Error("persist stopped AI turn failed",
					zap.String("sessionId", sessionID),
					zap.Error(perr),
				)
			}
			return nil, ErrGenerationStopped
		}
		msg := ai.PublicGenerateError(err)
		s.logger.Error("AI chat failed",
			zap.String("sessionId", sessionID),
			zap.String("error", msg),
			zap.String("detail", err.Error()),
		)
		if s.isCurrentRun(sessionID, run) {
			s.emitAIStream(AIStreamChunkData{
				SessionID: sessionID,
				Type:      ai.EventTextDelta,
				Delta:     msg,
			})
		}
		errResp := mergeErrorIntoResponse(aiResp, msg)
		last, perr := persister.Finalize(errResp, false)
		if perr != nil {
			s.logger.Error("persist failed AI turn failed",
				zap.String("sessionId", sessionID),
				zap.Error(perr),
			)
			return nil, perr
		}
		if last == nil {
			// Finalize sealed an empty turn; still return the public error for the UI.
			last = &database.AIMessage{
				SessionID: sessionID,
				Role:      ai.RoleModel,
				Content:   msg,
				Parts:     ai.UserTextPartsJSON(msg),
				Timestamp: time.Now(),
			}
		}
		return &ChatResponse{Message: *toAIMessage(last)}, nil
	}

	last, persistErr := persister.Finalize(aiResp, false)
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

func mergeErrorIntoResponse(resp *ai.AgentResponse, msg string) *ai.AgentResponse {
	out := &ai.AgentResponse{}
	if resp != nil {
		out.Turn = append([]ai.ChatMessage(nil), resp.Turn...)
		out.Content = resp.Content
	}
	if n := len(out.Turn); n > 0 && out.Turn[n-1].Role == ai.RoleModel && out.Turn[n-1].Content == msg {
		out.Content = msg
		return out
	}
	out.Turn = append(out.Turn, ai.ChatMessage{
		Role:    ai.RoleModel,
		Content: msg,
		Parts:   ai.UserTextPartsJSON(msg),
	})
	out.Content = msg
	return out
}

func isChatStopped(err error, ctx context.Context) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if ctx == nil {
		return false
	}
	return errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func (s *AIService) isCurrentRun(sessionID string, run *agentRun) bool {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	return run != nil && s.runs[sessionID] == run
}

func (s *AIService) emitAIStream(data AIStreamChunkData) {
	if s == nil || s.app == nil {
		return
	}
	s.app.Event.Emit(EventAIStreamChunk, data)
}

// waitSessionRunIdle waits until no agentRun is registered for sessionID, or timeout.
func (s *AIService) waitSessionRunIdle(sessionID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.runsMu.Lock()
		_, busy := s.runs[sessionID]
		s.runsMu.Unlock()
		if !busy {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// forceFinalizeSessionRun steals a still-registered run after wait timed out: persists
// whatever was checkpointed/streamed, marks the persister sealed, and removes the run
// so a later Chat cannot orphan-seal the same run_id while Finalize is in flight.
func (s *AIService) forceFinalizeSessionRun(sessionID string) {
	s.runsMu.Lock()
	run := s.runs[sessionID]
	if run == nil {
		s.runsMu.Unlock()
		return
	}
	delete(s.runs, sessionID)
	s.runsMu.Unlock()

	s.rejectApprovalsAndCancel(run)
	if run.persister == nil {
		return
	}
	if _, err := run.persister.ForceFinalize(); err != nil {
		s.logger.Error("force finalize superseded AI turn failed",
			zap.String("sessionId", sessionID),
			zap.String("runId", run.runID),
			zap.Error(err),
		)
	}
}

func (s *AIService) markMessageCancelled(sessionID, msgID string) {
	if msgID == "" {
		return
	}
	if uerr := s.sessionRepo.UpdateMessageStatus(context.Background(), msgID, database.MessageStatusCancelled); uerr != nil {
		s.logger.Error("mark cancelled user message failed",
			zap.String("sessionId", sessionID),
			zap.Error(uerr),
		)
	}
}

// liveRunIDs returns run IDs currently registered in memory.
func (s *AIService) liveRunIDs() map[string]struct{} {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	out := make(map[string]struct{}, len(s.runs))
	for _, run := range s.runs {
		if run != nil && run.runID != "" {
			out[run.runID] = struct{}{}
		}
	}
	return out
}

// RecoverOrphanTurns seals any unsealed run_id rows left by a crash (no live run).
func (s *AIService) RecoverOrphanTurns() {
	s.sealOrphanRuns("")
}

// sealOrphanRuns finalizes DB rows with non-empty run_id that have no live agentRun.
// If sessionID is non-empty, only that session is scanned.
func (s *AIService) sealOrphanRuns(sessionID string) {
	refs, err := s.sessionRepo.ListUnsealedRuns(context.Background(), sessionID)
	if err != nil {
		s.logger.Error("list unsealed AI runs failed", zap.Error(err))
		return
	}
	live := s.liveRunIDs()
	for _, ref := range refs {
		if _, ok := live[ref.RunID]; ok {
			continue
		}
		if err := s.sealOrphanRun(ref.SessionID, ref.RunID); err != nil {
			s.logger.Error("seal orphan AI run failed",
				zap.String("sessionId", ref.SessionID),
				zap.String("runId", ref.RunID),
				zap.Error(err),
			)
		}
	}
}

func (s *AIService) sealOrphanRun(sessionID, runID string) error {
	msgs, err := s.sessionRepo.ListMessagesByRunID(context.Background(), sessionID, runID)
	if err != nil {
		return err
	}
	var turn []ai.ChatMessage
	var userIDs []string
	var modelToolIDs []string
	for _, m := range msgs {
		if m == nil {
			continue
		}
		switch m.Role {
		case ai.RoleUser:
			userIDs = append(userIDs, m.ID)
		case ai.RoleModel, ai.RoleTool:
			turn = append(turn, ai.ChatMessage{Role: m.Role, Content: m.Content, Parts: m.Parts, Status: m.Status})
			modelToolIDs = append(modelToolIDs, m.ID)
		}
	}
	return persistCancelledRunTurn(s.sessionRepo, s.logger, sessionID, runID, turn, modelToolIDs, userIDs)
}

// FlushAndStop cancels active runs, force-finalizes stragglers, and seals orphans before DB close.
func (s *AIService) FlushAndStop() {
	s.runsMu.Lock()
	runs := make([]*agentRun, 0, len(s.runs))
	for _, run := range s.runs {
		if run != nil {
			runs = append(runs, run)
		}
	}
	s.runsMu.Unlock()
	for _, run := range runs {
		s.rejectApprovalsAndCancel(run)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.runsMu.Lock()
		n := len(s.runs)
		s.runsMu.Unlock()
		if n == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.runsMu.Lock()
	leftover := make([]*agentRun, 0, len(s.runs))
	for sid, run := range s.runs {
		if run != nil {
			leftover = append(leftover, run)
		}
		delete(s.runs, sid)
	}
	s.runsMu.Unlock()
	for _, run := range leftover {
		if run.persister == nil {
			continue
		}
		if _, err := run.persister.ForceFinalize(); err != nil {
			s.logger.Error("force finalize AI turn failed",
				zap.String("runId", run.runID),
				zap.Error(err),
			)
		}
	}
	s.RecoverOrphanTurns()
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
	if ch == nil {
		run.mu.Unlock()
		return fmt.Errorf("unknown or expired approval id")
	}
	_, isSSH := run.sshApprovals[approvalID]
	delete(run.sshApprovals, approvalID)
	// Arm CancelSSHExec as soon as SSH is approved so Stop works before Wait/Exec race.
	if req.Approved && isSSH {
		run.execPending = true
	}
	run.mu.Unlock()
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
	s.rejectApprovalsAndCancel(run)
	return nil
}

// CancelSSHExec cancels only the in-flight run_ssh_command Exec for the session.
// It does not stop the agent run or deny pending approvals.
// Idle calls (no Exec and not between approve→Exec) are no-ops so a late Stop
// click cannot poison the next command via a sticky abort flag.
func (s *AIService) CancelSSHExec(sessionID string) error {
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
	run.mu.Lock()
	cancel := run.execCancel
	if cancel != nil {
		run.mu.Unlock()
		cancel()
		return nil
	}
	if run.execPending {
		run.execAbort = true
	}
	run.mu.Unlock()
	return nil
}

// CancelChat stops a specific send. If the run is already registered it is cancelled;
// otherwise runID is marked so Chat can refuse to start.
func (s *AIService) CancelChat(sessionID, runID string) error {
	sessionID = strings.TrimSpace(sessionID)
	runID = strings.TrimSpace(runID)
	if sessionID == "" || runID == "" {
		return fmt.Errorf("session id and run id are required")
	}
	s.runsMu.Lock()
	run := s.runs[sessionID]
	if run != nil && run.runID == runID {
		s.runsMu.Unlock()
		s.rejectApprovalsAndCancel(run)
		return nil
	}
	// Not registered yet (or a different run is active): remember for Chat's start checks.
	s.cancelled[runID] = struct{}{}
	s.runsMu.Unlock()
	// Bound pre-register marks so a late CancelChat after Chat finished cannot leak forever.
	time.AfterFunc(time.Minute, func() { s.clearCancelledRun(runID) })
	return nil
}

func (s *AIService) isRunCancelled(runID string) bool {
	if runID == "" {
		return false
	}
	s.runsMu.Lock()
	_, ok := s.cancelled[runID]
	s.runsMu.Unlock()
	return ok
}

func (s *AIService) errIfRunCancelled(sessionID, runID string) error {
	if !s.isRunCancelled(runID) {
		return nil
	}
	s.logger.Info("AI generation stopped before start",
		zap.String("sessionId", sessionID),
		zap.String("runId", runID),
	)
	return ErrGenerationStopped
}

func (s *AIService) clearCancelledRun(runID string) {
	if runID == "" {
		return
	}
	s.runsMu.Lock()
	delete(s.cancelled, runID)
	s.runsMu.Unlock()
}

func (s *AIService) rejectApprovalsAndCancel(run *agentRun) {
	if run == nil {
		return
	}
	// Prefer delivering denial so tools persist status=denied before ctx cancel races Wait.
	run.mu.Lock()
	for _, ch := range run.approvals {
		select {
		case ch <- approvalDecision{Approved: false, Reason: "stopped"}:
		default:
		}
	}
	execCancel := run.execCancel
	run.execPending = false
	run.execAbort = false
	clear(run.sshApprovals)
	run.mu.Unlock()
	if execCancel != nil {
		execCancel()
	}
	run.cancel()
}

// aiHost adapts SSH + approval for internal/ai ports.
type aiHost struct {
	svc *AIService
	run *agentRun
}

func (h *aiHost) Exec(ctx context.Context, linkID, command string) (string, error) {
	execCtx, cancel := context.WithCancel(ctx)
	if h.run != nil {
		h.run.mu.Lock()
		h.run.execCancel = cancel
		h.run.execPending = false
		abort := h.run.execAbort
		h.run.execAbort = false // single-shot: do not leak to the next command
		h.run.mu.Unlock()
		if abort {
			cancel()
		}
		defer func() {
			h.run.mu.Lock()
			// Sequential run_ssh_command: this Exec owns the slot for its lifetime.
			h.run.execCancel = nil
			h.run.mu.Unlock()
			cancel()
		}()
	} else {
		defer cancel()
	}
	// timeout=0: no wall-clock limit; cancelled via execCtx (CancelSSHExec or run cancel).
	return h.svc.sshService.exec(execCtx, linkID, command, 0, ssh.DefaultExecMaxOut)
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
		return false, "stopped", fmt.Errorf("no active generation")
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
			return false, "stopped", context.Canceled
		}
		return d.Approved, d.Reason, nil
	case <-ctx.Done():
		select {
		case d, ok := <-ch:
			if !ok {
				return false, "stopped", context.Canceled
			}
			return d.Approved, d.Reason, nil
		default:
			// Match rejectApprovalsAndCancel / synthetic cancel denials ("stopped").
			return false, "stopped", ctx.Err()
		}
	}
}

// MarkSSHApproval records that approvalID will SSH-Exec after approve (not MCP).
func (h *aiHost) MarkSSHApproval(approvalID string) {
	approvalID = strings.TrimSpace(approvalID)
	if h.run == nil || approvalID == "" {
		return
	}
	h.run.mu.Lock()
	if h.run.sshApprovals == nil {
		h.run.sshApprovals = make(map[string]struct{})
	}
	h.run.sshApprovals[approvalID] = struct{}{}
	h.run.mu.Unlock()
}

// ArmSSHExecCancel arms CancelSSHExec for the SSH approve→Exec race only.
func (h *aiHost) ArmSSHExecCancel() {
	if h.run == nil {
		return
	}
	h.run.mu.Lock()
	h.run.execPending = true
	h.run.mu.Unlock()
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
	return &AIMessage{
		ID: m.ID, SessionID: m.SessionID, Role: m.Role, Content: m.Content,
		Parts: m.Parts, Status: m.Status, RunID: m.RunID, Timestamp: m.Timestamp.Unix(),
	}
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
