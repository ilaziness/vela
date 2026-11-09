package serial

import (
	"os"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/usererr"
)

// StartSessionLog begins teeing session output to path (O_TRUNC).
func (m *Manager) StartSessionLog(id, path string) error {
	if path == "" {
		return usererr.Coded("serial.log_path_required", "Log path is required")
	}
	sess, err := m.GetSession(id)
	if err != nil {
		return err
	}
	sess.closeMu.Lock()
	closed := sess.isClosed
	sess.closeMu.Unlock()
	if closed {
		return usererr.Coded("serial.session_closed", "Session is closed")
	}
	sess.logMu.Lock()
	already := sess.logFile != nil
	sess.logMu.Unlock()
	if already {
		return usererr.Coded("serial.log_already_recording", "Session log is already recording")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	sess.closeMu.Lock()
	defer sess.closeMu.Unlock()
	if sess.isClosed {
		_ = f.Close()
		return usererr.Coded("serial.session_closed", "Session is closed")
	}
	sess.logMu.Lock()
	defer sess.logMu.Unlock()
	if sess.logFile != nil {
		_ = f.Close()
		return usererr.Coded("serial.log_already_recording", "Session log is already recording")
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

func (s *Session) teeOutput(data []byte) {
	if len(data) == 0 {
		return
	}
	s.logMu.Lock()
	if s.logFile == nil {
		s.logMu.Unlock()
		return
	}
	_, err := s.logFile.Write(data)
	if err == nil {
		s.logMu.Unlock()
		return
	}
	id := s.id
	if s.manager != nil {
		s.manager.logger.Error("session log write failed",
			zap.String("id", id), zap.Error(err))
	}
	s.closeLogFileLocked()
	s.logMu.Unlock()
	s.emitLogStopped(id, SessionLogStopReasonWriteError)
}

func (s *Session) stopSessionLog() {
	s.logMu.Lock()
	s.closeLogFileLocked()
	s.logMu.Unlock()
}

func (s *Session) closeLogFileLocked() (id string, stopped bool) {
	if s.logFile == nil {
		return "", false
	}
	_ = s.logFile.Close()
	s.logFile = nil
	return s.id, true
}

func (s *Session) emitLogStopped(id, reason string) {
	if s.manager == nil || s.manager.onSessionLogStopped == nil || id == "" {
		return
	}
	s.manager.onSessionLogStopped(id, reason)
}
