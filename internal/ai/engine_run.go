package ai

import (
	"context"
	"fmt"
	"strings"

	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/middleware"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai/tool"
)

// Run executes one agent turn with Genkit ToolApproval interrupt/resume inside the call.
func (e *AIEngine) Run(ctx context.Context, req *AgentRequest, rd RunDeps, onEvent func(StreamEvent) error) (*AgentResponse, error) {
	e.mu.RLock()
	modelName := e.modelName
	genConfig := e.genConfig
	g := e.genkit
	cfg := e.config
	tools := e.tools
	userDataDir := e.userDataDir
	e.mu.RUnlock()
	snap := e.Snapshot()

	if g == nil || !cfg.Enabled {
		return nil, fmt.Errorf("ai engine not initialized or not enabled")
	}
	if req == nil {
		return nil, fmt.Errorf("agent request is nil")
	}
	if onEvent == nil {
		onEvent = func(StreamEvent) error { return nil }
	}
	if tools.Plan == nil {
		return nil, fmt.Errorf("ai tools not registered")
	}

	deps := rd.Tools
	deps.Emit = func(ev StreamEvent) { _ = onEvent(ev) }
	deps.Echo = cfg.EchoSSHCommands
	if !req.EnableSSH {
		deps.LinkID = ""
	} else if deps.LinkID == "" {
		deps.LinkID = req.LinkID
	}
	ctx = tool.WithDeps(ctx, deps)

	var (
		messages []*genkitAI.Message
		err      error
	)
	if len(req.ResumeParts) > 0 && len(req.NewParts) == 0 {
		messages, err = buildGenkitHistoryOnly(req.Messages, req.SystemPrompt, userDataDir)
	} else {
		messages, err = buildGenkitMessages(req.Messages, req.NewParts, req.SystemPrompt, userDataDir)
	}
	if err != nil {
		return nil, err
	}
	promptLen := len(messages)
	// Cold resume: keep the interrupted model message inside the stored turn so Finalize
	// rewrites it (resolved interrupt + tool results) instead of appending beside it.
	if len(req.ResumeParts) > 0 {
		if i := TrailingInterruptModelIndex(messages); i >= 0 {
			promptLen = i
		}
	}

	activeTools, builtinNames, mcpNames := collectActiveTools(tools, deps, snap)
	skillNames := make([]string, 0, len(snap.Skills))
	for _, s := range snap.Skills {
		skillNames = append(skillNames, s.Name)
	}
	e.logger.Debug("ai run tools for genkit",
		zap.Strings("builtin", builtinNames),
		zap.Strings("mcp", mcpNames),
		zap.Strings("skills", skillNames),
		zap.Bool("use_skill_tool", len(skillNames) > 0),
		zap.Int("withToolsCount", len(activeTools)),
	)

	mw, cleanupSkills, err := e.buildRunMiddleware(snap, userDataDir, promptLen, rd.OnCheckpoint)
	if err != nil {
		return nil, err
	}
	if cleanupSkills != nil {
		defer cleanupSkills()
	}

	maxTurns := cfg.EffectiveMaxAgentTurns()
	textID := "text-" + uuid.New().String()
	reasoningID := "reasoning-" + uuid.New().String()

	var (
		resp     *genkitAI.ModelResponse
		streamed strings.Builder
		resume   []*genkitAI.Part
	)
	if len(req.ResumeParts) > 0 {
		resume = append(resume, req.ResumeParts...)
	}
	history := messages

	for {
		remaining := maxTurns - toolRoundsAfter(history, promptLen)
		if remaining <= 0 {
			return applyStreamedText(storedTurn(resp, promptLen), streamed.String()),
				fmt.Errorf("generate error: %w", genkitAI.ErrMaxTurnsExceeded)
		}

		opts := []genkitAI.GenerateOption{
			genkitAI.WithModelName(modelName),
			genkitAI.WithMessages(history...),
			genkitAI.WithMaxTurns(remaining),
			genkitAI.WithUse(mw...),
		}
		if len(activeTools) > 0 {
			opts = append(opts, genkitAI.WithTools(activeTools...))
		}
		if genConfig != nil {
			opts = append(opts, genkitAI.WithConfig(genConfig))
		}
		if len(resume) > 0 {
			opts = append(opts, genkitAI.WithResume(resume...))
			resume = nil
		}

		chunkText := ""
		var genErr error
		resp, chunkText, genErr = e.consumeStream(ctx, g, opts, onEvent, textID, reasoningID)
		if chunkText != "" {
			streamed.WriteString(chunkText)
		}

		if genErr != nil && len(activeTools) > 0 && len(history) == promptLen && isNoToolSupport(genErr) {
			e.logger.Warn("model does not support tools, retrying without tools", zap.Error(genErr))
			optsNoTools := []genkitAI.GenerateOption{
				genkitAI.WithModelName(modelName),
				genkitAI.WithMessages(history...),
				genkitAI.WithMaxTurns(remaining),
				genkitAI.WithUse(mw...),
			}
			if genConfig != nil {
				optsNoTools = append(optsNoTools, genkitAI.WithConfig(genConfig))
			}
			resp, chunkText, genErr = e.consumeStream(ctx, g, optsNoTools, onEvent, textID, reasoningID)
			if chunkText != "" {
				streamed.WriteString(chunkText)
			}
			if genErr != nil {
				return applyStreamedText(storedTurn(resp, promptLen), streamed.String()), fmt.Errorf("generate error: %w", genErr)
			}
			return applyStreamedText(storedTurn(resp, promptLen), streamed.String()), nil
		}
		if genErr != nil {
			return applyStreamedText(storedTurn(resp, promptLen), streamed.String()), fmt.Errorf("generate error: %w", genErr)
		}
		if resp == nil {
			return applyStreamedText(nil, streamed.String()), nil
		}

		if resp.FinishReason != genkitAI.FinishReasonInterrupted && len(resp.Interrupts()) == 0 {
			return applyStreamedText(storedTurn(resp, promptLen), streamed.String()), nil
		}

		interrupts := resp.Interrupts()
		pending := pendingFromInterruptParts(interrupts)
		if len(pending) == 0 {
			// Interrupted but no ToolApproval holds we can claim — stop.
			return applyStreamedText(storedTurn(resp, promptLen), streamed.String()), nil
		}

		// Emit tool card + approval; wrapToolCall later emits output (may repeat input — MUI merges by toolCallId).
		for _, p := range pending {
			_ = onEvent(StreamEvent{Type: EventToolInputStart, ToolCallID: p.ToolCallID, ToolName: p.ToolName})
			_ = onEvent(StreamEvent{Type: EventToolInputAvailable, ToolCallID: p.ToolCallID, ToolName: p.ToolName, Input: p.Input})
			_ = onEvent(StreamEvent{
				Type: EventToolApprovalRequest, ToolCallID: p.ToolCallID, ToolName: p.ToolName,
				ApprovalID: p.ApprovalID, Input: p.Input,
			})
		}

		if rd.WaitApprovals == nil {
			return applyStreamedText(storedTurn(resp, promptLen), streamed.String()),
				fmt.Errorf("generate error: tool approval wait unavailable")
		}
		decisions, waitErr := rd.WaitApprovals(ctx, pending)
		if waitErr != nil {
			return applyStreamedText(storedTurn(resp, promptLen), streamed.String()), waitErr
		}
		byID := make(map[string]ApprovalDecision, len(decisions))
		for _, d := range decisions {
			byID[d.ApprovalID] = d
		}

		resume = resume[:0]
		for _, part := range interrupts {
			if part == nil || part.ToolRequest == nil {
				continue
			}
			ref := toolRequestRef(part.ToolRequest)
			d, ok := byID[ref]
			if !ok {
				d = ApprovalDecision{ApprovalID: ref, Approved: false, Reason: "stopped"}
			}
			if !d.Approved {
				reason := denyReason(d.Reason)
				_ = onEvent(StreamEvent{
					Type: EventToolOutputDenied, ToolCallID: ref, ToolName: part.ToolRequest.Name,
					ApprovalID: ref, Reason: reason,
				})
				if rp := restartApprovalPart(part, false, reason); rp != nil {
					resume = append(resume, rp)
				}
				continue
			}
			if part.ToolRequest.Name == tool.RunSSHCommand && deps.ArmSSHExecCancel != nil {
				deps.ArmSSHExecCancel()
			}
			if rp := restartApprovalPart(part, true, ""); rp != nil {
				resume = append(resume, rp)
			}
		}

		history = resp.History()
	}
}

func collectActiveTools(tools tool.Set, deps tool.Deps, snap CatalogSnapshot) (active []genkitAI.ToolRef, builtin, mcp []string) {
	builtin = []string{tool.UpsertPlan}
	active = []genkitAI.ToolRef{tools.Plan}
	if deps.EnableSSH() {
		active = append(active, tools.SSH)
		builtin = append(builtin, tool.RunSSHCommand)
	}
	if deps.EnableFS() {
		active = append(active, tools.Read, tools.Write, tools.List)
		builtin = append(builtin, tool.ReadLocalFile, tool.WriteLocalFile, tool.ListLocalDirectory)
	}
	if deps.EnableWebSearch() {
		active = append(active, tools.Search)
		builtin = append(builtin, tool.WebSearch)
	}
	for _, t := range snap.MCPTools {
		active = append(active, t)
		if t != nil {
			mcp = append(mcp, t.Name())
		}
	}
	return active, builtin, mcp
}

func (e *AIEngine) buildRunMiddleware(
	snap CatalogSnapshot,
	userDataDir string,
	promptLen int,
	onCheckpoint func([]ChatMessage),
) (mw []genkitAI.Middleware, cleanup func(), err error) {
	allowed := []string{tool.UpsertPlan, tool.WebSearch, middleware.SkillToolName}
	mw = []genkitAI.Middleware{
		toolStreamMiddleware{},
		&middleware.ToolApproval{AllowedTools: allowed},
	}

	if len(snap.Skills) > 0 {
		overlay, clean, oerr := ensureSkillsOverlay(userDataDir, snap.Skills)
		if oerr != nil {
			return nil, nil, oerr
		}
		cleanup = clean
		mw = append(mw, &middleware.Skills{
			SkillPaths:          []string{overlay},
			AllowResourceAccess: false,
		})
	}

	if onCheckpoint != nil {
		mw = append(mw, checkpointMiddleware{
			promptLen:    promptLen,
			onCheckpoint: onCheckpoint,
		})
	}
	return mw, cleanup, nil
}

func toolRoundsAfter(msgs []*genkitAI.Message, promptLen int) int {
	if promptLen < 0 {
		promptLen = 0
	}
	n := 0
	for i := promptLen; i < len(msgs); i++ {
		m := msgs[i]
		if m == nil || m.Role != genkitAI.RoleModel {
			continue
		}
		for _, p := range m.Content {
			if p != nil && p.IsToolRequest() {
				n++
				break
			}
		}
	}
	return n
}
