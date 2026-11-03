package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"
)

const WindowStateFileName = "window-state.toml"

const (
	DefaultWindowWidth  = 1600
	DefaultWindowHeight = 900
	minWindowWidth      = 400
	minWindowHeight     = 400
)

// WindowState is machine-local main-window geometry (not synced).
type WindowState struct {
	Width     int  `toml:"width"`
	Height    int  `toml:"height"`
	Maximised bool `toml:"maximised"`
}

// EffectiveSize returns a usable width/height, falling back to defaults when invalid.
func (w WindowState) EffectiveSize() (width, height int) {
	if w.Width < minWindowWidth || w.Height < minWindowHeight {
		return DefaultWindowWidth, DefaultWindowHeight
	}
	return w.Width, w.Height
}

// ClampToWorkArea shrinks size so it fits within workW×workH when those are positive.
func ClampToWorkArea(width, height, workW, workH int) (int, int) {
	if workW > 0 && width > workW {
		width = workW
	}
	if workH > 0 && height > workH {
		height = workH
	}
	return width, height
}

func (s *Store) windowStatePath() string {
	if s.windowStateFile != "" {
		return s.windowStateFile
	}
	return filepath.Join(s.Config.General.UserDataDir, WindowStateFileName)
}

// Window returns the in-memory main-window state (may be zero if never saved).
func (s *Store) Window() WindowState {
	return s.window
}

// LoadWindow reads window-state.toml from the user data directory.
func (s *Store) LoadWindow() {
	path := s.windowStatePath()
	s.windowStateFile = path
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) && s.logger != nil {
			s.logger.Debug("load window state skipped", zap.Error(err))
		}
		return
	}
	var st WindowState
	if err := toml.Unmarshal(data, &st); err != nil {
		if s.logger != nil {
			s.logger.Error("unmarshal window state failed", zap.Error(err))
		}
		return
	}
	s.window = st
}

// SaveWindow writes window-state.toml.
func (s *Store) SaveWindow(st WindowState) error {
	path := s.windowStatePath()
	// When maximised, keep last normal size if caller did not supply one.
	if st.Maximised && (st.Width <= 0 || st.Height <= 0) {
		st.Width = s.window.Width
		st.Height = s.window.Height
	}
	if !st.Maximised && (st.Width <= 0 || st.Height <= 0) {
		return fmt.Errorf("window size invalid: %dx%d", st.Width, st.Height)
	}
	data, err := toml.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshal window state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create window state dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write window state: %w", err)
	}
	s.window = st
	s.windowStateFile = path
	return nil
}
