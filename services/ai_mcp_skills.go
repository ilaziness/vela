package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
)

func (s *AIService) userDataDir() string {
	if s.configService == nil {
		return ""
	}
	return s.configService.userDataDir()
}

func (s *AIService) requireUserDataDir() (string, error) {
	dir := s.userDataDir()
	if dir == "" {
		return "", fmt.Errorf("user data dir is empty")
	}
	return dir, nil
}

func (s *AIService) ensureEngineDir() (*ai.AIEngine, string, error) {
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
func (s *AIService) ListMCPServers() ([]ai.MCPServerInfo, error) {
	engine, _, err := s.ensureEngineDir()
	if err != nil {
		return nil, err
	}
	return engine.ListMCPServers()
}

// ReloadMCPServers re-reads mcp.json, probes availability, and returns fresh status.
func (s *AIService) ReloadMCPServers() ([]ai.MCPServerInfo, error) {
	engine, _, err := s.ensureEngineDir()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := engine.ReloadMCPAndSkills(ctx); err != nil {
		return nil, err
	}
	return engine.ListMCPServers()
}

// SetMCPServerEnabled writes enabled into mcp.json, reloads connections, and returns fresh status.
func (s *AIService) SetMCPServerEnabled(name string, enabled bool) ([]ai.MCPServerInfo, error) {
	engine, dir, err := s.ensureEngineDir()
	if err != nil {
		return nil, err
	}
	if err := ai.SaveMCPEnabled(dir, name, enabled); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := engine.ReloadMCPAndSkills(ctx); err != nil {
		return nil, err
	}
	return engine.ListMCPServers()
}

// OpenMCPConfig opens mcp.json with the system default editor/viewer.
func (s *AIService) OpenMCPConfig() error {
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
func (s *AIService) ListSkills() ([]ai.SkillInfo, error) {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return nil, err
	}
	return ai.ListSkills(dir)
}

// SetSkillEnabled updates skills-state.json.
func (s *AIService) SetSkillEnabled(name string, enabled bool) error {
	dir, err := s.requireUserDataDir()
	if err != nil {
		return err
	}
	return ai.SetSkillEnabled(dir, name, enabled)
}

// OpenSkillsDir opens the skills directory.
func (s *AIService) OpenSkillsDir() error {
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
func (s *AIService) OpenSkillDir(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	if name != filepath.Base(name) || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid skill name")
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
		return fmt.Errorf("skill %q not found", name)
	}
	return ai.OpenPath(path)
}

// PickChatAttachments opens a native multi-file dialog and returns absolute paths.
func (s *AIService) PickChatAttachments() ([]string, error) {
	dlg := s.app.Dialog.OpenFile().
		SetTitle("选择附件").
		CanChooseFiles(true).
		CanChooseDirectories(false)
	dlg.AddFilter("附件", "*.png;*.jpg;*.jpeg;*.webp;*.gif;*.pdf;*.txt;*.md;*.csv")
	paths, err := dlg.PromptForMultipleSelection()
	if dialogCancelled(err) {
		return nil, nil
	}
	return paths, err
}

// ReadLocalAttachment returns a data URL for composer preview of a local file (not copied).
func (s *AIService) ReadLocalAttachment(path string) (string, error) {
	dataURL, err := ai.LocalFileDataURL(path)
	if err != nil {
		s.logger.Error("read local attachment", zap.Error(err))
		return "", err
	}
	return dataURL, nil
}

// ReadChatAttachment returns a data URL for in-app image preview.
func (s *AIService) ReadChatAttachment(uri, mime string) (string, error) {
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
func (s *AIService) OpenChatAttachment(uri, filename string) error {
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
func (s *AIService) ShutdownMCP() {
	s.engineMu.Lock()
	engine := s.engine
	s.engineMu.Unlock()
	if engine != nil {
		engine.Close()
	}
}
