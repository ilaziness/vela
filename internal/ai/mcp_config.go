package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	mcpConfigFileName = "mcp.json"
	mcpTransportStdio = "stdio"
	mcpTransportHTTP  = "http"
)

// MCPFileConfig is the on-disk mcp.json document (Cursor / Claude Code shape).
type MCPFileConfig struct {
	MCPServers map[string]MCPServerEntry `json:"mcpServers"`
}

// MCPServerEntry is one MCP server under mcpServers.<name>.
// Name is the map key and is filled on load (not written back as a field).
type MCPServerEntry struct {
	Name    string `json:"-"`
	Enabled *bool  `json:"enabled,omitempty"`
	// Type is optional: "stdio" or "http". When empty, inferred from command/url.
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// IsEnabled returns whether the server is enabled (default true).
func (e MCPServerEntry) IsEnabled() bool {
	if e.Enabled == nil {
		return true
	}
	return *e.Enabled
}

// ResolvedTransport returns stdio/http from type or from command/url presence.
func (e MCPServerEntry) ResolvedTransport() string {
	t := strings.ToLower(strings.TrimSpace(e.Type))
	switch t {
	case mcpTransportStdio, mcpTransportHTTP:
		return t
	case "sse", "streamable-http", "streamable_http":
		// Treat remote HTTP variants as streamable HTTP client.
		return mcpTransportHTTP
	}
	if strings.TrimSpace(e.Command) != "" {
		return mcpTransportStdio
	}
	if strings.TrimSpace(e.URL) != "" {
		return mcpTransportHTTP
	}
	return ""
}

// EnvSlice converts env map to KEY=VALUE pairs for stdio process APIs.
func (e MCPServerEntry) EnvSlice() []string {
	if len(e.Env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(e.Env))
	for k := range e.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+e.Env[k])
	}
	return out
}

// Entries returns servers sorted by name with Name set from map keys.
func (c *MCPFileConfig) Entries() []MCPServerEntry {
	if c == nil || len(c.MCPServers) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.MCPServers))
	for name := range c.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]MCPServerEntry, 0, len(names))
	for _, name := range names {
		e := c.MCPServers[name]
		e.Name = name
		out = append(out, e)
	}
	return out
}

// MCPServerInfo is the list/status view for UI.
type MCPServerInfo struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Available bool   `json:"available"`
	Transport string `json:"transport"`
	Error     string `json:"error,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// MCPConfigPath returns {userDataDir}/mcp.json.
func MCPConfigPath(userDataDir string) string {
	return filepath.Join(userDataDir, mcpConfigFileName)
}

// EnsureMCPConfig creates mcp.json with {"mcpServers":{}} when missing.
func EnsureMCPConfig(userDataDir string) (string, error) {
	if strings.TrimSpace(userDataDir) == "" {
		return "", fmt.Errorf("user data dir is empty")
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return "", fmt.Errorf("ensure user data dir: %w", err)
	}
	path := MCPConfigPath(userDataDir)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	cfg := MCPFileConfig{MCPServers: map[string]MCPServerEntry{}}
	if err := writeMCPConfig(path, &cfg); err != nil {
		return "", err
	}
	return path, nil
}

// LoadMCPConfig reads mcp.json, creating the default empty file when absent.
func LoadMCPConfig(userDataDir string) (*MCPFileConfig, error) {
	path, err := EnsureMCPConfig(userDataDir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg MCPFileConfig
	if len(strings.TrimSpace(string(data))) == 0 {
		cfg.MCPServers = map[string]MCPServerEntry{}
		return &cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp.json: %w", err)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]MCPServerEntry{}
	}
	normalized := make(map[string]MCPServerEntry, len(cfg.MCPServers))
	for rawName, entry := range cfg.MCPServers {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		entry.Name = name
		entry.Type = strings.ToLower(strings.TrimSpace(entry.Type))
		entry.Command = strings.TrimSpace(entry.Command)
		entry.URL = strings.TrimSpace(entry.URL)
		normalized[name] = entry
	}
	cfg.MCPServers = normalized
	return &cfg, nil
}

// SaveMCPEnabled updates one server's enabled flag and writes mcp.json.
func SaveMCPEnabled(userDataDir, name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("server name is required")
	}
	cfg, err := LoadMCPConfig(userDataDir)
	if err != nil {
		return err
	}
	entry, ok := cfg.MCPServers[name]
	if !ok {
		return fmt.Errorf("mcp server %q not found", name)
	}
	entry.Enabled = &enabled
	cfg.MCPServers[name] = entry
	return writeMCPConfig(MCPConfigPath(userDataDir), cfg)
}

func writeMCPConfig(path string, cfg *MCPFileConfig) error {
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]MCPServerEntry{}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func (e MCPServerEntry) summary() string {
	switch e.ResolvedTransport() {
	case mcpTransportStdio:
		parts := []string{e.Command}
		parts = append(parts, e.Args...)
		return strings.TrimSpace(strings.Join(parts, " "))
	case mcpTransportHTTP:
		return e.URL
	default:
		return e.Type
	}
}

func (e MCPServerEntry) validate() error {
	if e.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch e.ResolvedTransport() {
	case mcpTransportStdio:
		if e.Command == "" {
			return fmt.Errorf("command is required for stdio transport")
		}
	case mcpTransportHTTP:
		if e.URL == "" {
			return fmt.Errorf("url is required for http transport")
		}
	default:
		return fmt.Errorf("set command (stdio) or url (http), or type stdio/http")
	}
	return nil
}
