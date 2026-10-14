package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	skillsDirName       = "skills"
	skillsStateFileName = "skills-state.json"
	skillMarkdownName   = "SKILL.md"
)

// SkillsStateFile stores disabled skill names only.
type SkillsStateFile struct {
	Disabled []string `json:"disabled"`
}

// SkillInfo is one discovered skill for UI and runtime.
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Path        string `json:"path"`
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// SkillsDir returns {userDataDir}/skills.
func SkillsDir(userDataDir string) string {
	return filepath.Join(userDataDir, skillsDirName)
}

// SkillsStatePath returns {userDataDir}/skills-state.json.
func SkillsStatePath(userDataDir string) string {
	return filepath.Join(userDataDir, skillsStateFileName)
}

// EnsureSkillsDir creates the skills directory when missing.
func EnsureSkillsDir(userDataDir string) (string, error) {
	if strings.TrimSpace(userDataDir) == "" {
		return "", fmt.Errorf("user data dir is empty")
	}
	dir := SkillsDir(userDataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("ensure skills dir: %w", err)
	}
	return dir, nil
}

// LoadSkillsState reads skills-state.json (missing => empty disabled list).
func LoadSkillsState(userDataDir string) (*SkillsStateFile, error) {
	path := SkillsStatePath(userDataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SkillsStateFile{Disabled: []string{}}, nil
		}
		return nil, err
	}
	var st SkillsStateFile
	if len(strings.TrimSpace(string(data))) == 0 {
		return &SkillsStateFile{Disabled: []string{}}, nil
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parse skills-state.json: %w", err)
	}
	if st.Disabled == nil {
		st.Disabled = []string{}
	}
	return &st, nil
}

func saveSkillsState(userDataDir string, st *SkillsStateFile) error {
	if st.Disabled == nil {
		st.Disabled = []string{}
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(SkillsStatePath(userDataDir), data, 0o600)
}

// ListSkills scans skills/*/SKILL.md and merges enabled state.
func ListSkills(userDataDir string) ([]SkillInfo, error) {
	dir, err := EnsureSkillsDir(userDataDir)
	if err != nil {
		return nil, err
	}
	st, err := LoadSkillsState(userDataDir)
	if err != nil {
		return nil, err
	}
	disabled := make(map[string]bool, len(st.Disabled))
	for _, n := range st.Disabled {
		n = strings.TrimSpace(n)
		if n != "" {
			disabled[n] = true
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]SkillInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillMD := filepath.Join(dir, entry.Name(), skillMarkdownName)
		data, err := os.ReadFile(skillMD)
		if err != nil {
			continue
		}
		fm := parseSkillFrontmatter(data)
		desc := strings.TrimSpace(fm.Description)
		out = append(out, SkillInfo{
			Name:        entry.Name(),
			Description: desc,
			Enabled:     !disabled[entry.Name()],
			Path:        filepath.Join(dir, entry.Name()),
		})
	}
	return out, nil
}

// EnabledSkillPaths returns absolute paths of enabled skill directories for middleware.
func EnabledSkillPaths(userDataDir string) ([]string, error) {
	skills, err := ListSkills(userDataDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for _, s := range skills {
		if s.Enabled {
			paths = append(paths, s.Path)
		}
	}
	return paths, nil
}

// SetSkillEnabled updates skills-state.json disabled list.
func SetSkillEnabled(userDataDir, name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	if name != filepath.Base(name) || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid skill name")
	}
	skillsDir, err := EnsureSkillsDir(userDataDir)
	if err != nil {
		return err
	}
	skillPath := filepath.Join(skillsDir, name)
	if st, err := os.Stat(skillPath); err != nil || !st.IsDir() {
		return fmt.Errorf("skill %q not found", name)
	}
	st, err := LoadSkillsState(userDataDir)
	if err != nil {
		return err
	}
	next := make([]string, 0, len(st.Disabled))
	seen := false
	for _, n := range st.Disabled {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if n == name {
			seen = true
			if !enabled {
				next = append(next, n)
			}
			continue
		}
		next = append(next, n)
	}
	if !enabled && !seen {
		next = append(next, name)
	}
	st.Disabled = next
	return saveSkillsState(userDataDir, st)
}

func parseSkillFrontmatter(content []byte) skillFrontmatter {
	var fm skillFrontmatter
	text := strings.TrimPrefix(string(content), "\ufeff")
	if !strings.HasPrefix(text, "---") {
		return fm
	}
	rest := text[3:]
	if !strings.HasPrefix(rest, "\n") && !strings.HasPrefix(rest, "\r\n") {
		return fm
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return fm
	}
	_ = yaml.Unmarshal([]byte(rest[:idx]), &fm)
	return fm
}
