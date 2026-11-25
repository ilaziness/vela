package term

import "github.com/go-gui-org/go-gui/gui"

// Stream is a byte transport that replaces a local PTY. Embedders use it to
// drive Term from SSH, serial, or any other remote session.
//
// The method set matches the internal ptyIO contract used by New and NewReplay.
type Stream interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	// Resize takes rows, cols, then the text-area size in device pixels
	// (width, height). Pixels may be 0 until the first frame measures a cell.
	Resize(rows, cols, xpx, ypx int) error
	Close() error
	PID() int
}

// NewWithStream builds a Term around s instead of spawning a local shell.
// Cfg.Command, Args, Env, and Dir are ignored: no child process exists.
func NewWithStream(w *gui.Window, cfg Cfg, s Stream) (*Term, error) {
	if s == nil {
		return nil, errNilStream
	}
	return newWithPTY(w, cfg, s)
}

type nilStreamError struct{}

func (nilStreamError) Error() string { return "term: NewWithStream: nil Stream" }

var errNilStream error = nilStreamError{}
