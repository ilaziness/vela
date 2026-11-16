package updater

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/httpproxy"
)

// Source is an update provider. Implementations return nil,nil when the
// current version is already up to date.
type Source interface {
	Name() string
	Check(ctx context.Context, currentVersion string) (*Release, error)
}

// Asset is the platform package to download for a release.
type Asset struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string
}

// Release is a newer app version found by a Source.
type Release struct {
	Source  string
	Tag     string
	Version string
	Notes   string
	URL     string
	Asset   Asset
}

// Phase identifies the install stage shown in the UI.
type Phase string

const (
	PhaseIdle     Phase = ""
	PhaseDownload Phase = "download"
	PhaseVerify   Phase = "verify"
	PhaseInstall  Phase = "install"
	PhaseDone     Phase = "done"
)

// InstallStatus is a snapshot of the in-flight install for UI polling.
type InstallStatus struct {
	Phase   Phase
	Written int64
	Total   int64
	Done    bool
	Err     error
}

// Busy reports whether an install is currently running.
func (s InstallStatus) Busy() bool {
	return s.Phase == PhaseDownload || s.Phase == PhaseVerify || s.Phase == PhaseInstall
}

// Manager owns update sources, the cached latest release and install state.
type Manager struct {
	logger    *zap.Logger
	current   string
	sources   []Source
	http      *http.Client
	installMu sync.Mutex
	status    InstallStatus
	cancel    context.CancelFunc

	checkMu  sync.Mutex
	inflight *checkCall
	latestMu sync.RWMutex
	latest   *Release
}

type checkCall struct {
	done chan struct{}
	rel  *Release
	err  error
}

// NewManager builds the process updater. logger may be nil.
func NewManager(logger *zap.Logger, currentVersion string, sources ...Source) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{logger: logger, current: currentVersion, sources: sources}
}

// SetHTTPClient overrides the client used for downloads. Tests use it to
// point at httptest servers; nil keeps httpproxy.Client().
func (m *Manager) SetHTTPClient(c *http.Client) { m.http = c }

func (m *Manager) client() *http.Client {
	if m.http != nil {
		return m.http
	}
	return httpproxy.Client()
}

// Check asks every source for updates and caches the newest release.
// Concurrent callers share one in-flight check.
func (m *Manager) Check(ctx context.Context) (*Release, error) {
	m.checkMu.Lock()
	if m.inflight != nil {
		call := m.inflight
		m.checkMu.Unlock()
		select {
		case <-call.done:
			return call.rel, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &checkCall{done: make(chan struct{})}
	m.inflight = call
	m.checkMu.Unlock()

	rel, err := m.check(ctx)
	call.rel, call.err = rel, err
	close(call.done)

	m.checkMu.Lock()
	m.inflight = nil
	m.checkMu.Unlock()
	return rel, err
}

func (m *Manager) check(ctx context.Context) (*Release, error) {
	var best *Release
	var firstErr error
	for _, src := range m.sources {
		if src == nil {
			continue
		}
		rel, err := src.Check(ctx, m.current)
		if err != nil {
			m.logger.Warn("update source check failed",
				zap.String("source", src.Name()), zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if rel == nil {
			continue
		}
		if best == nil || compareVersions(rel.Version, best.Version) > 0 {
			best = rel
		}
	}
	if best == nil && firstErr != nil {
		return nil, firstErr
	}
	m.latestMu.Lock()
	m.latest = best
	m.latestMu.Unlock()
	return best, nil
}

// Latest returns the cached newer release, or nil.
func (m *Manager) Latest() *Release {
	m.latestMu.RLock()
	defer m.latestMu.RUnlock()
	return m.latest
}

// InstallStatus returns a copy of the current install state.
func (m *Manager) InstallStatus() InstallStatus {
	m.installMu.Lock()
	defer m.installMu.Unlock()
	return m.status
}

func (m *Manager) setStatus(s InstallStatus) {
	m.installMu.Lock()
	m.status = s
	m.installMu.Unlock()
}

func (m *Manager) setCancel(cancel context.CancelFunc) {
	m.installMu.Lock()
	m.cancel = cancel
	m.installMu.Unlock()
}

// CancelInstall aborts an in-flight download. Verify/install phases are not cancellable.
func (m *Manager) CancelInstall() {
	m.installMu.Lock()
	cancel, phase := m.cancel, m.status.Phase
	m.installMu.Unlock()
	if phase == PhaseDownload && cancel != nil {
		cancel()
	}
}

var errUpdateInProgress = errors.New("update install already in progress")
