package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const skillsOverlayDirName = ".skills-enabled"

// ensureSkillsOverlay builds {userDataDir}/.skills-enabled with only enabled skill dirs
// linked in (symlink, or junction-style symlink on Windows). Returns overlay path and cleanup.
func ensureSkillsOverlay(userDataDir string, skills []EnabledSkill) (string, func(), error) {
	if strings.TrimSpace(userDataDir) == "" {
		return "", nil, fmt.Errorf("user data dir is empty")
	}
	overlay := filepath.Join(userDataDir, skillsOverlayDirName)
	if err := os.RemoveAll(overlay); err != nil && !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("clear skills overlay: %w", err)
	}
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		return "", nil, fmt.Errorf("create skills overlay: %w", err)
	}
	for _, s := range skills {
		name := strings.TrimSpace(s.Name)
		src := strings.TrimSpace(s.Path)
		if name == "" || src == "" {
			continue
		}
		dst := filepath.Join(overlay, name)
		if err := os.Symlink(src, dst); err != nil {
			// Fallback: copy is too heavy; surface the error so Run fails loudly.
			_ = os.RemoveAll(overlay)
			return "", nil, fmt.Errorf("link skill %q into overlay: %w", name, err)
		}
	}
	cleanup := func() {
		_ = os.RemoveAll(overlay)
	}
	return overlay, cleanup, nil
}
