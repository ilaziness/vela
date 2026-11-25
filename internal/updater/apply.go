package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/applog"
)

var errProcessGone = errors.New("process already exited")

const applyArg = "--vela-updater-apply"

type applyManifest struct {
	ParentPID int      `json:"parent_pid"`
	OldExe    string   `json:"old_exe"`
	NewExe    string   `json:"new_exe"`
	WorkDir   string   `json:"work_dir"`
	Args      []string `json:"args"`
	TempDir   string   `json:"temp_dir"`
}

// RunApplyMode handles the hidden updater helper invocation. It returns
// handled=false for normal app args.
func RunApplyMode(args []string) (bool, error) {
	if len(args) < 2 || args[0] != applyArg {
		return false, nil
	}
	return true, applyManifestFile(args[1], applog.L())
}

func applyManifestFile(path string, logger *zap.Logger) error {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Error("updater manifest read failed", zap.String("path", path), zap.Error(err))
		return err
	}
	var m applyManifest
	if err := json.Unmarshal(data, &m); err != nil {
		logger.Error("updater manifest parse failed", zap.String("path", path), zap.Error(err))
		return err
	}
	if m.TempDir != "" {
		defer func() { _ = os.RemoveAll(m.TempDir) }()
	}
	logger.Info("updater apply started",
		zap.Int("parent_pid", m.ParentPID),
		zap.String("old_exe", m.OldExe),
		zap.String("new_exe", m.NewExe),
		zap.String("work_dir", m.WorkDir))
	if err := waitForExit(m.ParentPID, 30*time.Second); err != nil && !errors.Is(err, errProcessGone) {
		logger.Error("updater wait parent failed", zap.Int("pid", m.ParentPID), zap.Error(err))
		return err
	}
	if err := applyUpdate(m, logger); err != nil {
		logger.Error("updater apply failed", zap.Error(err))
		return err
	}
	logger.Info("updater apply finished")
	return nil
}

func applyUpdate(m applyManifest, logger *zap.Logger) error {
	if m.OldExe == "" || m.NewExe == "" {
		return errors.New("invalid apply manifest")
	}
	backup := m.OldExe + ".old"
	if _, err := os.Stat(m.OldExe); errors.Is(err, os.ErrNotExist) {
		if _, berr := os.Stat(backup); berr == nil {
			_ = os.Rename(backup, m.OldExe)
		}
		if _, err := os.Stat(m.OldExe); errors.Is(err, os.ErrNotExist) {
			if err := installNewExe(m.NewExe, m.OldExe); err != nil {
				return fmt.Errorf("install new exe: %w", err)
			}
			if err := launchApp(m.OldExe, m.Args, m.WorkDir); err != nil {
				return fmt.Errorf("launch updated app: %w", err)
			}
			logger.Info("updater installed new exe without backup")
			return nil
		}
	}
	_ = os.Remove(backup)
	if err := renameWithRetry(m.OldExe, backup, 30*time.Second); err != nil {
		_ = launchApp(m.OldExe, m.Args, m.WorkDir)
		return fmt.Errorf("backup current exe: %w", err)
	}
	logger.Debug("updater current exe backed up", zap.String("backup", backup))
	if err := installNewExe(m.NewExe, m.OldExe); err != nil {
		_ = os.Rename(backup, m.OldExe)
		_ = launchApp(m.OldExe, m.Args, m.WorkDir)
		return fmt.Errorf("install new exe: %w", err)
	}
	logger.Info("updater new exe installed", zap.String("exe", m.OldExe))
	if err := launchApp(m.OldExe, m.Args, m.WorkDir); err != nil {
		if _, serr := os.Stat(backup); serr == nil {
			_ = os.Rename(backup, m.OldExe)
			_ = launchApp(m.OldExe, m.Args, m.WorkDir)
		}
		logger.Warn("updater launch failed; restored backup", zap.String("exe", m.OldExe), zap.Error(err))
		return fmt.Errorf("launch updated app: %w", err)
	}
	logger.Info("updater relaunched app", zap.String("exe", m.OldExe))
	return nil
}

// installNewExe copies src beside dst and renames it into place. The temp
// download may live on a different volume than the install dir, where
// os.Rename cannot move files directly.
func installNewExe(src, dst string) error {
	tmp := dst + ".new"
	_ = os.Remove(tmp)
	if err := copyFile(src, tmp); err != nil {
		return fmt.Errorf("copy new exe: %w", err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil && runtime.GOOS != "windows" {
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod new exe: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename new exe: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func renameWithRetry(oldPath, newPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var err error
	for {
		err = os.Rename(oldPath, newPath)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func launchApp(path string, args []string, dir string) error {
	cmd := exec.Command(path, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = os.Environ()
	return cmd.Start()
}

// CleanupLeftovers removes the previous .old backup and stale update temp dirs.
func CleanupLeftovers(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	exe := exePathOfSelf()
	if exe != "" {
		old := exe + ".old"
		if err := os.Remove(old); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Warn("updater cleanup failed", zap.String("path", old), zap.Error(err))
		}
	}
	tmp := os.TempDir()
	matches, err := filepath.Glob(filepath.Join(tmp, "vela-update-*"))
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && fi.ModTime().Before(cutoff) {
			if err := os.RemoveAll(m); err != nil {
				logger.Debug("updater temp cleanup failed", zap.String("path", m), zap.Error(err))
			}
		}
	}
}
