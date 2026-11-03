package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"
)

const LocalStateFileName = "local-state.toml"

const (
	DefaultWindowWidth  = 1600
	DefaultWindowHeight = 900
	minWindowWidth      = 400
	minWindowHeight     = 400
)

// LocalState is machine-local UI state (not synced).
type LocalState struct {
	Window        WindowGeometry    `toml:"window"`
	LastDialogDir string            `toml:"last_dialog_dir,omitempty"`
	SftpLocalDirs map[string]string `toml:"sftp_local_dirs,omitempty"`
}

// WindowGeometry is main-window size/maximised flag.
type WindowGeometry struct {
	Width     int  `toml:"width"`
	Height    int  `toml:"height"`
	Maximised bool `toml:"maximised"`
}

// EffectiveSize returns a usable width/height, falling back to defaults when invalid.
func (w WindowGeometry) EffectiveSize() (width, height int) {
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

func isExistingDir(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// Window returns the in-memory main-window geometry (may be zero if never saved).
func (s *Store) Window() WindowGeometry {
	s.localMu.Lock()
	defer s.localMu.Unlock()
	return s.local.Window
}

// LastDialogDir returns the remembered native-dialog directory if it still exists.
func (s *Store) LastDialogDir() string {
	s.localMu.Lock()
	dir := s.local.LastDialogDir
	s.localMu.Unlock()
	if isExistingDir(dir) {
		return dir
	}
	return ""
}

// SftpLocalDir returns the remembered local SFTP browse dir for ownerKey if still valid.
func (s *Store) SftpLocalDir(ownerKey string) string {
	if ownerKey == "" {
		return ""
	}
	s.localMu.Lock()
	var dir string
	if s.local.SftpLocalDirs != nil {
		dir = s.local.SftpLocalDirs[ownerKey]
	}
	s.localMu.Unlock()
	if isExistingDir(dir) {
		return dir
	}
	return ""
}

// LoadLocalState reads local-state.toml from the user data directory.
func (s *Store) LoadLocalState() {
	path := filepath.Join(s.Config.General.UserDataDir, LocalStateFileName)
	s.localMu.Lock()
	s.localStateFile = path
	s.localMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			s.logger.Debug("load local state skipped", zap.Error(err))
		}
		return
	}
	var st LocalState
	if err := toml.Unmarshal(data, &st); err != nil {
		s.logger.Error("unmarshal local state failed", zap.Error(err))
		return
	}
	s.localMu.Lock()
	s.local = st
	s.localMu.Unlock()
}

func (s *Store) writeLocalStateLocked() error {
	path := s.localStateFile
	if path == "" {
		path = filepath.Join(s.Config.General.UserDataDir, LocalStateFileName)
		s.localStateFile = path
	}
	data, err := toml.Marshal(&s.local)
	if err != nil {
		return fmt.Errorf("marshal local state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create local state dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write local state: %w", err)
	}
	return nil
}

// SaveWindow updates window geometry and writes local-state.toml.
func (s *Store) SaveWindow(st WindowGeometry) error {
	s.localMu.Lock()
	defer s.localMu.Unlock()
	if st.Maximised && (st.Width <= 0 || st.Height <= 0) {
		st.Width = s.local.Window.Width
		st.Height = s.local.Window.Height
	}
	if !st.Maximised && (st.Width <= 0 || st.Height <= 0) {
		return fmt.Errorf("window size invalid: %dx%d", st.Width, st.Height)
	}
	s.local.Window = st
	return s.writeLocalStateLocked()
}

// SetLastDialogDir persists a native-dialog directory when it exists.
func (s *Store) SetLastDialogDir(dir string) error {
	if !isExistingDir(dir) {
		return nil
	}
	s.localMu.Lock()
	defer s.localMu.Unlock()
	if s.local.LastDialogDir == dir {
		return nil
	}
	s.local.LastDialogDir = dir
	return s.writeLocalStateLocked()
}

// SetSftpLocalDir persists the local SFTP browse directory for ownerKey.
func (s *Store) SetSftpLocalDir(ownerKey, dir string) error {
	if ownerKey == "" || !isExistingDir(dir) {
		return nil
	}
	s.localMu.Lock()
	defer s.localMu.Unlock()
	if s.local.SftpLocalDirs == nil {
		s.local.SftpLocalDirs = make(map[string]string)
	}
	if s.local.SftpLocalDirs[ownerKey] == dir {
		return nil
	}
	s.local.SftpLocalDirs[ownerKey] = dir
	return s.writeLocalStateLocked()
}
