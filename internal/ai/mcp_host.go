package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	genkitAI "github.com/firebase/genkit/go/ai"
	genkitTool "github.com/firebase/genkit/go/ai/tool"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

const mcpProbeTimeout = 20 * time.Second

// mcpSession is one live MCP server connection (stdio or HTTP).
type mcpSession struct {
	client *mcpclient.Client
}

func (s *mcpSession) Close() {
	if s == nil || s.client == nil {
		return
	}
	_ = s.client.Close()
	s.client = nil
}

// mcpRuntime manages MCP client connections and availability probes.
type mcpRuntime struct {
	logger *zap.Logger
	mu     sync.Mutex

	clients map[string]*mcpSession
	infos   []MCPServerInfo
	tools   []genkitAI.Tool
}

func newMCPRuntime(logger *zap.Logger) *mcpRuntime {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &mcpRuntime{
		logger:  logger,
		clients: make(map[string]*mcpSession),
	}
}

func (m *mcpRuntime) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectAllLocked()
	// Connections are gone; clear availability so UI does not show stale green dots.
	for i := range m.infos {
		m.infos[i].Available = false
		m.infos[i].Error = ""
	}
}

func (m *mcpRuntime) disconnectAllLocked() {
	for name, c := range m.clients {
		if c != nil {
			c.Close()
		}
		delete(m.clients, name)
	}
	m.clients = make(map[string]*mcpSession)
	m.tools = nil
}

type mcpProbeResult struct {
	entry   MCPServerEntry
	info    MCPServerInfo
	session *mcpSession
	tools   []genkitAI.Tool
}

// Reload reconnects enabled servers, probes health, and optionally keeps connections.
// When keepConnections is false (e.g. AI/Genkit not ready), servers are probed for
// status only and then disconnected so stdio processes are not left running.
// Probes run without holding the runtime mutex so List/ActiveTools stay responsive.
func (m *mcpRuntime) Reload(ctx context.Context, keepConnections bool, userDataDir string) error {
	cfg, err := LoadMCPConfig(userDataDir)
	if err != nil {
		return err
	}
	entries := cfg.Entries()
	results := make([]mcpProbeResult, 0, len(entries))

	for _, entry := range entries {
		info := MCPServerInfo{
			Name:      entry.Name,
			Enabled:   entry.IsEnabled(),
			Transport: entry.ResolvedTransport(),
			Summary:   entry.summary(),
		}
		if !entry.IsEnabled() {
			results = append(results, mcpProbeResult{entry: entry, info: info})
			continue
		}
		if err := entry.validate(); err != nil {
			info.Error = err.Error()
			results = append(results, mcpProbeResult{entry: entry, info: info})
			m.logger.Warn("mcp server config invalid", zap.String("server", entry.Name), zap.Error(err))
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, mcpProbeTimeout)
		session, toolsForServer, probeErr := probeMCPServer(probeCtx, entry)
		cancel()
		if probeErr != nil {
			info.Error = probeErr.Error()
			results = append(results, mcpProbeResult{entry: entry, info: info})
			m.logger.Warn("mcp server unavailable", zap.String("server", entry.Name), zap.Error(probeErr))
			if session != nil {
				session.Close()
			}
			continue
		}
		if len(toolsForServer) == 0 {
			info.Error = "no tools available"
			results = append(results, mcpProbeResult{entry: entry, info: info})
			m.logger.Warn("mcp server has no tools", zap.String("server", entry.Name))
			session.Close()
			continue
		}
		info.Available = true
		results = append(results, mcpProbeResult{
			entry:   entry,
			info:    info,
			session: session,
			tools:   toolsForServer,
		})
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectAllLocked()

	infos := make([]MCPServerInfo, 0, len(results))
	tools := make([]genkitAI.Tool, 0)
	for _, r := range results {
		infos = append(infos, r.info)
		if !r.info.Available || r.session == nil {
			continue
		}
		if !keepConnections {
			r.session.Close()
			continue
		}
		m.clients[r.entry.Name] = r.session
		tools = append(tools, r.tools...)
	}
	m.infos = infos
	m.tools = tools
	return nil
}

// List returns the last known server statuses (from Init/Reload or config hydrate).
func (m *mcpRuntime) List() []MCPServerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MCPServerInfo, len(m.infos))
	copy(out, m.infos)
	return out
}

// HasCachedInfos reports whether a probe/hydrate has populated the status list.
func (m *mcpRuntime) HasCachedInfos() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.infos != nil
}

// HydrateFromConfig fills infos from mcp.json without connecting or probing.
// Availability stays unset (false, empty error) until Reload. Preserves prior
// Available/Error when the same server name already exists in cache.
func (m *mcpRuntime) HydrateFromConfig(userDataDir string) error {
	cfg, err := LoadMCPConfig(userDataDir)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := make(map[string]MCPServerInfo, len(m.infos))
	for _, info := range m.infos {
		prev[info.Name] = info
	}
	entries := cfg.Entries()
	infos := make([]MCPServerInfo, 0, len(entries))
	for _, entry := range entries {
		info := MCPServerInfo{
			Name:      entry.Name,
			Enabled:   entry.IsEnabled(),
			Transport: entry.ResolvedTransport(),
			Summary:   entry.summary(),
		}
		if old, ok := prev[entry.Name]; ok {
			info.Available = old.Available
			info.Error = old.Error
		}
		infos = append(infos, info)
	}
	m.infos = infos
	return nil
}

func (m *mcpRuntime) ActiveTools() []genkitAI.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]genkitAI.Tool, len(m.tools))
	copy(out, m.tools)
	return out
}

// probeMCPServer connects with ctx (so stdio CommandContext + Initialize can time out),
// lists tools, and builds Genkit tool wrappers. Caller owns the session.
func probeMCPServer(ctx context.Context, entry MCPServerEntry) (*mcpSession, []genkitAI.Tool, error) {
	client, err := connectMCPClient(ctx, entry)
	if err != nil {
		return nil, nil, err
	}
	session := &mcpSession{client: client}

	list, err := client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		session.Close()
		return nil, nil, fmt.Errorf("list tools: %w", err)
	}
	tools, err := buildMCPTools(entry.Name, client, list.Tools)
	if err != nil {
		session.Close()
		return nil, nil, err
	}
	return session, tools, nil
}

func connectMCPClient(ctx context.Context, entry MCPServerEntry) (*mcpclient.Client, error) {
	tr, err := createMCPTransport(entry)
	if err != nil {
		return nil, err
	}
	// Manual round-trips so input_required results reach mcpToolCallOutput
	// (tool.Fail) instead of auto-failing on missing elicitation handlers.
	client := mcpclient.NewClient(tr, mcpclient.WithManualInputRoundTrips())
	if err := client.Start(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start transport: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "vela-mcp-client",
		Version: "1.0.0",
	}
	if _, err := client.Initialize(ctx, initReq); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}
	return client, nil
}

func createMCPTransport(entry MCPServerEntry) (transport.Interface, error) {
	switch entry.ResolvedTransport() {
	case mcpTransportStdio:
		return transport.NewStdio(entry.Command, entry.EnvSlice(), append([]string(nil), entry.Args...)...), nil
	case mcpTransportHTTP:
		opts := []transport.StreamableHTTPCOption{
			transport.WithHTTPTimeout(30 * time.Second),
		}
		if headers := copyStringMap(entry.Headers); len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		tr, err := transport.NewStreamableHTTP(entry.URL, opts...)
		if err != nil {
			return nil, fmt.Errorf("create http transport: %w", err)
		}
		return tr, nil
	default:
		return nil, fmt.Errorf("unsupported transport %q", entry.ResolvedTransport())
	}
}

func buildMCPTools(serverName string, client *mcpclient.Client, mcpTools []mcp.Tool) ([]genkitAI.Tool, error) {
	out := make([]genkitAI.Tool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		tool, err := buildMCPTool(serverName, client, mcpTool)
		if err != nil {
			return nil, err
		}
		out = append(out, tool)
	}
	return out, nil
}

// maxMCPToolNameLen matches Google GenAI tool-name limits (≤64 chars).
const maxMCPToolNameLen = 64

// mcpNamespacedToolName follows Claude Code / Codex: mcp__{server}__{tool}.
// Parts are sanitized to [A-Za-z0-9_-] so OpenAI-style tool name validation passes.
// Names longer than maxMCPToolNameLen are truncated with a stable hash suffix
// so Gemini registration does not reject them.
func mcpNamespacedToolName(serverName, toolName string) string {
	name := "mcp__" + sanitizeMCPNamePart(serverName) + "__" + sanitizeMCPNamePart(toolName)
	if len(name) <= maxMCPToolNameLen {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(sum[:4]) // 8 hex chars
	keep := maxMCPToolNameLen - 1 - len(suffix)
	if keep < 1 {
		keep = 1
	}
	return name[:keep] + "_" + suffix
}

func sanitizeMCPNamePart(s string) string {
	if s == "" {
		return "_"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func buildMCPTool(serverName string, client *mcpclient.Client, mcpTool mcp.Tool) (genkitAI.Tool, error) {
	namespaced := mcpNamespacedToolName(serverName, mcpTool.Name)
	inputSchema := mcpInputSchemaMap(mcpTool)
	rawToolName := mcpTool.Name
	fn := func(toolCtx *genkitAI.ToolContext, args any) (any, error) {
		ctx := context.Background()
		if toolCtx != nil && toolCtx.Context != nil {
			ctx = toolCtx.Context
		}
		callArgs, err := prepareMCPToolArgs(mcpTool, args)
		if err != nil {
			// Let the model correct missing/invalid args.
			return nil, genkitTool.Fail(ctx, err)
		}
		var callReq mcp.CallToolRequest
		callReq.Params.Name = rawToolName
		callReq.Params.Arguments = callArgs
		result, err := client.CallTool(ctx, callReq)
		if err != nil {
			// Transport/protocol failures are hard errors (model cannot fix them).
			return nil, fmt.Errorf("mcp tool %s: %w", namespaced, err)
		}
		out, err := mcpToolCallOutput(ctx, result)
		if err != nil {
			return nil, err
		}
		return out, nil
	}

	return genkitAI.NewTool(namespaced, mcpTool.Description, fn, genkitAI.WithInputSchema(inputSchema)), nil
}

func mcpInputSchemaMap(mcpTool mcp.Tool) map[string]any {
	var out map[string]any
	if len(mcpTool.RawInputSchema) > 0 {
		_ = json.Unmarshal(mcpTool.RawInputSchema, &out)
	}
	if out == nil {
		schemaBytes, err := json.Marshal(mcpTool.InputSchema)
		if err != nil {
			out = map[string]any{}
		} else {
			_ = json.Unmarshal(schemaBytes, &out)
			if out == nil {
				out = map[string]any{}
			}
		}
	}
	// mcp-go marshals a zero ToolInputSchema as type:"" — normalize for providers.
	if t, _ := out["type"].(string); t == "" {
		out["type"] = "object"
	}
	if out["properties"] == nil {
		out["properties"] = map[string]any{}
	}
	return out
}

// mcpToolCallOutput turns an MCP tools/call result into a Genkit tool payload.
// Modern protocol results include protocol bookkeeping (_meta, resultType); the
// model only needs structuredContent or text content. MCP isError / NeedsInput
// use tool.Fail so Genkit answers the model instead of aborting the turn.
func mcpToolCallOutput(ctx context.Context, result *mcp.CallToolResult) (any, error) {
	if result == nil {
		return nil, fmt.Errorf("empty result")
	}
	if result.NeedsInput() {
		return nil, genkitTool.Fail(ctx, errors.New("additional input required (not supported)"))
	}
	payload := mcpToolResultPayload(result)
	if result.IsError {
		return nil, genkitTool.Fail(ctx, errors.New(mcpToolErrorMessage(payload)))
	}
	return payload, nil
}

func mcpToolErrorMessage(payload any) string {
	switch v := payload.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return v
		}
	case map[string]any:
		if len(v) == 0 {
			return "the tool reported an error without a message"
		}
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	}
	msg := fmt.Sprint(payload)
	if msg == "" || msg == "map[]" {
		return "the tool reported an error without a message"
	}
	return msg
}

func mcpToolResultPayload(result *mcp.CallToolResult) any {
	if result.StructuredContent != nil {
		return result.StructuredContent
	}
	texts := make([]string, 0, len(result.Content))
	for _, c := range result.Content {
		switch v := c.(type) {
		case mcp.TextContent:
			texts = append(texts, v.Text)
		default:
			if b, err := json.Marshal(v); err == nil {
				texts = append(texts, string(b))
			}
		}
	}
	switch len(texts) {
	case 0:
		return map[string]any{}
	case 1:
		return texts[0]
	default:
		return strings.Join(texts, "\n")
	}
}

func prepareMCPToolArgs(mcpTool mcp.Tool, args any) (map[string]any, error) {
	callArgs := map[string]any{}
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("marshal tool args: %w", err)
		}
		if err := json.Unmarshal(b, &callArgs); err != nil {
			return nil, fmt.Errorf("tool args must be an object: %w", err)
		}
		if callArgs == nil {
			callArgs = map[string]any{}
		}
	}
	for _, required := range mcpTool.InputSchema.Required {
		if _, ok := callArgs[required]; !ok {
			return nil, fmt.Errorf("required field %q missing for tool %q", required, mcpTool.Name)
		}
	}
	return callArgs, nil
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
