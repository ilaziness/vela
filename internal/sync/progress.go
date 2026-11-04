package sync

import "sync"

type Progress struct {
	Stage       string  `json:"stage"`
	Progress    float64 `json:"progress"`
	TotalBytes  int64   `json:"totalBytes"`
	DoneBytes   int64   `json:"doneBytes"`
	Speed       float64 `json:"speed"`
	ETA         int     `json:"eta"`
	IsCompleted bool    `json:"isCompleted"`
	Error       string  `json:"error"`
}

type ProgressReporter struct {
	mu       sync.RWMutex
	progress Progress
	onUpdate func(Progress)
}

func NewProgressReporter(onUpdate func(Progress)) *ProgressReporter {
	return &ProgressReporter{onUpdate: onUpdate}
}

func (p *ProgressReporter) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress = Progress{}
}

func (p *ProgressReporter) UpdateProgress(stage string, done, total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress.Stage = stage
	p.progress.DoneBytes = done
	p.progress.TotalBytes = total
	if total > 0 {
		p.progress.Progress = float64(done) / float64(total) * 100
	}
	if p.onUpdate != nil {
		p.onUpdate(p.progress)
	}
}

func (p *ProgressReporter) SetStage(stage string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress.Stage = stage
	if p.onUpdate != nil {
		p.onUpdate(p.progress)
	}
}

func (p *ProgressReporter) SetCompleted() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress.IsCompleted = true
	p.progress.Progress = 100
	if p.onUpdate != nil {
		p.onUpdate(p.progress)
	}
}

func (p *ProgressReporter) SetError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress.Error = err
	if p.onUpdate != nil {
		p.onUpdate(p.progress)
	}
}

func (p *ProgressReporter) GetProgress() Progress {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.progress
}
