package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	"go.uber.org/zap"
)

const mcpProbeTimeout = 20 * time.Second

// mcpRuntime manages Genkit MCPHost connections and UI status.
type mcpRuntime struct {
	logger *zap.Logger
	mu     sync.Mutex

	host  *mcp.MCPHost
	infos []MCPServerInfo
	tools []genkitAI.Tool
}

func newMCPRuntime(logger *zap.Logger) *mcpRuntime {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &mcpRuntime{logger: logger}
}

func (m *mcpRuntime) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectAllLocked(context.Background())
	for i := range m.infos {
		m.infos[i].Available = false
		m.infos[i].Error = ""
	}
}

func (m *mcpRuntime) disconnectAllLocked(ctx context.Context) {
	if m.host == nil {
		m.tools = nil
		return
	}
	for _, info := range m.infos {
		_ = m.host.Disconnect(ctx, info.Name)
	}
	m.host = nil
	m.tools = nil
}

// Reload reconnects enabled servers. When keepConnections is false, probes only then disconnects.
func (m *mcpRuntime) Reload(ctx context.Context, g *genkit.Genkit, keepConnections bool, userDataDir string) error {
	cfg, err := LoadMCPConfig(userDataDir)
	if err != nil {
		return err
	}
	entries := cfg.Entries()

	type result struct {
		entry MCPServerEntry
		info  MCPServerInfo
		tools []genkitAI.Tool
	}
	results := make([]result, 0, len(entries))

	for _, entry := range entries {
		info := MCPServerInfo{
			Name:      entry.Name,
			Enabled:   entry.IsEnabled(),
			Transport: entry.ResolvedTransport(),
			Summary:   entry.summary(),
		}
		if !entry.IsEnabled() {
			results = append(results, result{entry: entry, info: info})
			continue
		}
		if err := entry.validate(); err != nil {
			info.Error = err.Error()
			results = append(results, result{entry: entry, info: info})
			m.logger.Warn("mcp server config invalid", zap.String("server", entry.Name), zap.Error(err))
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, mcpProbeTimeout)
		toolsForServer, probeErr := probeMCPServer(probeCtx, g, entry)
		cancel()
		if probeErr != nil {
			info.Error = probeErr.Error()
			results = append(results, result{entry: entry, info: info})
			m.logger.Warn("mcp server unavailable", zap.String("server", entry.Name), zap.Error(probeErr))
			continue
		}
		if g != nil && len(toolsForServer) == 0 {
			info.Error = "no tools available"
			results = append(results, result{entry: entry, info: info})
			m.logger.Warn("mcp server has no tools", zap.String("server", entry.Name))
			continue
		}
		info.Available = true
		results = append(results, result{entry: entry, info: info, tools: toolsForServer})
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectAllLocked(ctx)

	infos := make([]MCPServerInfo, 0, len(results))
	var tools []genkitAI.Tool
	if keepConnections && g != nil {
		host, err := mcp.NewMCPHost(g, mcp.MCPHostOptions{Name: "vela"})
		if err != nil {
			return err
		}
		m.host = host
		for i, r := range results {
			infos = append(infos, r.info)
			if !r.info.Available {
				continue
			}
			opts, err := entryToMCPClientOptions(r.entry)
			if err != nil {
				infos[i].Available = false
				infos[i].Error = err.Error()
				continue
			}
			if err := host.Connect(ctx, g, r.entry.Name, opts); err != nil {
				infos[i].Available = false
				infos[i].Error = err.Error()
				m.logger.Warn("mcp connect failed", zap.String("server", r.entry.Name), zap.Error(err))
				continue
			}
		}
		live, err := host.GetActiveTools(ctx, g)
		if err != nil {
			m.logger.Warn("mcp get active tools failed", zap.Error(err))
		} else {
			tools = live
		}
	} else {
		for _, r := range results {
			infos = append(infos, r.info)
		}
	}
	m.infos = infos
	m.tools = tools
	return nil
}

func probeMCPServer(ctx context.Context, g *genkit.Genkit, entry MCPServerEntry) ([]genkitAI.Tool, error) {
	opts, err := entryToMCPClientOptions(entry)
	if err != nil {
		return nil, err
	}
	client, err := mcp.NewGenkitMCPClient(opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Disconnect() }()
	if g == nil {
		return nil, nil
	}
	tools, err := client.GetActiveTools(ctx, g)
	if err != nil {
		return nil, err
	}
	return tools, nil
}

func entryToMCPClientOptions(entry MCPServerEntry) (mcp.MCPClientOptions, error) {
	opts := mcp.MCPClientOptions{
		Name:     entry.Name,
		Disabled: !entry.IsEnabled(),
	}
	switch entry.ResolvedTransport() {
	case mcpTransportStdio:
		opts.Stdio = &mcp.StdioConfig{
			Command: entry.Command,
			Args:    append([]string(nil), entry.Args...),
			Env:     entry.EnvSlice(),
		}
	case mcpTransportHTTP:
		opts.StreamableHTTP = &mcp.StreamableHTTPConfig{
			BaseURL: entry.URL,
			Headers: copyStringMap(entry.Headers),
			Timeout: 30 * time.Second,
		}
	default:
		return opts, fmt.Errorf("unsupported transport %q", entry.ResolvedTransport())
	}
	return opts, nil
}

func (m *mcpRuntime) List() []MCPServerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MCPServerInfo, len(m.infos))
	copy(out, m.infos)
	return out
}

func (m *mcpRuntime) HasCachedInfos() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.infos != nil
}

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
