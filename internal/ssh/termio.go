package ssh

import (
	"fmt"
	"io"

	"github.com/ilaziness/vela/internal/termio"
)

// HubSession adapts *Session to termio.Session without renaming exported fields.
type HubSession struct {
	*Session
}

var _ termio.Session = HubSession{}

// AsTermio wraps s for Hub registration. Nil yields a zero value that should not be registered.
func AsTermio(s *Session) HubSession {
	return HubSession{Session: s}
}

func (h HubSession) ID() string {
	if h.Session == nil {
		return ""
	}
	return h.Session.ID
}

func (h HubSession) Protocol() termio.Protocol {
	return termio.ProtocolSSH
}

func (h HubSession) Label() string {
	if h.Session == nil {
		return ""
	}
	return h.label
}

func (h HubSession) Stdin() io.WriteCloser {
	if h.Session == nil {
		return nil
	}
	return h.Session.Stdin
}

func (h HubSession) Output() <-chan []byte {
	if h.Session == nil {
		return nil
	}
	return h.OutputChan
}

func (h HubSession) Close() error {
	if h.Session == nil || h.manager == nil {
		return nil
	}
	return h.manager.CloseSession(h.Session.ID)
}

func sessionLabel(ep Endpoint) string {
	user := ep.User
	if user == "" {
		user = "?"
	}
	host := ep.Host
	if host == "" {
		host = "?"
	}
	if ep.Port > 0 {
		return fmt.Sprintf("%s@%s:%d", user, host, ep.Port)
	}
	return fmt.Sprintf("%s@%s", user, host)
}
