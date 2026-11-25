package transfer

import (
	"context"
	"errors"
	"io"
	"math"
	"sync"
	"time"

	"github.com/ilaziness/vela/internal/utils"
)

const (
	TypeUpload   = "upload"
	TypeDownload = "download"
)

type ProgressData struct {
	ID           string  `json:"id"`
	SessionID    string  `json:"sessionID"`
	OwnerKey     string  `json:"ownerKey"`
	TransferType string  `json:"transferType"`
	LocalFile    string  `json:"localFile"`
	RemoteFile   string  `json:"remoteFile"`
	TotalSize    int64   `json:"totalSize"`
	Transferred  int64   `json:"transferred"`
	Rate         float64 `json:"rate"`
	Done         bool    `json:"done"`
	Error        string  `json:"error"`
}

// NewOpts configures optional tracker fields.
type NewOpts struct {
	ID        string
	OwnerKey  string
	Overwrite bool
}

type Registry struct {
	mu       sync.Mutex
	active   map[string]*Tracker
	onUpdate func(ProgressData)
}

func NewRegistry(onUpdate func(ProgressData)) *Registry {
	return &Registry{
		active:   make(map[string]*Tracker),
		onUpdate: onUpdate,
	}
}

func (r *Registry) New(sessionID, transferType, localFile, remoteFile string, total int64) (*Tracker, error) {
	return r.NewWith(sessionID, transferType, localFile, remoteFile, total, NewOpts{})
}

func (r *Registry) HasActive(id string) bool {
	if id == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.active[id]
	return ok
}

func (r *Registry) NewWith(sessionID, transferType, localFile, remoteFile string, total int64, opts NewOpts) (*Tracker, error) {
	id := opts.ID
	if id == "" {
		id = utils.GenerateRandomID()
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tracker{
		reg:          r,
		sessionID:    sessionID,
		ownerKey:     opts.OwnerKey,
		id:           id,
		transferType: transferType,
		localFile:    localFile,
		remoteFile:   remoteFile,
		total:        total,
		overwrite:    opts.Overwrite,
		done:         make(chan struct{}),
		ctx:          ctx,
		cancelFunc:   cancel,
	}
	r.mu.Lock()
	if _, exists := r.active[id]; exists {
		r.mu.Unlock()
		cancel()
		return nil, errors.New("transfer already active")
	}
	r.active[t.id] = t
	r.mu.Unlock()
	t.startProgress()
	return t, nil
}

func (r *Registry) Cancel(id string) error {
	r.mu.Lock()
	t, ok := r.active[id]
	r.mu.Unlock()
	if !ok {
		return errors.New("transfer not found")
	}
	t.markCancelled()
	t.cancelFunc()
	return nil
}

// InterruptSession cancels all active transfers for sessionID and marks them interrupted.
func (r *Registry) InterruptSession(sessionID string) []string {
	r.mu.Lock()
	var ids []string
	var trackers []*Tracker
	for id, t := range r.active {
		if t.sessionID == sessionID {
			ids = append(ids, id)
			trackers = append(trackers, t)
		}
	}
	r.mu.Unlock()
	for _, t := range trackers {
		t.markInterrupted()
		t.cancelFunc()
	}
	return ids
}

// InterruptAll cancels every active transfer (app shutdown).
func (r *Registry) InterruptAll() []string {
	r.mu.Lock()
	ids := make([]string, 0, len(r.active))
	trackers := make([]*Tracker, 0, len(r.active))
	for id, t := range r.active {
		ids = append(ids, id)
		trackers = append(trackers, t)
	}
	r.mu.Unlock()
	for _, t := range trackers {
		t.markInterrupted()
		t.cancelFunc()
	}
	return ids
}

func (r *Registry) emit(p ProgressData) {
	if r.onUpdate != nil {
		r.onUpdate(p)
	}
}

func (r *Registry) remove(id string) {
	r.mu.Lock()
	delete(r.active, id)
	r.mu.Unlock()
}

type Tracker struct {
	reg          *Registry
	sessionID    string
	ownerKey     string
	id           string
	transferType string
	localFile    string
	remoteFile   string
	total        int64
	transferred  int64
	mutex        sync.Mutex
	ticker       *time.Ticker
	done         chan struct{}
	stopOnce     sync.Once
	ctx          context.Context
	cancelFunc   context.CancelFunc
	cancelled    bool
	interrupted  bool
	overwrite    bool
	err          error
}

func (t *Tracker) ID() string           { return t.id }
func (t *Tracker) OwnerKey() string     { return t.ownerKey }
func (t *Tracker) TransferType() string { return t.transferType }
func (t *Tracker) LocalFile() string    { return t.localFile }
func (t *Tracker) RemoteFile() string   { return t.remoteFile }
func (t *Tracker) TotalSize() int64     { return t.total }
func (t *Tracker) Overwrite() bool      { return t.overwrite }

// Wait blocks until Stop and returns the error recorded there.
func (t *Tracker) Wait() error {
	<-t.done
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.err
}

func (t *Tracker) WasCancelled() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.cancelled
}

func (t *Tracker) WasInterrupted() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.interrupted
}

func (t *Tracker) markCancelled() {
	t.mutex.Lock()
	t.cancelled = true
	t.mutex.Unlock()
}

func (t *Tracker) markInterrupted() {
	t.mutex.Lock()
	t.interrupted = true
	t.mutex.Unlock()
}

func (t *Tracker) AddTransferred(n int64) {
	if n <= 0 {
		return
	}
	t.update(n)
}

func (t *Tracker) update(n int64) {
	t.mutex.Lock()
	t.transferred += n
	t.mutex.Unlock()
}

func (t *Tracker) getRate() float64 {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.total == 0 {
		return 100.0
	}
	rate := float64(t.transferred) * 100.0 / float64(t.total)
	if rate > 100.0 {
		rate = 100.0
	}
	return math.Round(rate*100) / 100
}

func (t *Tracker) data(rate float64, done bool, err error) ProgressData {
	t.mutex.Lock()
	transferred := t.transferred
	t.mutex.Unlock()
	p := ProgressData{
		ID:           t.id,
		SessionID:    t.sessionID,
		OwnerKey:     t.ownerKey,
		TransferType: t.transferType,
		LocalFile:    t.localFile,
		RemoteFile:   t.remoteFile,
		TotalSize:    t.total,
		Transferred:  transferred,
		Rate:         rate,
		Done:         done,
	}
	if err != nil {
		p.Error = err.Error()
	}
	return p
}

func (t *Tracker) startProgress() {
	t.reg.emit(t.data(t.getRate(), false, nil))
	t.ticker = time.NewTicker(500 * time.Millisecond)
	go func() {
		defer t.ticker.Stop()
		for {
			select {
			case <-t.ticker.C:
				t.reg.emit(t.data(t.getRate(), false, nil))
			case <-t.done:
				return
			}
		}
	}()
}

func (t *Tracker) Stop(err error) {
	t.stopOnce.Do(func() {
		t.mutex.Lock()
		t.err = err
		t.mutex.Unlock()
		if t.ticker != nil {
			t.ticker.Stop()
		}
		close(t.done)
		t.reg.emit(t.data(t.getRate(), true, err))
		t.reg.remove(t.id)
	})
}

type Reader struct {
	Reader  io.Reader
	Tracker *Tracker
}

func (pr *Reader) Read(p []byte) (int, error) {
	select {
	case <-pr.Tracker.ctx.Done():
		if errors.Is(pr.Tracker.ctx.Err(), context.Canceled) {
			return 0, errors.New("user cancelled")
		}
		return 0, pr.Tracker.ctx.Err()
	default:
	}
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Tracker.update(int64(n))
	}
	return n, err
}

type Writer struct {
	Writer  io.Writer
	Tracker *Tracker
}

func (pw *Writer) Write(p []byte) (int, error) {
	select {
	case <-pw.Tracker.ctx.Done():
		if errors.Is(pw.Tracker.ctx.Err(), context.Canceled) {
			return 0, errors.New("user cancelled")
		}
		return 0, pw.Tracker.ctx.Err()
	default:
	}
	n, err := pw.Writer.Write(p)
	if n > 0 {
		pw.Tracker.update(int64(n))
	}
	return n, err
}
