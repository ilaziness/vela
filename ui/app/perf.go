package app

import (
	"context"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/ssh"
)

const (
	// perfPollInterval matches the legacy bottom-panel cadence (5s);
	// FetchHostResources dedupes behind a 3s TTL cache.
	perfPollInterval = 5 * time.Second
	// perfHistoryMax is the rolling window: 24 points ≈ 2 minutes, which
	// keeps the curve readable in the narrow side panel.
	perfHistoryMax = 24
)

// PerfSample is one landed host-metrics point, shaped for the UI.
// Engine types (internal/ssh) never leave this package: ui/session
// only sees these plain fields.
type PerfSample struct {
	At         time.Time
	Hostname   string
	CPUPercent float64
	MemUsed    int64
	MemTotal   int64
	DiskUsed   int64
	DiskTotal  int64
	Load1      float64
	LatencyMs  int64
	Net        []PerfIface
	DiskIO     []PerfDisk
}

// PerfIface is one NIC's receive/transmit throughput in bytes per second.
type PerfIface struct {
	Name  string
	RxBps int64
	TxBps int64
}

// PerfDisk is one disk's read/write throughput in bytes per second.
type PerfDisk struct {
	Name     string
	ReadBps  int64
	WriteBps int64
}

// toPerfSample copies the UI-relevant fields out of the engine probe.
func toPerfSample(at time.Time, res ssh.HostResources) PerfSample {
	s := PerfSample{
		At:         at,
		Hostname:   res.Hostname,
		CPUPercent: res.CPUPercent,
		MemUsed:    res.MemUsed,
		MemTotal:   res.MemTotal,
		DiskUsed:   res.DiskUsed,
		DiskTotal:  res.DiskTotal,
		Load1:      res.Load1,
		LatencyMs:  res.LatencyMs,
	}
	for _, f := range res.NetIfaces {
		s.Net = append(s.Net, PerfIface{Name: f.Name, RxBps: f.RxBps, TxBps: f.TxBps})
	}
	for _, d := range res.DiskIO {
		s.DiskIO = append(s.DiskIO, PerfDisk{Name: d.Name, ReadBps: d.ReadBps, WriteBps: d.WriteBps})
	}
	return s
}

// perfMon owns the rolling history for one live session. The entry stays
// in Controller.perf after the panel closes (polling halted, history
// retained) so reopening the panel shows the past curve.
type perfMon struct {
	mu      sync.Mutex
	hist    []PerfSample
	lastErr string // localized; "" when healthy
	// stop is non-nil while a poller runs and identifies that poller,
	// so a goroutine that is still winding down can tell whether it
	// still owns the slot.
	stop chan struct{}
}

func (p *perfMon) append(s PerfSample) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hist = append(p.hist, s)
	if len(p.hist) > perfHistoryMax {
		p.hist = p.hist[len(p.hist)-perfHistoryMax:]
	}
}

func (p *perfMon) setError(msg string) {
	p.mu.Lock()
	p.lastErr = msg
	p.mu.Unlock()
}

func (p *perfMon) clearError() {
	p.mu.Lock()
	p.lastErr = ""
	p.mu.Unlock()
}

// halt stops a running poller once. The map entry (and history) survives.
func (p *perfMon) halt() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
}

func (p *perfMon) snapshot() ([]PerfSample, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]PerfSample, len(p.hist))
	copy(out, p.hist)
	return out, p.lastErr
}

// StartPerf starts (or resumes) host-metrics polling for a session.
// Idempotent: a running poller is left alone and its history kept.
func (c *Controller) StartPerf(w *gui.Window, sessID string) {
	if sessID == "" {
		return
	}
	c.mu.Lock()
	if c.perf == nil {
		c.perf = make(map[string]*perfMon)
	}
	pm := c.perf[sessID]
	if pm == nil {
		pm = &perfMon{}
		c.perf[sessID] = pm
	}
	mgr := c.sshMgr
	logger := c.logger
	c.mu.Unlock()

	pm.mu.Lock()
	if pm.stop != nil {
		// Already polling: keep the history and leave it alone.
		pm.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	pm.stop = stop
	pm.mu.Unlock()

	go c.runPerfPoll(w, mgr, sessID, pm, stop, logger)
}

// StopPerf halts polling for a session but keeps its history, so a
// reopened panel still shows the past curve.
func (c *Controller) StopPerf(sessID string) {
	if sessID == "" {
		return
	}
	c.mu.Lock()
	pm := c.perf[sessID]
	c.mu.Unlock()
	if pm != nil {
		pm.halt()
	}
}

// DropPerf discards a session's perf state entirely (tab closed,
// session dropped, app shutting down).
func (c *Controller) DropPerf(sessID string) {
	if sessID == "" {
		return
	}
	c.mu.Lock()
	pm := c.perf[sessID]
	delete(c.perf, sessID)
	c.mu.Unlock()
	if pm != nil {
		pm.halt()
	}
}

// DropAllPerf discards every session's perf state (shutdown path).
func (c *Controller) DropAllPerf() {
	c.mu.Lock()
	pms := make([]*perfMon, 0, len(c.perf))
	for id, pm := range c.perf {
		pms = append(pms, pm)
		delete(c.perf, id)
	}
	c.mu.Unlock()
	for _, pm := range pms {
		pm.halt()
	}
}

// PerfSnapshot returns a copy of the session's history and the last
// fetch error (localized; "" when healthy). Call from view build only.
func (c *Controller) PerfSnapshot(sessID string) ([]PerfSample, string) {
	if sessID == "" {
		return nil, ""
	}
	c.mu.Lock()
	pm := c.perf[sessID]
	c.mu.Unlock()
	if pm == nil {
		return nil, ""
	}
	return pm.snapshot()
}

// runPerfPoll samples immediately, then on a ticker until halted or
// the host reports metrics as unsupported.
func (c *Controller) runPerfPoll(
	w *gui.Window,
	mgr *ssh.Manager,
	sessID string,
	pm *perfMon,
	stop chan struct{},
	logger *zap.Logger,
) {
	defer func() {
		// Release the slot only if this poller still owns it: a halt
		// followed by a quick restart hands the slot to a new
		// goroutine, and this one must not clear that one's stop.
		pm.mu.Lock()
		if pm.stop == stop {
			pm.stop = nil
		}
		pm.mu.Unlock()
	}()

	fetch := func() bool { // false → stop polling
		res, err := mgr.FetchHostResources(context.Background(), sessID)
		if err != nil {
			logger.Debug("perf poll failed",
				zap.String("sessID", sessID), zap.Error(err))
			pm.setError(errText(err))
			return true
		}
		if !res.Supported {
			// Non-Linux target or a probe that found nothing: stop
			// polling and surface the reason in the panel.
			pm.setError(i18n.T("shell.perf.unsupported"))
			return false
		}
		pm.clearError()
		pm.append(toPerfSample(time.Now(), res))
		// Rebuild the view on the next frame so the charts pick up the
		// new history. InvalidateLayout is goroutine-safe and is the
		// documented "layout is stale" signal.
		w.InvalidateLayout()
		return true
	}

	if !fetch() {
		return
	}
	ticker := time.NewTicker(perfPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !fetch() {
				return
			}
		}
	}
}
