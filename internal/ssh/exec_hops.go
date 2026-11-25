package ssh

import (
	"context"
	"fmt"
	"time"

	"github.com/ilaziness/vela/internal/usererr"
)

// ExecHops runs command on a fresh non-interactive SSH session for the given
// hop chain, without registering a UI session. When an interactive client for
// the same hop chain is already cached it is reused; otherwise a dedicated
// connection is dialed, used once, and closed (no cache entry, no keepalive).
// Host key / keyboard-interactive prompts go through the default (main
// window) prompters, so callers must tolerate blocking on user confirmation.
func (m *Manager) ExecHops(ctx context.Context, hops []Endpoint, proxy ProxyConfig, command string, timeout time.Duration, maxOut int) (string, error) {
	if len(hops) == 0 {
		return "", usererr.Coded("ssh.empty_hops", "Empty hop list")
	}
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	if maxOut <= 0 {
		maxOut = DefaultExecMaxOut
	}
	execCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	clientKey := hopsClientKey(hops, proxy)
	if v, ok := m.clients.Load(clientKey); ok {
		return runRemoteCommandCtx(execCtx, v.(*hopClient).target, command, maxOut)
	}

	hc, err := m.dialHops(hops, m.dialTimeout(), proxy, "")
	if err != nil {
		return "", err
	}
	defer hc.Close()
	return runRemoteCommandCtx(execCtx, hc.target, command, maxOut)
}
