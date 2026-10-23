package ssh

import (
	"sync"
	"time"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
)

const serverAliveCountMax = 3

type keepAliveCtrl struct {
	stop chan struct{}
	once sync.Once
}

func (k *keepAliveCtrl) Stop() {
	if k == nil {
		return
	}
	k.once.Do(func() { close(k.stop) })
}

func (m *Manager) startKeepAlive(clientKey string, client *cryptossh.Client) {
	interval := m.aliveInterval()
	if interval <= 0 || client == nil {
		return
	}
	ctrl := &keepAliveCtrl{stop: make(chan struct{})}
	if old, loaded := m.keepAlives.LoadOrStore(clientKey, ctrl); loaded {
		old.(*keepAliveCtrl).Stop()
		m.keepAlives.Store(clientKey, ctrl)
	}
	go m.runKeepAlive(clientKey, client, ctrl, interval)
}

func (m *Manager) stopKeepAlive(clientKey string) {
	if v, ok := m.keepAlives.LoadAndDelete(clientKey); ok {
		v.(*keepAliveCtrl).Stop()
	}
}

func (m *Manager) runKeepAlive(clientKey string, client *cryptossh.Client, ctrl *keepAliveCtrl, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctrl.stop:
			return
		case <-ticker.C:
			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				failures++
				m.logger.Debug("SSH keepalive failed",
					zap.String("clientKey", clientKey),
					zap.Int("failures", failures),
					zap.Error(err),
				)
				if failures >= serverAliveCountMax {
					m.logger.Warn("SSH keepalive exceeded max failures, closing client",
						zap.String("clientKey", clientKey),
					)
					m.dropClient(clientKey)
					return
				}
				continue
			}
			failures = 0
		}
	}
}

// dropClient tears down a cached client after keepalive failure.
// Session close is driven by PTY Wait once the TCP/SSH client dies.
func (m *Manager) dropClient(clientKey string) {
	m.stopKeepAlive(clientKey)
	lock := m.clientLock(clientKey)
	lock.Lock()
	defer lock.Unlock()
	if client, ok := m.clients.LoadAndDelete(clientKey); ok {
		client.(*hopClient).Close()
	}
	m.clearHostMetrics(clientKey)
}
