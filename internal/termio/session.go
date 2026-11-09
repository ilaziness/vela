package termio

import "io"

// Session is the thin terminal I/O contract shared by SSH and serial.
type Session interface {
	ID() string
	Protocol() Protocol
	Label() string
	Stdin() io.WriteCloser
	Output() <-chan []byte
	Start(cols, rows int) error
	Resize(cols, rows int) error
	Close() error
}

// Info is a Hub listing entry for RPC/UI.
type Info struct {
	ID       string   `json:"id"`
	Protocol Protocol `json:"protocol"`
	Label    string   `json:"label"`
}
