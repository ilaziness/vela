package termcharset

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Canonical charset names stored on bookmarks / endpoints.
const (
	UTF8    = "utf-8"
	GBK     = "gbk"
	GB18030 = "gb18030"
)

// Normalize returns a canonical charset name. Empty becomes utf-8.
func Normalize(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", UTF8, "utf8":
		return UTF8, nil
	case GBK:
		return GBK, nil
	case GB18030:
		return GB18030, nil
	default:
		return "", fmt.Errorf("不支持的终端编码: %s", name)
	}
}

func encodingOf(name string) (encoding.Encoding, error) {
	n, err := Normalize(name)
	if err != nil {
		return nil, err
	}
	switch n {
	case GBK:
		return simplifiedchinese.GBK, nil
	case GB18030:
		return simplifiedchinese.GB18030, nil
	default:
		return nil, nil
	}
}

// Stream transforms chunks in one direction, retaining incomplete trailing bytes.
// Not safe for concurrent use; callers must not share one Stream across goroutines.
// Do not use transform.Bytes — it Reset()s the transformer each call.
type Stream struct {
	t   transform.Transformer
	dst []byte
	src []byte // held-over incomplete bytes from prior chunk
}

// NewDecoder converts remote charset bytes to UTF-8. UTF-8 → nil (passthrough).
func NewDecoder(name string) (*Stream, error) {
	enc, err := encodingOf(name)
	if err != nil {
		return nil, err
	}
	if enc == nil {
		return nil, nil
	}
	return &Stream{t: enc.NewDecoder()}, nil
}

// NewEncoder converts UTF-8 to remote charset. UTF-8 → nil (passthrough).
func NewEncoder(name string) (*Stream, error) {
	enc, err := encodingOf(name)
	if err != nil {
		return nil, err
	}
	if enc == nil {
		return nil, nil
	}
	return &Stream{t: enc.NewEncoder()}, nil
}

// Bytes transforms one chunk. Nil receiver returns in unchanged.
func (s *Stream) Bytes(in []byte) []byte {
	if s == nil || s.t == nil {
		return in
	}
	if len(s.src) > 0 {
		combined := make([]byte, len(s.src)+len(in))
		copy(combined, s.src)
		copy(combined[len(s.src):], in)
		in = combined
		s.src = s.src[:0]
	}
	if len(in) == 0 {
		return nil
	}
	return s.transform(in, false)
}

// Flush emits any held incomplete sequence as if at EOF. Nil-safe.
func (s *Stream) Flush() []byte {
	if s == nil || s.t == nil {
		return nil
	}
	in := s.src
	s.src = s.src[:0]
	if len(in) == 0 {
		// Still allow transformer to emit trailing state.
		return s.transform(nil, true)
	}
	return s.transform(in, true)
}

func (s *Stream) transform(in []byte, atEOF bool) []byte {
	if cap(s.dst) < len(in)*2+16 {
		s.dst = make([]byte, len(in)*2+16)
	}
	var out []byte
	for {
		nDst, nSrc, err := s.t.Transform(s.dst, in, atEOF)
		if nDst > 0 {
			out = append(out, s.dst[:nDst]...)
		}
		in = in[nSrc:]
		switch {
		case err == nil:
			return out
		case errors.Is(err, transform.ErrShortDst):
			s.dst = make([]byte, len(s.dst)*2)
			continue
		case errors.Is(err, transform.ErrShortSrc):
			if atEOF {
				// Cannot make progress; drop remainder.
				return out
			}
			if len(in) > 0 {
				s.src = append(s.src[:0], in...)
			}
			return out
		default:
			// Skip one bad byte and continue (keep session alive).
			if len(in) > 0 {
				in = in[1:]
				continue
			}
			return out
		}
	}
}

// WriteCloser wraps an io.WriteCloser, encoding UTF-8 writes to the remote charset.
// Writes are serialized so concurrent SendToSession / termws callers are safe.
type WriteCloser struct {
	mu  sync.Mutex
	w   io.WriteCloser
	enc *Stream
}

// WrapWriteCloser returns w unchanged when enc is nil.
func WrapWriteCloser(w io.WriteCloser, enc *Stream) io.WriteCloser {
	if w == nil || enc == nil {
		return w
	}
	return &WriteCloser{w: w, enc: enc}
}

func (c *WriteCloser) Write(p []byte) (int, error) {
	if c == nil {
		return 0, io.ErrClosedPipe
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.w == nil {
		return 0, io.ErrClosedPipe
	}
	out := c.enc.Bytes(p)
	if len(out) == 0 {
		return len(p), nil
	}
	_, err := c.w.Write(out)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *WriteCloser) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.w == nil {
		return nil
	}
	if flushed := c.enc.Flush(); len(flushed) > 0 {
		_, _ = c.w.Write(flushed)
	}
	err := c.w.Close()
	c.w = nil
	return err
}
