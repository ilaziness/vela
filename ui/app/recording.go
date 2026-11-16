package app

import (
	"os"
	"path/filepath"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
)

// ToggleSessionRecording starts a .gtr session recording on the session's
// terminal, or stops the one already running. Recording is per-terminal,
// matching go-term's model: the file replays through NewReplay/gotermrec.
//
// The destination is <record_dir setting>/<timestamp>-<sessID>.gtr; when the
// setting is empty it falls back to <user_data_dir>/recordings. record_dir is
// read live from termCfg on every start, so a settings change applies to all
// sessions immediately.
//
// Main-thread only: Term.StartRecording/StopRecording require it, which the
// context-menu Action callback guarantees.
func (c *Controller) ToggleSessionRecording(w *gui.Window, sessID string) {
	tm := c.TermFor(sessID)
	if tm == nil {
		return
	}
	if tm.Recording() {
		if err := tm.StopRecording(); err != nil {
			c.logger.Error("stop session recording", zap.String("sess", sessID), zap.Error(err))
			w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
			return
		}
		w.Toast(gui.ToastCfg{
			Body:     i18n.T("terminal.log_stopped"),
			Severity: gui.ToastSuccess,
			Duration: 3 * time.Second,
		})
		return
	}
	path, err := c.recordingPath(sessID)
	if err != nil {
		c.logger.Error("resolve recording path", zap.String("sess", sessID), zap.Error(err))
		w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	if err := tm.StartRecording(path); err != nil {
		c.logger.Error("start session recording", zap.String("sess", sessID), zap.Error(err))
		w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	w.Toast(gui.ToastCfg{
		Body:     i18n.Tf("terminal.log_started", path),
		Severity: gui.ToastSuccess,
		Duration: 4 * time.Second,
	})
}

// recordingPath builds the destination for a new recording:
// <record_dir or user_data_dir/recordings>/<timestamp>-<sessID>.gtr.
func (c *Controller) recordingPath(sessID string) (string, error) {
	c.mu.Lock()
	dir := c.termCfg.RecordDir
	var storeDir string
	if c.store != nil {
		storeDir = c.store.UserDataDir()
	}
	c.mu.Unlock()

	if dir == "" {
		if storeDir != "" {
			dir = filepath.Join(storeDir, "recordings")
		} else {
			// store unset (tests): park under the user cache dir rather
			// than the process working directory.
			cacheDir, err := os.UserCacheDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(cacheDir, "vela", "recordings")
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Session ids are hex (utils.GenerateRandomID), safe in a filename.
	name := time.Now().Format("2006-01-02T15-04-05") + "-" + sessID + ".gtr"
	return filepath.Join(dir, name), nil
}
