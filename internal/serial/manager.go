package serial

import (
	"fmt"
	"sync"

	goserial "go.bug.st/serial"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/termcharset"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/internal/utils"
)

// Manager owns open serial sessions and enforces per-device mutual exclusion.
type Manager struct {
	logger              *zap.Logger
	sessions            sync.Map // id -> *Session
	devices             sync.Map // device -> session id
	onClose             func(sessionID string)
	onSessionLogStopped func(sessionID, reason string)
}

func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{logger: logger}
}

func (m *Manager) SetOnClose(fn func(sessionID string)) {
	m.onClose = fn
}

func (m *Manager) SetOnSessionLogStopped(fn func(sessionID, reason string)) {
	m.onSessionLogStopped = fn
}

// ListPorts returns OS serial port names.
func (m *Manager) ListPorts() ([]string, error) {
	ports, err := goserial.GetPortsList()
	if err != nil {
		return nil, usererr.Wrap(err, "serial.list_failed", "Failed to list serial ports: %s")
	}
	if ports == nil {
		return []string{}, nil
	}
	return ports, nil
}

// Connect opens the port and creates a session (read starts on Start).
func (m *Manager) Connect(cfg OpenConfig) (string, error) {
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return "", err
	}
	id := utils.GenerateRandomID()
	// Reserve device before Open so concurrent Connect calls cannot both pass.
	if _, loaded := m.devices.LoadOrStore(cfg.Device, id); loaded {
		return "", usererr.Coded("serial.port_busy", "Serial port is already in use: %s", cfg.Device)
	}
	rollback := true
	defer func() {
		if rollback {
			m.releaseDevice(cfg.Device, id)
		}
	}()

	mode, err := cfg.mode()
	if err != nil {
		return "", err
	}
	port, err := goserial.Open(cfg.Device, mode)
	if err != nil {
		m.logger.Error("serial open failed", zap.String("device", cfg.Device), zap.Error(err))
		return "", usererr.Wrap(err, "serial.open_failed", "Failed to open serial port: %s")
	}
	if err := applyFlowControl(port, cfg.FlowControl); err != nil {
		_ = port.Close()
		return "", err
	}

	charset, err := termcharset.Normalize(cfg.Charset)
	if err != nil {
		_ = port.Close()
		return "", err
	}
	var enc *termcharset.Stream
	if e, eerr := termcharset.NewEncoder(charset); eerr == nil {
		enc = e
	} else {
		m.logger.Warn("serial charset encoder", zap.String("charset", charset), zap.Error(eerr))
	}

	sess := &Session{
		id:         id,
		label:      cfg.label(),
		device:     cfg.Device,
		cfg:        cfg,
		port:       port,
		outputChan: make(chan []byte, 200),
		stopOutput: make(chan struct{}),
		manager:    m,
		charset:    charset,
		inEncoder:  enc,
	}
	m.sessions.Store(id, sess)
	rollback = false
	m.logger.Debug("serial connect ok", zap.String("id", id), zap.String("device", cfg.Device))
	return id, nil
}

// TestConnect opens then closes the port. Busy ports return port_busy.
func (m *Manager) TestConnect(cfg OpenConfig) error {
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return err
	}
	if _, ok := m.devices.Load(cfg.Device); ok {
		return usererr.Coded("serial.port_busy", "Serial port is already in use: %s", cfg.Device)
	}
	mode, err := cfg.mode()
	if err != nil {
		return err
	}
	port, err := goserial.Open(cfg.Device, mode)
	if err != nil {
		m.logger.Error("serial test open failed", zap.String("device", cfg.Device), zap.Error(err))
		return usererr.Wrap(err, "serial.open_failed", "Failed to open serial port: %s")
	}
	_ = port.Close()
	return nil
}

func (m *Manager) GetSession(id string) (*Session, error) {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil, usererr.Coded("serial.session_not_found", "Serial session not found")
	}
	return v.(*Session), nil
}

func (m *Manager) HasSession(id string) bool {
	_, ok := m.sessions.Load(id)
	return ok
}

func (m *Manager) CloseSession(id string) error {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil
	}
	_ = v.(*Session).closePort(false)
	m.sessions.Delete(id)
	return nil
}

func (m *Manager) CloseAll() {
	m.sessions.Range(func(key, value any) bool {
		_ = value.(*Session).closePort(false)
		m.sessions.Delete(key)
		return true
	})
}

func (m *Manager) releaseDevice(device, id string) {
	if device == "" {
		return
	}
	if v, ok := m.devices.Load(device); ok && fmt.Sprint(v) == id {
		m.devices.Delete(device)
	}
}

func (m *Manager) notifyClosed(id string) {
	if m.onClose != nil {
		m.onClose(id)
	}
}

func applyFlowControl(port goserial.Port, flow string) error {
	switch flow {
	case "", "none":
		return nil
	case "hardware":
		if err := port.SetRTS(true); err != nil {
			return usererr.Wrap(err, "serial.open_failed", "Failed to set RTS: %s")
		}
		if err := port.SetDTR(true); err != nil {
			return usererr.Wrap(err, "serial.open_failed", "Failed to set DTR: %s")
		}
		return nil
	default:
		return usererr.Coded("serial.flow_invalid", "Invalid flow control: %s", flow)
	}
}
