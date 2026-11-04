package ssh

import (
	"os"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/usererr"
)

const (
	SessionLogStopReasonWriteError = "write_error"
	SessionLogStopReasonClosed     = "closed"
)

// SetOnSessionLogStopped registers a callback when logging stops due to write
// error or session close (not manual StopSessionLog).
func (m *Manager) SetOnSessionLogStopped(fn func(sessionID, reason string)) {
	m.onSessionLogStopped = fn
}

// StartSessionLog begins teeing session output to path (O_TRUNC). Fails if
// already recording; caller must StopSessionLog first.
// OpenFile runs without session locks so closePTY is not blocked on disk I/O.
func (m *Manager) StartSessionLog(id, path string) error {
	if path == "" {
		return usererr.Coded("ssh.log_path_required", "Log path is required")
	}
	sess, err := m.GetSession(id)
	if err != nil {
		return err
	}
	sess.closeMu.Lock()
	closed := sess.isClosed
	sess.closeMu.Unlock()
	if closed {
		return usererr.Coded("ssh.session_closed", "Session is closed")
	}
	// Reject before OpenFile so we do not truncate a path only to fail attach.
	sess.logMu.Lock()
	already := sess.logFile != nil
	sess.logMu.Unlock()
	if already {
		return usererr.Coded("ssh.log_already_recording", "Session log is already recording")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	// Lock order: closeMu then logMu (same as closePTY).
	sess.closeMu.Lock()
	defer sess.closeMu.Unlock()
	if sess.isClosed {
		_ = f.Close()
		return usererr.Coded("ssh.session_closed", "Session is closed")
	}
	sess.logMu.Lock()
	defer sess.logMu.Unlock()
	if sess.logFile != nil {
		_ = f.Close()
		return usererr.Coded("ssh.log_already_recording", "Session log is already recording")
	}
	sess.logFile = f
	return nil
}

// StopSessionLog stops recording. Idempotent if not logging or session missing.
func (m *Manager) StopSessionLog(id string) error {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil
	}
	v.(*Session).stopSessionLog()
	return nil
}

// IsSessionLogging reports whether id currently has an open log file.
func (m *Manager) IsSessionLogging(id string) bool {
	sess, err := m.GetSession(id)
	if err != nil {
		return false
	}
	sess.logMu.Lock()
	defer sess.logMu.Unlock()
	return sess.logFile != nil
}

func (sc *Session) teeOutput(data []byte) {
	if len(data) == 0 {
		return
	}
	sc.logMu.Lock()
	if sc.logFile == nil {
		sc.logMu.Unlock()
		return
	}
	_, err := sc.logFile.Write(data)
	if err == nil {
		sc.logMu.Unlock()
		return
	}
	id := sc.ID
	sc.manager.logger.Error("session log write failed",
		zap.String("id", id), zap.Error(err))
	sc.closeLogFileLocked()
	sc.logMu.Unlock()
	sc.emitLogStopped(id, SessionLogStopReasonWriteError)
}

func (sc *Session) stopSessionLog() {
	sc.logMu.Lock()
	sc.closeLogFileLocked()
	sc.logMu.Unlock()
}

func (sc *Session) closeLogFileLocked() (id string, stopped bool) {
	if sc.logFile == nil {
		return "", false
	}
	_ = sc.logFile.Close()
	sc.logFile = nil
	return sc.ID, true
}

func (sc *Session) emitLogStopped(id, reason string) {
	if sc.manager == nil || sc.manager.onSessionLogStopped == nil || id == "" {
		return
	}
	sc.manager.onSessionLogStopped(id, reason)
}
