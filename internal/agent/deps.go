package agent

import (
	"context"
	"strings"
	"time"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/ai/tool"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/usererr"
)

// AIConfig is the persisted engine config (alias kept for call-site clarity).
type AIConfig = ai.Config

// StreamChunk is one emitted agent stream payload tagged with its AI session.
type StreamChunk = AIStreamChunkData

func (s *Service) userDataDir() string {
	if s.store == nil {
		return ""
	}
	return s.store.UserDataDir()
}

func (s *Service) requireUserDataDir() (string, error) {
	dir := s.userDataDir()
	if dir == "" {
		return "", usererr.Coded("ai.user_data_dir_empty", "User data directory is not set")
	}
	return dir, nil
}

func (s *Service) vaultPassword() (string, error) {
	if s.vault == nil {
		return "", usererr.Coded("error.password_not_entered", "Password not entered")
	}
	return s.vault.Get()
}

func (s *Service) encryptAPIKey(apiKey string) (string, error) {
	password, err := s.vaultPassword()
	if err != nil {
		return "", err
	}
	return secret.Encrypt(password, apiKey)
}

func (s *Service) decryptAPIKey(encrypted string) (string, error) {
	password, err := s.vaultPassword()
	if err != nil {
		return "", err
	}
	return secret.Decrypt(password, encrypted)
}

func (s *Service) saveAI(cfg ai.Config) error {
	if s.store == nil {
		return usererr.Coded("ai.config_unavailable", "Config service is not initialized")
	}
	s.store.Config.AI = cfg
	return s.store.Save()
}

func (s *Service) sshExec(ctx context.Context, linkID, command string, timeout time.Duration, maxOut int) (string, error) {
	if s.sshMgr == nil {
		return "", usererr.Coded("ai.ssh_unavailable", "SSH manager is not available")
	}
	return s.sshMgr.Exec(ctx, linkID, command, timeout, maxOut)
}

func (s *Service) sshAnnotate(linkID, notice string) error {
	if s.sshMgr == nil {
		return nil
	}
	return s.sshMgr.AnnotateSession(linkID, notice)
}

// screenFor binds the screen provider to one session; nil when unbound or
// no provider is wired (read_terminal stays unmounted).
func (s *Service) screenFor(linkID string) tool.ScreenFunc {
	if s.screenFn == nil || strings.TrimSpace(linkID) == "" {
		return nil
	}
	return func(_ context.Context, scrollbackLines int) (string, error) {
		return s.screenFn(linkID, scrollbackLines)
	}
}

func (s *Service) protocolOf(id string) (termio.Protocol, bool) {
	if s.hub == nil {
		return "", false
	}
	sess, ok := s.hub.Get(id)
	if !ok || sess == nil {
		return "", false
	}
	return sess.Protocol(), true
}

func (s *Service) serialPromptInfo(id string) (device string, baud int, ok bool) {
	if s.serialMgr == nil {
		return "", 0, false
	}
	ser, err := s.serialMgr.GetSession(id)
	if err != nil {
		return "", 0, false
	}
	return ser.Device(), ser.BaudRate(), true
}
