package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"uuid"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/ollama"
	"go.uber.org/zap"
)

// AIEngine AI引擎
type AIEngine struct {
	logger      *zap.Logger
	genkit      *genkit.Genkit
	config      *Config
	modelName   string
	genConfig   any
	planTool    genkitAI.ToolRef
	sshTool     genkitAI.ToolRef
	userDataDir string
	mcp         *mcpRuntime
	mu          sync.RWMutex
}

// NewAIEngine 创建AI引擎
func NewAIEngine(logger *zap.Logger) *AIEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AIEngine{
		logger: logger,
		mcp:    newMCPRuntime(logger),
		config: &Config{
			Enabled:        false,
			Provider:       ProviderOllama,
			Model:          "llama3.2",
			Endpoint:       "http://localhost:11434",
			Temperature:    0.7,
			MaxTokens:      2048,
			MaxAgentTurns:  DefaultMaxAgentTurns,
			ExecTimeoutSec: DefaultExecTimeoutSec,
		},
	}
}

// SetUserDataDir sets the directory for mcp.json and skills/.
func (e *AIEngine) SetUserDataDir(dir string) {
	e.mu.Lock()
	e.userDataDir = strings.TrimSpace(dir)
	e.mu.Unlock()
}

// Init initializes Genkit plugins, models, and tools.
func (e *AIEngine) Init(ctx context.Context, cfg *Config) error {
	var ollamaPlugin *ollama.Ollama
	var g *genkit.Genkit

	switch cfg.Provider {
	case ProviderOllama:
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:11434"
		}
		ollamaPlugin = &ollama.Ollama{
			ServerAddress: endpoint,
			Timeout:       300,
		}
		g = genkit.Init(ctx, genkit.WithPlugins(ollamaPlugin))

	case ProviderGoogle:
		g = genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: cfg.APIKey}))

	case ProviderOpenAI:
		g = genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{
			APIKey: cfg.APIKey,
		}))

	case ProviderOpenAICompatible:
		if cfg.Endpoint == "" {
			return fmt.Errorf("endpoint is required for openai_compatible")
		}
		g = genkit.Init(ctx, genkit.WithPlugins(&compat_oai.OpenAICompatible{
			Provider: "custom",
			APIKey:   cfg.APIKey,
			BaseURL:  cfg.Endpoint,
		}))

	default:
		return fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	if cfg.Provider == ProviderOllama {
		if ollamaPlugin == nil {
			return fmt.Errorf("ollama plugin not initialized")
		}
		ollamaPlugin.DefineModel(g, ollama.ModelDefinition{
			Name: cfg.Model,
			Type: "chat",
		}, nil)
	}

	modelName, err := resolveModelName(cfg)
	if err != nil {
		return err
	}

	tools := registerTools(g)

	e.mu.Lock()
	e.genkit = g
	e.config = cfg
	e.modelName = modelName
	e.genConfig = cfg.buildGenConfig()
	e.planTool = tools.plan
	e.sshTool = tools.ssh
	userDataDir := e.userDataDir
	mcp := e.mcp
	e.mu.Unlock()

	if userDataDir != "" && mcp != nil {
		if err := mcp.Reload(ctx, true, userDataDir); err != nil {
			e.logger.Warn("reload mcp after init failed", zap.Error(err))
		}
	}

	return nil
}

// Run executes one agent turn (text + optional tools). onEvent receives stream events.
func (e *AIEngine) Run(ctx context.Context, req *AgentRequest, deps ToolDeps, onEvent func(StreamEvent) error) (*AgentResponse, error) {
	e.mu.RLock()
	modelName := e.modelName
	genConfig := e.genConfig
	g := e.genkit
	cfg := e.config
	planTool := e.planTool
	sshTool := e.sshTool
	userDataDir := e.userDataDir
	mcp := e.mcp
	e.mu.RUnlock()

	if g == nil || !cfg.Enabled {
		return nil, fmt.Errorf("ai engine not initialized or not enabled")
	}
	if req == nil {
		return nil, fmt.Errorf("agent request is nil")
	}
	if onEvent == nil {
		onEvent = func(StreamEvent) error { return nil }
	}
	if planTool == nil {
		return nil, fmt.Errorf("ai tools not registered")
	}

	deps.Emit = func(ev StreamEvent) { _ = onEvent(ev) }
	deps.Echo = cfg.EchoSSHCommands
	if !req.EnableSSH {
		deps.LinkID = ""
	} else if deps.LinkID == "" {
		deps.LinkID = req.LinkID
	}
	ctx = withToolDeps(ctx, deps)

	messages := buildGenkitMessages(req.Messages, req.NewMessage, req.SystemPrompt)
	promptLen := len(messages)
	finish := func(resp *genkitAI.ModelResponse, streamed string, err error) (*AgentResponse, error) {
		return applyStreamedText(storedTurn(resp, promptLen), streamed), err
	}

	activeTools := []genkitAI.ToolRef{planTool}
	if deps.EnableSSHTool() && sshTool != nil {
		activeTools = append(activeTools, sshTool)
	}
	if mcp != nil {
		for _, t := range mcp.ActiveTools() {
			activeTools = append(activeTools, t)
		}
	}

	middleware := []genkitAI.Middleware{toolStreamMiddleware{}}
	if skillDirs, err := EnabledSkillPaths(userDataDir); err == nil && len(skillDirs) > 0 {
		middleware = append(middleware, skillsMiddleware{SkillDirs: skillDirs})
	} else if err != nil {
		e.logger.Warn("list enabled skills failed", zap.Error(err))
	}
	// Checkpoint last so WrapGenerate snapshots after skills/system injection and generate.
	if deps.OnCheckpoint != nil {
		middleware = append(middleware, checkpointMiddleware{
			promptLen:    promptLen,
			onCheckpoint: deps.OnCheckpoint,
		})
	}
	opts := []genkitAI.GenerateOption{
		genkitAI.WithModelName(modelName),
		genkitAI.WithMessages(messages...),
		genkitAI.WithMaxTurns(cfg.EffectiveMaxAgentTurns()),
		genkitAI.WithUse(middleware...),
	}
	if len(activeTools) > 0 {
		opts = append(opts, genkitAI.WithTools(activeTools...))
	}
	if genConfig != nil {
		opts = append(opts, genkitAI.WithConfig(genConfig))
	}

	textID := "text-" + uuid.New().String()
	reasoningID := "reasoning-" + uuid.New().String()

	resp, streamed, err := e.consumeStream(ctx, g, opts, onEvent, textID, reasoningID)
	if err != nil && len(activeTools) > 0 && isNoToolSupport(err) {
		e.logger.Warn("model does not support tools, retrying without tools", zap.Error(err))
		optsNoTools := []genkitAI.GenerateOption{
			genkitAI.WithModelName(modelName),
			genkitAI.WithMessages(messages...),
			genkitAI.WithMaxTurns(cfg.EffectiveMaxAgentTurns()),
			genkitAI.WithUse(middleware...),
		}
		if genConfig != nil {
			optsNoTools = append(optsNoTools, genkitAI.WithConfig(genConfig))
		}
		resp, streamed, err = e.consumeStream(ctx, g, optsNoTools, onEvent, textID, reasoningID)
	}
	if err != nil {
		return finish(resp, streamed, fmt.Errorf("generate error: %w", err))
	}
	return finish(resp, streamed, nil)
}

func (e *AIEngine) consumeStream(
	ctx context.Context,
	g *genkit.Genkit,
	opts []genkitAI.GenerateOption,
	onEvent func(StreamEvent) error,
	textID, reasoningID string,
) (*genkitAI.ModelResponse, string, error) {
	var streamed strings.Builder
	stream := genkit.GenerateStream(ctx, g, opts...)
	for result, err := range stream {
		if err != nil {
			var resp *genkitAI.ModelResponse
			if result != nil {
				resp = result.Response
			}
			return resp, streamed.String(), err
		}
		if result == nil {
			continue
		}
		if result.Done {
			return result.Response, streamed.String(), nil
		}
		if result.Chunk == nil {
			continue
		}

		if reasoning := result.Chunk.Reasoning(); reasoning != "" {
			_ = onEvent(StreamEvent{Type: EventReasoningDelta, ID: reasoningID, Delta: reasoning})
			continue
		}
		if text := result.Chunk.Text(); text != "" {
			streamed.WriteString(text)
			_ = onEvent(StreamEvent{Type: EventTextDelta, ID: textID, Delta: text})
		}
	}
	return nil, streamed.String(), nil
}

func isNoToolSupport(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not support tool use") || strings.Contains(msg, "does not support tools")
}

func isToolArgsParseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "could not parse tool args") ||
		strings.Contains(msg, "unmarshal failed to parse json string")
}

// PublicGenerateError is a short message for the chat UI. Full detail belongs in logs.
func PublicGenerateError(err error) string {
	if err == nil {
		return "generate failed"
	}
	if isToolArgsParseError(err) {
		return "工具参数 JSON 无效（命令过长或引号未转义），请改用更短的单条命令"
	}
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "AI chat failed: ")
	msg = strings.TrimPrefix(msg, "generate error: ")
	if strings.Contains(msg, `{"`) || strings.Contains(strings.ToLower(msg), "json string") {
		return "生成失败：模型输出无法解析"
	}
	if len(msg) > 240 {
		return msg[:240] + "…"
	}
	return msg
}

// IsEnabled 是否启用
func (e *AIEngine) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config.Enabled && e.genkit != nil
}

// SetConfig 更新配置
func (e *AIEngine) SetConfig(cfg *Config) error {
	if cfg == nil || !cfg.Enabled {
		if cfg == nil {
			cfg = &Config{Enabled: false}
		}
		e.mu.Lock()
		e.config = cfg
		e.genkit = nil
		e.modelName = ""
		e.genConfig = nil
		e.planTool = nil
		e.sshTool = nil
		mcp := e.mcp
		e.mu.Unlock()
		if mcp != nil {
			mcp.Close()
		}
		return nil
	}
	return e.Init(context.Background(), cfg)
}

// GetConfig 获取当前配置
func (e *AIEngine) GetConfig() Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return *e.config
}

// Close disconnects MCP clients.
func (e *AIEngine) Close() {
	e.mu.RLock()
	mcp := e.mcp
	e.mu.RUnlock()
	if mcp != nil {
		mcp.Close()
	}
}

// ReloadMCPAndSkills reloads mcp.json connections; skills are read from disk on each Run.
// When Genkit is not initialized, servers are probed for status only (no retained stdio processes).
func (e *AIEngine) ReloadMCPAndSkills(ctx context.Context) error {
	e.mu.RLock()
	g := e.genkit
	userDataDir := e.userDataDir
	mcp := e.mcp
	e.mu.RUnlock()
	if userDataDir == "" {
		return fmt.Errorf("user data dir is not set")
	}
	if mcp == nil {
		return nil
	}
	return mcp.Reload(ctx, g != nil, userDataDir)
}

// ListMCPServers returns cached MCP status without probing.
// When the cache is empty, hydrates names/enabled/summary from mcp.json only.
func (e *AIEngine) ListMCPServers() ([]MCPServerInfo, error) {
	e.mu.RLock()
	userDataDir := e.userDataDir
	mcp := e.mcp
	e.mu.RUnlock()
	if mcp == nil {
		return nil, nil
	}
	if userDataDir == "" {
		return mcp.List(), nil
	}
	if !mcp.HasCachedInfos() {
		if err := mcp.HydrateFromConfig(userDataDir); err != nil {
			return nil, err
		}
	}
	return mcp.List(), nil
}

func resolveModelName(cfg *Config) (string, error) {
	name := strings.TrimSpace(cfg.Model)
	if name == "" {
		return "", fmt.Errorf("model is required")
	}
	if strings.Contains(name, "/") {
		return name, nil
	}
	switch cfg.Provider {
	case ProviderOllama:
		return "ollama/" + name, nil
	case ProviderOpenAI:
		return "openai/" + name, nil
	case ProviderOpenAICompatible:
		return "custom/" + name, nil
	case ProviderGoogle:
		return "googleai/" + name, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}
