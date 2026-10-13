package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/things-go/go-socks5"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/buildinfo"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/internal/utils"
)

const (
	TypeLocal   = "local"
	TypeRemote  = "remote"
	TypeDynamic = "dynamic"
)

var ErrSessionNotFound = errors.New("ssh connect not found")

type TunnelList struct {
	TunnelType string       `json:"tunnelType"`
	Tunnels    []TunnelInfo `json:"tunnels"`
}

type TunnelInfo struct {
	ID         string `json:"id"`
	TunnelType string `json:"tunnelType"`
	SessionID  string `json:"sessionID"`
	LocalAddr  string `json:"localAddr"`
	RemoteAddr string `json:"remoteAddr"`
}

type tunnelConn struct {
	ID          string
	tunnelType  string
	sessionID   string
	LocalAddr   string
	RemoteAddr  string
	listener    net.Listener
	exitCh      chan struct{}
	closeOnce   sync.Once
	wg          sync.WaitGroup
	connLock    sync.Mutex
	activeConns map[net.Conn]struct{}
	socksServer *socks5.Server
}

type Manager struct {
	logger  *zap.Logger
	ssh     *ssh.Manager
	tunnels sync.Map
}

func NewManager(logger *zap.Logger, sshMgr *ssh.Manager) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{logger: logger, ssh: sshMgr}
}

func (t *Manager) List() []TunnelList {
	byType := make(map[string][]TunnelInfo)
	t.tunnels.Range(func(_, value any) bool {
		tn := value.(*tunnelConn)
		byType[tn.tunnelType] = append(byType[tn.tunnelType], TunnelInfo{
			ID: tn.ID, TunnelType: tn.tunnelType, SessionID: tn.sessionID,
			LocalAddr: tn.LocalAddr, RemoteAddr: tn.RemoteAddr,
		})
		return true
	})
	result := make([]TunnelList, 0, len(byType))
	for typ, items := range byType {
		result = append(result, TunnelList{TunnelType: typ, Tunnels: items})
	}
	return result
}

func (t *Manager) StartLocal(sessionID, localAddr, remoteAddr string) (string, error) {
	sc, err := t.ssh.GetClient(sessionID)
	if err != nil {
		return "", ErrSessionNotFound
	}
	ln, err := net.Listen("tcp", localAddr)
	if err != nil {
		return "", err
	}
	tn := &tunnelConn{
		ID: utils.GenerateRandomID(), tunnelType: TypeLocal, sessionID: sessionID,
		LocalAddr: localAddr, RemoteAddr: remoteAddr, listener: ln, exitCh: make(chan struct{}),
	}
	t.tunnels.Store(tn.ID, tn)
	tn.wg.Add(1)
	system.SafeGo(func() {
		defer tn.wg.Done()
		for {
			localConn, err := ln.Accept()
			if err != nil {
				t.logger.Debug("accept loop quit", zap.String("tunnelID", tn.ID), zap.Error(err))
				return
			}
			tn.wg.Add(1)
			system.SafeGo(func() {
				defer tn.wg.Done()
				defer localConn.Close()
				remoteConn, err := sc.Dial("tcp", tn.RemoteAddr)
				if err != nil {
					t.logger.Debug("failed to dial remote", zap.String("tunnelID", tn.ID), zap.Error(err))
					return
				}
				defer remoteConn.Close()
				pipe(tn, localConn, remoteConn)
			})
		}
	})
	return tn.ID, nil
}

func (t *Manager) StartRemote(sessionID string, remotePort int, localAddr string) (string, error) {
	sc, err := t.ssh.GetClient(sessionID)
	if err != nil {
		return "", ErrSessionNotFound
	}
	remoteAddr := fmt.Sprintf("127.0.0.1:%d", remotePort)
	ln, err := sc.Listen("tcp", remoteAddr)
	if err != nil {
		return "", err
	}
	tn := &tunnelConn{
		ID: utils.GenerateRandomID(), tunnelType: TypeRemote, sessionID: sessionID,
		LocalAddr: localAddr, RemoteAddr: remoteAddr, listener: ln, exitCh: make(chan struct{}),
	}
	t.tunnels.Store(tn.ID, tn)
	tn.wg.Add(1)
	system.SafeGo(func() {
		defer tn.wg.Done()
		for {
			remoteConn, err := ln.Accept()
			if err != nil {
				t.logger.Debug("remote accept loop quit", zap.String("tunnelID", tn.ID), zap.Error(err))
				return
			}
			tn.wg.Add(1)
			system.SafeGo(func() {
				defer tn.wg.Done()
				defer remoteConn.Close()
				localConn, err := net.Dial("tcp", tn.LocalAddr)
				if err != nil {
					t.logger.Debug("failed to dial local", zap.String("tunnelID", tn.ID), zap.Error(err))
					return
				}
				defer localConn.Close()
				pipe(tn, localConn, remoteConn)
			})
		}
	})
	return tn.ID, nil
}

func (t *Manager) StartDynamic(sessionID, localAddr string) (string, error) {
	sc, err := t.ssh.GetClient(sessionID)
	if err != nil {
		return "", ErrSessionNotFound
	}
	ln, err := net.Listen("tcp", localAddr)
	if err != nil {
		return "", err
	}
	tn := &tunnelConn{
		ID: utils.GenerateRandomID(), tunnelType: TypeDynamic, sessionID: sessionID,
		LocalAddr: localAddr, listener: ln, exitCh: make(chan struct{}),
		activeConns: make(map[net.Conn]struct{}),
	}
	t.tunnels.Store(tn.ID, tn)
	opts := []socks5.Option{
		socks5.WithDial(func(ctx context.Context, network, address string) (net.Conn, error) {
			to, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			return sc.DialContext(to, network, address)
		}),
	}
	if !buildinfo.IsRelease() {
		opts = append(opts, socks5.WithLogger(socks5.NewLogger(log.New(os.Stdout, "socks5: ", log.LstdFlags))))
	}
	tn.socksServer = socks5.NewServer(opts...)
	tn.wg.Add(1)
	system.SafeGo(func() {
		defer tn.wg.Done()
		t.handleDynamic(tn, ln)
	})
	return tn.ID, nil
}

func (t *Manager) handleDynamic(tn *tunnelConn, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			tn.connLock.Lock()
			for c := range tn.activeConns {
				_ = c.Close()
			}
			tn.connLock.Unlock()
			t.logger.Debug("socks5 accept loop quit", zap.Error(err))
			return
		}
		tn.connLock.Lock()
		tn.activeConns[conn] = struct{}{}
		tn.connLock.Unlock()
		tn.wg.Add(1)
		system.SafeGo(func() {
			defer tn.wg.Done()
			defer func() {
				conn.Close()
				tn.connLock.Lock()
				delete(tn.activeConns, conn)
				tn.connLock.Unlock()
			}()
			_ = tn.socksServer.ServeConn(conn)
		})
	}
}

func (tn *tunnelConn) signalStop() {
	tn.closeOnce.Do(func() {
		if tn.listener != nil {
			_ = tn.listener.Close()
		}
		close(tn.exitCh)
	})
}

func (tn *tunnelConn) waitStop() {
	tn.wg.Wait()
}

func (tn *tunnelConn) stop() {
	tn.signalStop()
	tn.waitStop()
}

func (t *Manager) StopByID(id string) error {
	v, ok := t.tunnels.Load(id)
	if !ok {
		return fmt.Errorf("tunnel not found")
	}
	tn := v.(*tunnelConn)
	tn.stop()
	t.tunnels.Delete(id)
	return nil
}

func (t *Manager) StopAllAfter(afterSignal func()) {
	var stopping []*tunnelConn
	t.tunnels.Range(func(key, value any) bool {
		tn := value.(*tunnelConn)
		tn.signalStop()
		t.tunnels.Delete(key)
		stopping = append(stopping, tn)
		return true
	})
	if afterSignal != nil {
		afterSignal()
	}
	for _, tn := range stopping {
		tn.waitStop()
	}
}

func (t *Manager) StopAll() {
	t.StopAllAfter(nil)
}

func (t *Manager) signalStopBySession(sessionID string) []*tunnelConn {
	var stopping []*tunnelConn
	t.tunnels.Range(func(key, value any) bool {
		tn := value.(*tunnelConn)
		if tn.sessionID == sessionID {
			tn.signalStop()
			t.tunnels.Delete(key)
			stopping = append(stopping, tn)
		}
		return true
	})
	return stopping
}

func (t *Manager) StopSessionAfter(sessionID string, afterSignal func()) {
	stopping := t.signalStopBySession(sessionID)
	if afterSignal != nil {
		afterSignal()
	}
	for _, tn := range stopping {
		tn.waitStop()
	}
}

func (t *Manager) StopAllBySession(sessionID string) {
	t.StopSessionAfter(sessionID, nil)
}

func pipe(tn *tunnelConn, a, b net.Conn) {
	exitCh := make(chan struct{})
	var once sync.Once
	closeCh := func() { once.Do(func() { close(exitCh) }) }
	tn.wg.Add(2)
	system.SafeGo(func() { defer tn.wg.Done(); _, _ = io.Copy(b, a); closeCh() })
	system.SafeGo(func() { defer tn.wg.Done(); _, _ = io.Copy(a, b); closeCh() })
	select {
	case <-exitCh:
	case <-tn.exitCh:
		a.Close()
		b.Close()
		<-exitCh
	}
}
