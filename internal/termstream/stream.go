// Package termstream adapts a termio.Session to go-term's Stream transport.
package termstream

import (
	"io"
	"sync"

	"github.com/ilaziness/vela/internal/termio"
)

const (
	seedCols = 80
	seedRows = 24
)

// Stream bridges a protocol-agnostic termio.Session to go-term.
type Stream struct {
	sess termio.Session

	startOnce sync.Once
	startErr  error

	mu     sync.Mutex
	buf    []byte
	closed bool
}

// New wraps sess. sess must not be nil.
func New(sess termio.Session) *Stream {
	return &Stream{sess: sess}
}

func (s *Stream) ensureStarted(cols, rows int) error {
	s.startOnce.Do(func() {
		if cols < 1 {
			cols = seedCols
		}
		if rows < 1 {
			rows = seedRows
		}
		s.startErr = s.sess.Start(cols, rows)
	})
	return s.startErr
}

// Read blocks on session output. Returns io.EOF when the session ends.
func (s *Stream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := s.ensureStarted(seedCols, seedRows); err != nil {
		return 0, err
	}

	s.mu.Lock()
	if len(s.buf) > 0 {
		n := copy(p, s.buf)
		s.buf = s.buf[n:]
		s.mu.Unlock()
		return n, nil
	}
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return 0, io.EOF
	}

	ch := s.sess.Output()
	if ch == nil {
		return 0, io.EOF
	}
	data, ok := <-ch
	if !ok {
		return 0, io.EOF
	}
	if len(data) == 0 {
		return 0, nil
	}
	n := copy(p, data)
	if n < len(data) {
		s.mu.Lock()
		s.buf = append(s.buf[:0], data[n:]...)
		s.mu.Unlock()
	}
	return n, nil
}

// Write sends bytes to the remote session stdin.
func (s *Stream) Write(p []byte) (int, error) {
	if err := s.ensureStarted(seedCols, seedRows); err != nil {
		return 0, err
	}
	w := s.sess.Stdin()
	if w == nil {
		return 0, io.ErrClosedPipe
	}
	return w.Write(p)
}

// Resize forwards geometry; rows/cols follow go-term order.
func (s *Stream) Resize(rows, cols, _, _ int) error {
	if err := s.ensureStarted(cols, rows); err != nil {
		return err
	}
	return s.sess.Resize(cols, rows)
}

// Close ends the underlying session (idempotent at the engine).
func (s *Stream) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	if s.sess == nil {
		return nil
	}
	return s.sess.Close()
}

// PID reports no local child process.
func (s *Stream) PID() int { return 0 }
