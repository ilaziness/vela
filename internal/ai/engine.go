package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/ollama"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai/tool"
)

// AIEngine AI引擎
type AIEngine struct {
	logger      *zap.Logger
	genkit      *genkit.Genkit
	config      *Config
	modelName   string
	genConfig   any
	tools       tool.Set
	userDataDir string
	mcp         *mcpRuntime
	catalog     atomic.Value // *CatalogSnapshot
	catalogMu   sync.Mutex   // serializes catalog writers (Snapshot stays lock-free)
	mcpMu       sync.Mutex   // serializes ReloadMCP and shutdownMCP
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
			Enabled:       false,
			Provider:      ProviderOllama,
			Model:         "llama3.2",
			Endpoint:      "http://localhost:11434",
			Temperature:   0,
			MaxTokens:     2048,
			MaxAgentTurns: DefaultMaxAgentTurns,
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

	tools := tool.Register(g)

	e.mu.Lock()
	e.genkit = g
	e.config = cfg
	e.modelName = modelName
	e.genConfig = cfg.buildGenConfig()
	e.tools = tools
	userDataDir := e.userDataDir
	mcp := e.mcp
	e.mu.Unlock()

	e.PublishBuiltinFromConfig(cfg)
	if err := e.PublishSkills(); err != nil {
		e.logger.Warn("publish skills after init failed", zap.Error(err))
	}
	if userDataDir != "" && mcp != nil {
		if err := e.ReloadMCP(ctx); err != nil {
			e.logger.Warn("reload mcp after init failed", zap.Error(err))
		}
	}

	return nil
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

		// A single chunk may carry both reasoning and text (compat_oai).
		if reasoning := result.Chunk.Reasoning(); reasoning != "" {
			_ = onEvent(StreamEvent{Type: EventReasoningDelta, ID: reasoningID, Delta: reasoning})
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
		e.tools = tool.Set{}
		e.mu.Unlock()
		e.shutdownMCP()
		// Refresh builtin flags from the saved config (caller should pass full cfg with Enabled=false).
		e.PublishBuiltinFromConfig(cfg)
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
	e.shutdownMCP()
}

// shutdownMCP serializes with ReloadMCP so a finishing reload cannot revive clients after disable.
func (e *AIEngine) shutdownMCP() {
	e.mcpMu.Lock()
	defer e.mcpMu.Unlock()
	e.mu.RLock()
	mcp := e.mcp
	e.mu.RUnlock()
	if mcp != nil {
		mcp.Close()
	}
	e.clearMCPTools()
}

// ReloadMCP reloads mcp.json connections and publishes MCP tools into the catalog.
// When Genkit is not initialized, servers are probed for status only (no retained stdio processes).
func (e *AIEngine) ReloadMCP(ctx context.Context) error {
	e.mcpMu.Lock()
	defer e.mcpMu.Unlock()

	e.mu.RLock()
	g := e.genkit
	ready := g != nil
	userDataDir := e.userDataDir
	mcp := e.mcp
	e.mu.RUnlock()
	if userDataDir == "" {
		return fmt.Errorf("user data dir is not set")
	}
	if mcp == nil {
		return nil
	}
	if err := mcp.Reload(ctx, g, ready, userDataDir); err != nil {
		return err
	}
	// genkit may be cleared under e.mu while probes ran (disable does that before waiting on mcpMu).
	e.mu.RLock()
	stillReady := e.genkit != nil
	e.mu.RUnlock()
	if ready && !stillReady {
		mcp.Close()
		e.clearMCPTools()
		return nil
	}
	e.publishMCP()
	return nil
}

// EnsureMCPToolsConnected reloads MCP with keepConnections when Genkit is ready
// but catalog MCP tools are empty while enabled servers exist in mcp.json.
func (e *AIEngine) EnsureMCPToolsConnected(ctx context.Context) error {
	e.mu.RLock()
	ready := e.genkit != nil
	userDataDir := e.userDataDir
	mcp := e.mcp
	e.mu.RUnlock()
	if !ready || mcp == nil || userDataDir == "" {
		return nil
	}
	if len(e.Snapshot().MCPTools) > 0 {
		return nil
	}
	cfg, err := LoadMCPConfig(userDataDir)
	if err != nil {
		return err
	}
	hasEnabled := false
	for _, entry := range cfg.Entries() {
		if entry.IsEnabled() {
			hasEnabled = true
			break
		}
	}
	if !hasEnabled {
		return nil
	}
	return e.ReloadMCP(ctx)
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
