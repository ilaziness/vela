package services

import (
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/config"
)

const (
	windowStateDebounce = 400 * time.Millisecond
	windowStateQuiet    = time.Second // ignore event-driven saves during startup layout
)

// bindMainWindowState watches resize/maximise and returns a flush for WindowClosing.
func bindMainWindowState(win *application.WebviewWindow, store *config.Store, log *zap.Logger) (flush func()) {
	readyAt := time.Now().Add(windowStateQuiet)
	var mu sync.Mutex
	var timer *time.Timer

	flush = func() {
		mu.Lock()
		if timer != nil {
			timer.Stop()
			timer = nil
		}
		mu.Unlock()
		saveMainWindowState(win, store, log)
	}

	schedule := func() {
		if time.Now().Before(readyAt) {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(windowStateDebounce, func() {
			saveMainWindowState(win, store, log)
		})
	}

	// Capture a resize done during the quiet window once layout has settled.
	time.AfterFunc(windowStateQuiet, func() {
		saveMainWindowState(win, store, log)
	})

	win.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) {
		schedule()
	})
	win.OnWindowEvent(events.Common.WindowMaximise, func(*application.WindowEvent) {
		schedule()
	})
	win.OnWindowEvent(events.Common.WindowUnMaximise, func(*application.WindowEvent) {
		schedule()
	})
	return flush
}

func saveMainWindowState(win *application.WebviewWindow, store *config.Store, log *zap.Logger) {
	prev := store.Window()
	st := config.WindowGeometry{
		Width:     prev.Width,
		Height:    prev.Height,
		Maximised: win.IsMaximised() || win.IsFullscreen(),
	}
	if !st.Maximised {
		w, h := win.Size()
		if w <= 0 || h <= 0 {
			return
		}
		st.Width = w
		st.Height = h
	}
	if err := store.SaveWindow(st); err != nil {
		log.Debug("save window state failed", zap.Error(err))
	}
}
