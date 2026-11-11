package sshui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/protocol"
)

// Connector dials SSH via the existing engine.
type Connector struct {
	mgr         *ssh.Manager
	hub         *termio.Hub
	logger      *zap.Logger
	globalProxy func() ssh.GlobalProxy
}

// NewConnector wires mgr and hub.
func NewConnector(mgr *ssh.Manager, hub *termio.Hub, logger *zap.Logger, globalProxy func() ssh.GlobalProxy) *Connector {
	if logger == nil {
		logger = zap.NewNop()
	}
	if globalProxy == nil {
		globalProxy = func() ssh.GlobalProxy { return ssh.GlobalProxy{} }
	}
	return &Connector{mgr: mgr, hub: hub, logger: logger, globalProxy: globalProxy}
}

// Protocol implements protocol.Connector.
func (c *Connector) Protocol() termio.Protocol { return termio.ProtocolSSH }

// Connect dials and registers the session on the hub.
func (c *Connector) Connect(_ context.Context, draft protocol.Draft) (termio.Session, error) {
	ep, err := endpointFromDraft(draft)
	if err != nil {
		return nil, err
	}
	proxy, err := ssh.ResolveDialProxy("inherit", ssh.ProxyConfig{}, c.globalProxy())
	if err != nil {
		return nil, ssh.MapUserError(err)
	}
	id, err := c.mgr.Connect([]ssh.Endpoint{ep}, proxy)
	if err != nil {
		return nil, ssh.MapUserError(err)
	}
	sess, err := c.mgr.GetSession(id)
	if err != nil {
		_ = c.mgr.CloseSession(id)
		return nil, ssh.MapUserError(err)
	}
	hs := ssh.AsTermio(sess)
	c.hub.Register(hs)
	return hs, nil
}

// Test dials without keeping a session.
func (c *Connector) Test(_ context.Context, draft protocol.Draft) error {
	ep, err := endpointFromDraft(draft)
	if err != nil {
		return err
	}
	proxy, err := ssh.ResolveDialProxy("inherit", ssh.ProxyConfig{}, c.globalProxy())
	if err != nil {
		return ssh.MapUserError(err)
	}
	return ssh.MapUserError(c.mgr.TestConnect([]ssh.Endpoint{ep}, proxy))
}

func endpointFromDraft(draft protocol.Draft) (ssh.Endpoint, error) {
	p, ok := payloadFrom(draft.Payload)
	if !ok {
		return ssh.Endpoint{}, fmt.Errorf("invalid SSH draft payload")
	}
	host := strings.TrimSpace(p.Host)
	user := strings.TrimSpace(p.User)
	if host == "" || user == "" {
		return ssh.Endpoint{}, fmt.Errorf("host and username are required")
	}
	port := p.Port
	if port <= 0 {
		port = 22
	}
	ep := ssh.Endpoint{
		Host:        host,
		Port:        port,
		User:        user,
		Password:    p.Password,
		KeyPassword: p.KeyPassword,
	}
	if path := strings.TrimSpace(p.KeyPath); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return ssh.Endpoint{}, fmt.Errorf("read key file: %w", err)
		}
		ep.KeyPEM = string(b)
		ep.Key = path
	}
	if path := strings.TrimSpace(p.CertPath); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return ssh.Endpoint{}, fmt.Errorf("read certificate: %w", err)
		}
		ep.Certificate = string(b)
	}
	return ep, nil
}

func payloadFrom(v any) (Payload, bool) {
	switch p := v.(type) {
	case Payload:
		return p, true
	case *Payload:
		if p == nil {
			return Payload{}, false
		}
		return *p, true
	default:
		return Payload{}, false
	}
}

// Entry builds the registry entry for SSH.
func Entry(mgr *ssh.Manager, hub *termio.Hub, logger *zap.Logger, globalProxy func() ssh.GlobalProxy) protocol.Entry {
	return protocol.Entry{
		Protocol: termio.ProtocolSSH,
		Label:    "SSH",
		Enabled:  true,
		Connect:  NewConnector(mgr, hub, logger, globalProxy),
		Form:     Form,
		NewDraft: func() protocol.Draft {
			p := DefaultPayload()
			return protocol.Draft{Protocol: termio.ProtocolSSH, Payload: &p}
		},
	}
}
