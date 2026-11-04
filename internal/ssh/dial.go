package ssh

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ilaziness/vela/internal/usererr"
)

func (m *Manager) dialHops(hops []Endpoint, timeout time.Duration, proxy ProxyConfig) (*hopClient, error) {
	const maxDepth = 5
	if len(hops) == 0 {
		return nil, usererr.Coded("ssh.empty_hops", "Empty hop list")
	}
	if len(hops) > maxDepth+1 {
		return nil, usererr.Coded("ssh.jump_too_deep", "Jump host chain exceeds the maximum of %s hops", strconv.Itoa(maxDepth))
	}
	seen := make(map[string]bool)
	var jumps []*cryptossh.Client
	var prev *cryptossh.Client
	closeOpened := func() {
		if prev != nil {
			_ = prev.Close()
		}
		for i := len(jumps) - 1; i >= 0; i-- {
			_ = jumps[i].Close()
		}
	}
	for i, hop := range hops {
		addr := hop.Addr()
		if seen[addr] {
			closeOpened()
			return nil, usererr.Coded("ssh.jump_cycle", "Jump host cycle detected: %s", addr)
		}
		seen[addr] = true
		if i > 0 && hops[i-1].Host == hop.Host && hops[i-1].Port == hop.Port {
			closeOpened()
			return nil, usererr.Coded("ssh.jump_same_host", "Jump host cannot be the same as the target")
		}
		var hopProxy ProxyConfig
		if i == 0 {
			hopProxy = proxy
		}
		client, err := m.dialOne(hop, timeout, prev, hopProxy)
		if err != nil {
			closeOpened()
			return nil, err
		}
		if prev != nil {
			jumps = append(jumps, prev)
		}
		prev = client
	}
	return &hopClient{target: prev, jumps: jumps}, nil
}

func (m *Manager) dialOne(ep Endpoint, timeout time.Duration, via *cryptossh.Client, proxy ProxyConfig) (*cryptossh.Client, error) {
	var agentSigners []cryptossh.Signer
	signers, cleanup, err := openAgentSigners()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		if agentUnavailableIsFatal(ep, m.keyboardPrompter != nil) {
			return nil, usererr.Wrap(err, "ssh.agent_unavailable", "SSH agent is unavailable: %s")
		}
		m.logger.Debug("ssh agent unavailable, continuing with other auth", zap.Error(err))
	} else {
		agentSigners = signers
	}
	methods, err := authMethods(ep, agentSigners)
	if err != nil {
		return nil, err
	}
	var conn net.Conn
	var keyboard cryptossh.AuthMethod
	if m.keyboardPrompter != nil {
		keyboard = m.keyboardAuth(ep, func() net.Conn { return conn }, timeout)
	}
	methods, err = finalizeAuthMethods(methods, keyboard)
	if err != nil {
		return nil, err
	}
	cfg := &cryptossh.ClientConfig{
		User:            ep.User,
		Auth:            methods,
		HostKeyCallback: m.hostKeyCallback,
		Timeout:         timeout,
	}

	m.logger.Debug("ssh auth",
		zap.Int("agent_keys", len(agentSigners)),
		zap.String("file", ep.Key),
		zap.Bool("stored_key", ep.KeyPEM != ""),
		zap.Bool("certificate", ep.Certificate != ""),
		zap.String("certificate_file", ep.Certificate),
		zap.Bool("keyboard_interactive", keyboard != nil),
	)
	addr := net.JoinHostPort(ep.Host, fmt.Sprintf("%d", ep.Port))

	viaJump := via != nil
	if viaJump {
		m.logger.Debug("Connecting via ProxyJump", zap.String("host", ep.Host), zap.Int("port", ep.Port))
		dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		conn, err = via.DialContext(dialCtx, "tcp", addr)
		if err != nil {
			m.logger.Debug("proxyjump dial tcp error", zap.Error(err))
			return nil, err
		}
	} else {
		if proxy.Enabled() {
			m.logger.Debug("Connecting via dial proxy",
				zap.String("type", proxy.Type),
				zap.String("proxyHost", proxy.Host),
				zap.Int("proxyPort", proxy.Port),
				zap.String("target", addr),
			)
		}
		conn, err = dialTCP(addr, timeout, proxy)
		if err != nil {
			m.logger.Debug("tcp connect error", zap.Error(err))
			return nil, err
		}
	}

	// Jump channel dials have no reliable local deadline; apply only for direct/proxy TCP.
	if !viaJump {
		if err = conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			conn.Close()
			m.logger.Debug("set deadline error", zap.Error(err))
			return nil, err
		}
	}
	c, chans, reqs, err := cryptossh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		m.logger.Debug("ssh handshake error", zap.Error(err))
		return nil, err
	}
	if !viaJump {
		if err := conn.SetDeadline(time.Time{}); err != nil {
			c.Close()
			m.logger.Debug("set deadline error", zap.Error(err))
			return nil, err
		}
	}
	return cryptossh.NewClient(c, chans, reqs), nil
}
