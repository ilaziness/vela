package zmodem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	zm "github.com/xx25/go-zmodem"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/system"
)

const protoBufMax = 4 << 20 // 4 MiB

// ErrCancelled is returned when the user dismisses a file/directory dialog.
var ErrCancelled = errors.New("zmodem: cancelled")

// FilePicker selects local paths for Zmodem transfers. Implemented in services.
type FilePicker interface {
	PickSaveDirectory(ctx context.Context, sessionID string) (string, error)
	PickOpenFile(ctx context.Context, sessionID string) (string, error)
}

// abortSeq is 8×CAN + 10×BS (ZMODEM spec).
var abortSeq = []byte{
	0x18, 0x18, 0x18, 0x18, 0x18, 0x18, 0x18, 0x18,
	0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08,
}

type filterState int

const (
	stateIdle filterState = iota
	statePending
	stateActive
)

// Deps wires Filter dependencies. Picker nil → FeedOut is a passthrough.
type Deps struct {
	Picker   FilePicker
	Logger   *zap.Logger
	Annotate func([]byte) // inject progress/status into the terminal
}

// Filter detects Zmodem in the PTY stream, splices protocol bytes away from the
// terminal, and runs send/receive over a duplex bridge to stdin.
type Filter struct {
	sessionID string
	picker    FilePicker
	logger    *zap.Logger
	annotate  func([]byte)

	mu         sync.Mutex
	state      filterState
	lookbehind []byte
	stdin      io.WriteCloser
	queue      *byteQueue
	cancel     context.CancelFunc
}

// NewFilter creates a session-scoped filter.
func NewFilter(sessionID string, deps Deps) *Filter {
	logger := deps.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Filter{
		sessionID: sessionID,
		picker:    deps.Picker,
		logger:    logger,
		annotate:  deps.Annotate,
	}
}

// BindStdin attaches the real SSH StdinPipe.
func (f *Filter) BindStdin(w io.WriteCloser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stdin = w
}

// WrapStdin returns a WriteCloser for termws / SendToSession.
// Idle: pass-through. Pending/active: drop user keystrokes.
func (f *Filter) WrapStdin() io.WriteCloser {
	return &userStdin{f: f}
}

// FeedOut processes a PTY output chunk. Must not block.
func (f *Filter) FeedOut(data []byte) []byte {
	if f == nil || f.picker == nil || len(data) == 0 {
		return data
	}

	f.mu.Lock()
	switch f.state {
	case stateIdle:
		echo := f.feedIdleLocked(data)
		f.mu.Unlock()
		return echo
	case statePending, stateActive:
		if f.queue == nil {
			f.mu.Unlock()
			return nil
		}
		err := f.queue.tryWrite(data)
		if err == nil || errors.Is(err, io.ErrClosedPipe) {
			f.mu.Unlock()
			return nil
		}
		f.logger.Warn("zmodem proto buffer full, aborting",
			zap.String("id", f.sessionID), zap.Error(err))
		w := f.abortLocked()
		f.mu.Unlock()
		writeAbort(w)
		f.emitLine("Zmodem: transfer aborted: buffer full")
		return nil
	}
	f.mu.Unlock()
	return data
}

func (f *Filter) feedIdleLocked(data []byte) []byte {
	combined := data
	lbLen := len(f.lookbehind)
	if lbLen > 0 {
		combined = append(append([]byte{}, f.lookbehind...), data...)
	}

	det, ok := findHexAutostart(combined)
	if !ok {
		f.lookbehind = trailingPartialAutostart(combined)
		return data
	}

	proto := combined[det.index:]
	var echo []byte
	if det.index >= lbLen {
		echo = data[:det.index-lbLen]
	}
	f.lookbehind = nil
	f.startTransferLocked(det.dir, proto)
	return echo
}

func (f *Filter) startTransferLocked(dir Direction, initial []byte) {
	f.queue = newByteQueue(protoBufMax)
	if err := f.queue.tryWrite(initial); err != nil {
		f.logger.Warn("zmodem failed to queue initial frame", zap.Error(err))
		f.queue = nil
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.state = statePending

	system.SafeGo(func() { f.runTransfer(ctx, dir) })
}

func (f *Filter) runTransfer(ctx context.Context, dir Direction) {
	defer f.finish()

	var (
		saveDir string
		offer   *zm.FileOffer
		err     error
	)

	switch dir {
	case DirectionReceive:
		f.emitLine("Zmodem: receiving — choose save directory…")
		saveDir, err = f.picker.PickSaveDirectory(ctx, f.sessionID)
		if err != nil {
			f.handlePickErr(err)
			return
		}
	case DirectionSend:
		f.emitLine("Zmodem: sending — choose a file…")
		var path string
		path, err = f.picker.PickOpenFile(ctx, f.sessionID)
		if err != nil {
			f.handlePickErr(err)
			return
		}
		offer, err = openFileOffer(path)
		if err != nil {
			f.emitLine(fmt.Sprintf("Zmodem: open file failed: %v", err))
			f.sendAbort()
			return
		}
		defer func() {
			if rc, ok := offer.Reader.(io.Closer); ok {
				_ = rc.Close()
			}
		}()
	}

	f.mu.Lock()
	if f.state != statePending {
		f.mu.Unlock()
		return
	}
	f.state = stateActive
	bridge := &duplexBridge{q: f.queue, w: f.stdin}
	f.mu.Unlock()

	handler := &fileHandler{
		f:       f,
		dir:     dir,
		saveDir: saveDir,
		offer:   offer,
	}
	sess := zm.NewSession(bridge, handler, &zm.Config{
		RecvTimeout:  0, // picker may take longer than the library default
		Use32BitCRC:  true,
		MaxBlockSize: 8192,
	})

	var runErr error
	switch dir {
	case DirectionReceive:
		runErr = sess.Receive(ctx)
	case DirectionSend:
		runErr = sess.Send(ctx)
	}
	if ctx.Err() != nil {
		return
	}
	if runErr != nil {
		f.logger.Warn("zmodem session ended with error",
			zap.String("id", f.sessionID), zap.Error(runErr))
		if !handler.completedOK {
			f.emitLine(fmt.Sprintf("Zmodem: failed: %v", runErr))
		}
		return
	}
	if !handler.completedOK {
		f.emitLine("Zmodem: done")
	}
}

func (f *Filter) handlePickErr(err error) {
	if errors.Is(err, ErrCancelled) {
		f.emitLine("Zmodem: cancelled")
	} else {
		f.emitLine(fmt.Sprintf("Zmodem: %v", err))
		f.logger.Warn("zmodem pick failed", zap.String("id", f.sessionID), zap.Error(err))
	}
	f.sendAbort()
}

func (f *Filter) sendAbort() {
	f.mu.Lock()
	w := f.stdin
	f.mu.Unlock()
	writeAbort(w)
}

func writeAbort(w io.Writer) {
	if w != nil {
		_, _ = w.Write(abortSeq)
	}
}

// Reset cancels any transfer and returns to idle.
func (f *Filter) Reset() {
	if f == nil {
		return
	}
	f.mu.Lock()
	w := f.abortLocked()
	f.mu.Unlock()
	writeAbort(w)
}

// abortLocked cancels transfer state. Caller must hold f.mu.
func (f *Filter) abortLocked() (stdin io.WriteCloser) {
	f.lookbehind = nil
	if f.state == stateIdle && f.cancel == nil && f.queue == nil {
		return nil
	}
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
	if f.queue != nil {
		f.queue.Close()
		f.queue = nil
	}
	w := f.stdin
	f.state = stateIdle
	return w
}

func (f *Filter) finish() {
	f.mu.Lock()
	_ = f.abortLocked()
	f.mu.Unlock()
}

func (f *Filter) emitLine(msg string) {
	if f.annotate == nil || msg == "" {
		return
	}
	f.annotate([]byte("\r\n" + msg + "\r\n"))
}

func (f *Filter) emitProgress(msg string) {
	if f.annotate == nil || msg == "" {
		return
	}
	// \r refresh + clear to end of line so shorter updates do not leave garbage.
	f.annotate([]byte("\r" + msg + "\033[K"))
}

type userStdin struct {
	f *Filter
}

func (u *userStdin) Write(p []byte) (int, error) {
	u.f.mu.Lock()
	if u.f.state != stateIdle {
		u.f.mu.Unlock()
		return len(p), nil
	}
	w := u.f.stdin
	u.f.mu.Unlock()
	if w == nil {
		return 0, io.ErrClosedPipe
	}
	return w.Write(p)
}

func (u *userStdin) Close() error {
	u.f.mu.Lock()
	w := u.f.stdin
	u.f.stdin = nil
	u.f.mu.Unlock()
	if w == nil {
		return nil
	}
	return w.Close()
}

type duplexBridge struct {
	q *byteQueue
	w io.Writer
}

func (b *duplexBridge) Read(p []byte) (int, error) {
	return b.q.Read(p)
}

func (b *duplexBridge) Write(p []byte) (int, error) {
	if b.w == nil {
		return 0, io.ErrClosedPipe
	}
	return b.w.Write(p)
}
