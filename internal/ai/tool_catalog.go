package ai

import (
	"path/filepath"
	"strings"

	genkitAI "github.com/firebase/genkit/go/ai"

	"github.com/ilaziness/vela/internal/ai/tool"
)

// EnabledSkill is one enabled skill entry for Run (prompt list + use_skill path).
type EnabledSkill struct {
	Name        string
	Description string
	Path        string // skill directory (contains SKILL.md)
}

// CatalogSnapshot is an atomic, whole-table view of tool availability for Chat/Run.
// It holds no secrets and no SSH session state.
type CatalogSnapshot struct {
	AllowDirs       []string
	EnableFS        bool
	EnableWebSearch bool
	MCPTools        []genkitAI.Tool
	Skills          []EnabledSkill
}

func (s CatalogSnapshot) clone() CatalogSnapshot {
	out := CatalogSnapshot{
		EnableFS:        s.EnableFS,
		EnableWebSearch: s.EnableWebSearch,
	}
	if len(s.AllowDirs) > 0 {
		out.AllowDirs = append([]string(nil), s.AllowDirs...)
	}
	if len(s.MCPTools) > 0 {
		out.MCPTools = append([]genkitAI.Tool(nil), s.MCPTools...)
	}
	if len(s.Skills) > 0 {
		out.Skills = append([]EnabledSkill(nil), s.Skills...)
	}
	return out
}

func (e *AIEngine) storeSnapLocked(s CatalogSnapshot) {
	cp := s.clone()
	e.catalog.Store(&cp)
}

func (e *AIEngine) loadSnap() CatalogSnapshot {
	v := e.catalog.Load()
	if v == nil {
		return CatalogSnapshot{}
	}
	return v.(*CatalogSnapshot).clone()
}

// Snapshot returns a copy of the current catalog for one Run/Chat.
func (e *AIEngine) Snapshot() CatalogSnapshot {
	return e.loadSnap()
}

// PublishBuiltinFromConfig updates allowlist / FS / web-search flags from config (no MCP).
func (e *AIEngine) PublishBuiltinFromConfig(cfg *Config) {
	dirs := cfg.EffectiveAllowedDirs()
	enableWS := webSearchEnabledFromConfig(cfg)
	e.catalogMu.Lock()
	defer e.catalogMu.Unlock()
	next := e.loadSnap()
	next.AllowDirs = dirs
	next.EnableFS = len(dirs) > 0
	next.EnableWebSearch = enableWS
	e.storeSnapLocked(next)
}

// PublishSkills reloads enabled skills from disk into the snapshot (no MCP).
func (e *AIEngine) PublishSkills() error {
	e.mu.RLock()
	dir := e.userDataDir
	e.mu.RUnlock()
	skills, err := enabledSkillsFromDisk(dir)
	if err != nil {
		return err
	}
	e.catalogMu.Lock()
	defer e.catalogMu.Unlock()
	next := e.loadSnap()
	next.Skills = skills
	e.storeSnapLocked(next)
	return nil
}

// publishMCP copies live MCP tool refs from mcpRuntime into the snapshot.
func (e *AIEngine) publishMCP() {
	e.mu.RLock()
	mcp := e.mcp
	e.mu.RUnlock()
	var tools []genkitAI.Tool
	if mcp != nil {
		tools = mcp.ActiveTools()
	}
	e.catalogMu.Lock()
	defer e.catalogMu.Unlock()
	next := e.loadSnap()
	next.MCPTools = tools
	e.storeSnapLocked(next)
}

// clearMCPTools empties MCP tools in the snapshot (e.g. after AI disable / Close).
func (e *AIEngine) clearMCPTools() {
	e.catalogMu.Lock()
	defer e.catalogMu.Unlock()
	next := e.loadSnap()
	next.MCPTools = nil
	e.storeSnapLocked(next)
}

func webSearchEnabledFromConfig(cfg *Config) bool {
	if cfg == nil || !tool.ValidSearchProvider(cfg.WebSearch.Provider) {
		return false
	}
	return strings.TrimSpace(cfg.WebSearch.APIKey) != "" ||
		strings.TrimSpace(cfg.WebSearch.APIKeyEncrypted) != ""
}

func enabledSkillsFromDisk(userDataDir string) ([]EnabledSkill, error) {
	if strings.TrimSpace(userDataDir) == "" {
		return nil, nil
	}
	list, err := ListSkills(userDataDir)
	if err != nil {
		return nil, err
	}
	out := make([]EnabledSkill, 0)
	for _, s := range list {
		if !s.Enabled {
			continue
		}
		desc := strings.TrimSpace(s.Description)
		if desc == "" {
			desc = skillsMissingDescLabel
		}
		out = append(out, EnabledSkill{
			Name:        s.Name,
			Description: desc,
			Path:        s.Path,
		})
	}
	return out, nil
}

func skillMarkdownPath(skillDir string) string {
	return filepath.Join(skillDir, skillMarkdownName)
}
