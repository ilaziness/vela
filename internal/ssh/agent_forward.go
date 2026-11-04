package ssh

import (
	"errors"
	"io"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/ilaziness/vela/internal/usererr"
)

func (m *Manager) enableAgentForward(clientKey string) error {
	v, ok := m.clients.Load(clientKey)
	if !ok {
		return usererr.Coded("ssh.client_not_found", "SSH client not found")
	}
	return v.(*hopClient).enableAgentForward(m.logger)
}

func (h *hopClient) enableAgentForward(logger *zap.Logger) error {
	h.forwardMu.Lock()
	defer h.forwardMu.Unlock()
	if h.agentForwarded {
		return nil
	}
	if h.target == nil {
		return usererr.Coded("ssh.client_not_found", "SSH client not found")
	}

	eps, err := dialAgentEndpoints()
	if err != nil || len(eps) == 0 {
		if err == nil {
			err = errors.New("no SSH agent endpoints")
		}
		return usererr.Wrap(err, "ssh.agent_unavailable", "SSH agent is unavailable: %s")
	}

	var agents []agent.Agent
	var closers []io.Closer
	cleanup := func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}
	var listErr error
	for i, ep := range eps {
		a := agent.NewClient(serialReadWriter{rw: ep.rw})
		if _, lerr := a.List(); lerr != nil {
			listErr = lerr
			if ep.closer != nil {
				_ = ep.closer.Close()
				eps[i].closer = nil
			}
			continue
		}
		agents = append(agents, a)
		if ep.closer != nil {
			closers = append(closers, ep.closer)
		}
	}
	if len(agents) == 0 {
		if listErr == nil {
			listErr = errors.New("no usable SSH agent")
		}
		return usererr.Wrap(listErr, "ssh.agent_unavailable", "SSH agent is unavailable: %s")
	}

	var keyring agent.Agent
	if len(agents) == 1 {
		keyring = agents[0]
	} else {
		keyring = &multiAgent{agents: agents}
	}
	if err := agent.ForwardToAgent(h.target, keyring); err != nil {
		cleanup()
		return err
	}
	h.agentClosers = closers
	h.agentForwarded = true
	logger.Debug("ssh agent forwarding enabled", zap.Int("agents", len(agents)))
	return nil
}

// multiAgent merges multiple local agents (e.g. OpenSSH pipe + Pageant).
type multiAgent struct {
	agents []agent.Agent
}

func (m *multiAgent) List() ([]*agent.Key, error) {
	seen := make(map[string]struct{})
	var out []*agent.Key
	var lastErr error
	for _, a := range m.agents {
		keys, err := a.List()
		if err != nil {
			lastErr = err
			continue
		}
		for _, k := range keys {
			blob := string(k.Blob)
			if _, ok := seen[blob]; ok {
				continue
			}
			seen[blob] = struct{}{}
			out = append(out, k)
		}
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}

func (m *multiAgent) Sign(key cryptossh.PublicKey, data []byte) (*cryptossh.Signature, error) {
	var lastErr error
	for _, a := range m.agents {
		sig, err := a.Sign(key, data)
		if err != nil {
			lastErr = err
			continue
		}
		return sig, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("agent: key not found")
}

func (m *multiAgent) Add(key agent.AddedKey) error {
	if len(m.agents) == 0 {
		return errors.New("agent: no backends")
	}
	return m.agents[0].Add(key)
}

func (m *multiAgent) Remove(key cryptossh.PublicKey) error {
	var lastErr error
	removed := false
	for _, a := range m.agents {
		if err := a.Remove(key); err != nil {
			lastErr = err
			continue
		}
		removed = true
	}
	if removed {
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("agent: key not found")
}

func (m *multiAgent) RemoveAll() error {
	var lastErr error
	for _, a := range m.agents {
		if err := a.RemoveAll(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (m *multiAgent) Lock(passphrase []byte) error {
	var lastErr error
	for _, a := range m.agents {
		if err := a.Lock(passphrase); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (m *multiAgent) Unlock(passphrase []byte) error {
	var lastErr error
	for _, a := range m.agents {
		if err := a.Unlock(passphrase); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (m *multiAgent) Signers() ([]cryptossh.Signer, error) {
	seen := make(map[string]struct{})
	var out []cryptossh.Signer
	var lastErr error
	for _, a := range m.agents {
		list, err := a.Signers()
		if err != nil {
			lastErr = err
			continue
		}
		for _, s := range list {
			blob := string(s.PublicKey().Marshal())
			if _, ok := seen[blob]; ok {
				continue
			}
			seen[blob] = struct{}{}
			out = append(out, s)
		}
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}
