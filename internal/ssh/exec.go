package ssh

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
)

const (
	DefaultExecMaxOut = 64 * 1024 // 64KB
)

// Exec runs command on a new non-interactive session bound to the same SSH
// client as linkID. Output is CombinedOutput, truncated to maxOut bytes.
func (m *Manager) Exec(ctx context.Context, linkID, command string, timeout time.Duration, maxOut int) (string, error) {
	if linkID == "" {
		return "", fmt.Errorf("link id is required")
	}
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	client, err := m.GetClient(linkID)
	if err != nil {
		return "", err
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
	return runRemoteCommandCtx(execCtx, client, command, maxOut)
}

// AnnotateSession writes a display-only notice to the interactive terminal
// output channel. It never writes to stdin and does not execute remotely.
func (m *Manager) AnnotateSession(sessionID, notice string) error {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}
	if notice == "" {
		return nil
	}
	data := []byte(notice)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\r', '\n')
	} else if len(data) == 1 || data[len(data)-2] != '\r' {
		data = append(data[:len(data)-1], '\r', '\n')
	}
	sess.teeOutput(data)
	select {
	case sess.OutputChan <- data:
		return nil
	case <-sess.stopOutput:
		return fmt.Errorf("session output closed")
	default:
		m.logger.Debug("AnnotateSession dropped: output buffer full", zap.String("id", sessionID))
		return nil
	}
}

func runRemoteCommandCtx(ctx context.Context, client *cryptossh.Client, command string, maxOut int) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	type cmdResult struct {
		out []byte
		err error
	}
	done := make(chan cmdResult, 1)
	go func() {
		out, runErr := session.CombinedOutput(command)
		done <- cmdResult{out: out, err: runErr}
	}()

	select {
	case r := <-done:
		out := truncateUTF8(string(r.out), maxOut)
		if r.err != nil && out == "" {
			return "", r.err
		}
		return out, r.err
	case <-ctx.Done():
		_ = session.Close()
		return "", ctx.Err()
	}
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s + "\n…[truncated]"
}
