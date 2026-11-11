package serialui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/ui/protocol"
)

// Connector opens serial ports via the existing engine.
type Connector struct {
	mgr *serial.Manager
	hub *termio.Hub
}

// NewConnector wires mgr and hub.
func NewConnector(mgr *serial.Manager, hub *termio.Hub) *Connector {
	return &Connector{mgr: mgr, hub: hub}
}

// Protocol implements protocol.Connector.
func (c *Connector) Protocol() termio.Protocol { return termio.ProtocolSerial }

// Connect opens the port and registers the session on the hub.
func (c *Connector) Connect(_ context.Context, draft protocol.Draft) (termio.Session, error) {
	cfg, err := openConfigFromDraft(draft)
	if err != nil {
		return nil, err
	}
	id, err := c.mgr.Connect(cfg)
	if err != nil {
		return nil, err
	}
	sess, err := c.mgr.GetSession(id)
	if err != nil {
		_ = c.mgr.CloseSession(id)
		return nil, err
	}
	c.hub.Register(sess)
	return sess, nil
}

// Test opens then closes the port.
func (c *Connector) Test(_ context.Context, draft protocol.Draft) error {
	cfg, err := openConfigFromDraft(draft)
	if err != nil {
		return err
	}
	return c.mgr.TestConnect(cfg)
}

func openConfigFromDraft(draft protocol.Draft) (serial.OpenConfig, error) {
	p, ok := payloadFrom(draft.Payload)
	if !ok {
		return serial.OpenConfig{}, fmt.Errorf("invalid serial draft payload")
	}
	device := strings.TrimSpace(p.Device)
	if device == "" {
		return serial.OpenConfig{}, fmt.Errorf("请选择串口设备")
	}
	baud := p.BaudRate
	if baud <= 0 {
		baud = 115200
	}
	return serial.OpenConfig{
		Device:   device,
		BaudRate: baud,
		// Engine defaults: 8N1, no flow — bookmarks can override later.
	}, nil
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

// Entry builds the registry entry for serial.
func Entry(mgr *serial.Manager, hub *termio.Hub) protocol.Entry {
	return protocol.Entry{
		Protocol: termio.ProtocolSerial,
		Label:    "串口",
		Enabled:  true,
		Connect:  NewConnector(mgr, hub),
		Form:     bindForm(mgr),
		NewDraft: func() protocol.Draft {
			p := DefaultPayload()
			return protocol.Draft{Protocol: termio.ProtocolSerial, Payload: &p}
		},
	}
}
