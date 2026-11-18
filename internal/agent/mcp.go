package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/usererr"
)

func (s *Service) ensureEngineDir() (*ai.AIEngine, string, error) {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return nil, "", err
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if s.engine == nil {
		s.engine = ai.NewAIEngine(s.logger)
	}
	s.engine.SetUserDataDir(dir)
	return s.engine, dir, nil
}

// ListMCPServers returns cached MCP status (no probe). Hydrates from mcp.json when cache is empty.
func (s *Service) ListMCPServers() ([]ai.MCPServerInfo, error) {
	engine, _, err := s.ensureEngineDir()
	if err != nil {
		return nil, err
	}
	return engine.ListMCPServers()
}

// ReloadMCPServers re-reads mcp.json, probes availability, and returns fresh status.
func (s *Service) ReloadMCPServers() ([]ai.MCPServerInfo, error) {
	engine, _, err := s.ensureEngineDir()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := engine.ReloadMCP(ctx); err != nil {
		return nil, err
	}
	return engine.ListMCPServers()
}

// SetMCPServerEnabled writes enabled into mcp.json, reloads connections, and returns fresh status.
func (s *Service) SetMCPServerEnabled(name string, enabled bool) ([]ai.MCPServerInfo, error) {
	engine, dir, err := s.ensureEngineDir()
	if err != nil {
		return nil, err
	}
	if err := ai.SaveMCPEnabled(dir, name, enabled); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := engine.ReloadMCP(ctx); err != nil {
		return nil, err
	}
	return engine.ListMCPServers()
}

// OpenMCPConfig opens mcp.json with the system default editor/viewer.
func (s *Service) OpenMCPConfig() error {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return err
	}
	path, err := ai.EnsureMCPConfig(dir)
	if err != nil {
		return err
	}
	return ai.OpenPath(path)
}

// ListSkills scans skills/ and skills-state.json.
func (s *Service) ListSkills() ([]ai.SkillInfo, error) {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return nil, err
	}
	return ai.ListSkills(dir)
}

// SetSkillEnabled updates skills-state.json then publishes skills into the catalog.
func (s *Service) SetSkillEnabled(name string, enabled bool) error {
	engine, dir, err := s.ensureEngineDir()
	if err != nil {
		return err
	}
	if err := ai.SetSkillEnabled(dir, name, enabled); err != nil {
		return err
	}
	return engine.PublishSkills()
}

// ReloadSkills rescans skills/ from disk and republishes them into the tool catalog.
func (s *Service) ReloadSkills() error {
	engine, _, err := s.ensureEngineDir()
	if err != nil {
		return err
	}
	return engine.PublishSkills()
}

// HydrateCatalog loads builtin + skills into the memory catalog without connecting MCP.
func (s *Service) HydrateCatalog() {
	engine, _, err := s.ensureEngineDir()
	if err != nil {
		s.logger.Warn("hydrate catalog: engine dir", zap.Error(err))
		return
	}
	cfg, err := s.GetConfig()
	if err != nil {
		s.logger.Warn("hydrate catalog: get config", zap.Error(err))
	} else if cfg != nil {
		engine.PublishBuiltinFromConfig(cfg)
	}
	if err := engine.PublishSkills(); err != nil {
		s.logger.Warn("hydrate catalog: skills", zap.Error(err))
	}
}

// OpenSkillsDir opens the skills directory.
func (s *Service) OpenSkillsDir() error {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return err
	}
	skillsDir, err := ai.EnsureSkillsDir(dir)
	if err != nil {
		return err
	}
	return ai.OpenPath(skillsDir)
}

// OpenSkillDir opens one skill subdirectory.
func (s *Service) OpenSkillDir(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return usererr.Coded("ai.skill_name_required", "Skill name is required")
	}
	if name != filepath.Base(name) || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return usererr.Coded("ai.skill_name_invalid", "Invalid skill name")
	}
	dir, err := s.requireUserDataDir()
	if err != nil {
		return err
	}
	skillsDir, err := ai.EnsureSkillsDir(dir)
	if err != nil {
		return err
	}
	path := filepath.Join(skillsDir, name)
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		return usererr.Coded("ai.skill_not_found", `Skill "%s" was not found`, name)
	}
	return ai.OpenPath(path)
}

// ReadLocalAttachment returns a data URL for composer preview of a local file (not copied).
func (s *Service) ReadLocalAttachment(path string) (string, error) {
	dataURL, err := ai.LocalFileDataURL(path)
	if err != nil {
		s.logger.Error("read local attachment", zap.Error(err))
		return "", err
	}
	return dataURL, nil
}

// ReadChatAttachment returns a data URL for in-app image preview.
func (s *Service) ReadChatAttachment(uri, mime string) (string, error) {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return "", err
	}
	dataURL, err := ai.AttachmentDataURL(dir, uri, mime)
	if err != nil {
		s.logger.Error("read chat attachment", zap.Error(err))
		return "", err
	}
	return dataURL, nil
}

// OpenChatAttachment opens a chat attachment with the OS default application.
func (s *Service) OpenChatAttachment(uri, filename string) error {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return err
	}
	if err := ai.OpenChatAttachment(dir, uri, filename); err != nil {
		s.logger.Error("open chat attachment", zap.Error(err))
		return err
	}
	return nil
}

// ShutdownMCP closes MCP child processes / connections.
func (s *Service) ShutdownMCP() {
	s.engineMu.Lock()
	engine := s.engine
	s.engineMu.Unlock()
	if engine != nil {
		engine.Close()
	}
}
