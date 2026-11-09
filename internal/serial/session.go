package serial

import (
	"errors"
	"io"
	"os"
	"sync"

	goserial "go.bug.st/serial"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/internal/termcharset"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/usererr"
)

const (
	SessionLogStopReasonWriteError = "write_error"
	SessionLogStopReasonClosed     = "closed"
)

// Session is an open serial port terminal session.
type Session struct {
	id         string
	label      string
	device     string
	cfg        OpenConfig
	port       goserial.Port
	stdin      io.WriteCloser
	outputChan chan []byte
	manager    *Manager

	closeMu    sync.Mutex
	isClosed   bool
	stopOutput chan struct{}
	stopOnce   sync.Once
	outputOnce sync.Once
	outputWg   sync.WaitGroup

	charset   string
	inEncoder *termcharset.Stream

	logMu   sync.Mutex
	logFile *os.File
}

var _ termio.Session = (*Session)(nil)

func (s *Session) ID() string                { return s.id }
func (s *Session) Protocol() termio.Protocol { return termio.ProtocolSerial }
func (s *Session) Label() string             { return s.label }
func (s *Session) Device() string            { return s.device }
func (s *Session) BaudRate() int             { return s.cfg.BaudRate }
func (s *Session) Stdin() io.WriteCloser     { return s.stdin }
func (s *Session) Output() <-chan []byte     { return s.outputChan }

func (s *Session) Start(_, _ int) error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.isClosed {
		return usererr.Coded("serial.session_closed", "Session is closed")
	}
	if s.stdin != nil {
		return nil
	}
	s.stdin = termcharset.WrapWriteCloser(&portWriter{port: s.port}, s.inEncoder)
	s.outputWg.Go(s.readLoop)
	return nil
}

func (s *Session) Resize(_, _ int) error { return nil }

func (s *Session) Close() error {
	return s.closePort(true)
}

func (s *Session) closePort(notify bool) error {
	s.closeMu.Lock()
	if s.isClosed {
		s.closeMu.Unlock()
		return nil
	}
	s.isClosed = true
	id := s.id
	device := s.device
	if s.stdin != nil {
		_ = s.stdin.Close()
		s.stdin = nil
	}
	s.logMu.Lock()
	logID, logStopped := s.closeLogFileLocked()
	s.logMu.Unlock()
	s.stopOnce.Do(func() { close(s.stopOutput) })
	port := s.port
	s.port = nil
	s.closeMu.Unlock()

	if port != nil {
		_ = port.Close()
	}
	if s.manager != nil {
		s.manager.releaseDevice(device, id)
	}
	if logStopped {
		s.emitLogStopped(logID, SessionLogStopReasonClosed)
	}
	go func() {
		s.outputWg.Wait()
		s.closeOutputChan()
	}()
	if notify && s.manager != nil {
		s.manager.notifyClosed(id)
	}
	return nil
}

func (s *Session) readLoop() {
	defer system.RecoverFromPanic()
	decoder, err := termcharset.NewDecoder(s.charset)
	if err != nil && s.manager != nil {
		s.manager.logger.Warn("serial charset decoder",
			zap.String("id", s.id), zap.String("charset", s.charset), zap.Error(err))
	}
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.stopOutput:
			return
		default:
		}
		s.closeMu.Lock()
		port := s.port
		closed := s.isClosed
		s.closeMu.Unlock()
		if closed || port == nil {
			return
		}
		n, readErr := port.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			if decoder != nil {
				data = decoder.Bytes(data)
			}
			s.teeOutput(data)
			select {
			case s.outputChan <- data:
			case <-s.stopOutput:
				return
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				if decoder != nil {
					flush := decoder.Flush()
					if len(flush) > 0 {
						s.teeOutput(flush)
						select {
						case s.outputChan <- flush:
						case <-s.stopOutput:
						}
					}
				}
			} else if s.manager != nil {
				s.manager.logger.Error("serial read failed",
					zap.String("id", s.id), zap.Error(readErr))
			}
			_ = s.closePort(true)
			return
		}
	}
}

func (s *Session) closeOutputChan() {
	s.outputOnce.Do(func() { close(s.outputChan) })
}

type portWriter struct {
	port goserial.Port
}

func (w *portWriter) Write(p []byte) (int, error) {
	if w == nil || w.port == nil {
		return 0, io.ErrClosedPipe
	}
	return w.port.Write(p)
}

func (w *portWriter) Close() error { return nil }
