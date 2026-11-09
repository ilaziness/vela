package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/ssh"
)

const EventHostKeyPrompt = "eventHostKeyPrompt"
const EventKeyboardInteractive = "eventKeyboardInteractive"
const EventKeyboardInteractiveClose = "eventKeyboardInteractiveClose"
const EventTerminalCwd = "eventTerminalCwd"

func init() {
	application.RegisterEvent[string](EventHostKeyPrompt)
	application.RegisterEvent[string](EventKeyboardInteractive)
	application.RegisterEvent[string](EventKeyboardInteractiveClose)
	application.RegisterEvent[string](EventTerminalCwd)
}

type hostKeyPrompter struct {
	app *application.App
}

func (p *hostKeyPrompter) Prompt(hp ssh.HostKeyPrompt) error {
	payload := map[string]any{
		"host": hp.Host, "address": hp.Address, "fingerprint": hp.Fingerprint,
		"key_type": hp.KeyType, "mismatch": hp.Mismatch,
	}
	if hp.Mismatch {
		payload["old_fingerprint"] = hp.OldFingerprint
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	p.app.Event.Emit(EventHostKeyPrompt, string(data))
	return nil
}

type keyboardInteractivePrompter struct {
	app *application.App
}

func (p *keyboardInteractivePrompter) Prompt(c ssh.KeyboardChallenge) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	p.app.Event.Emit(EventKeyboardInteractive, string(data))
	return nil
}

func (p *keyboardInteractivePrompter) Dismiss(id string) {
	if p == nil || p.app == nil || id == "" {
		return
	}
	p.app.Event.Emit(EventKeyboardInteractiveClose, id)
}

type SSHService struct {
	app          *application.App
	mgr          *ssh.Manager
	sftp         *sftp.Manager
	store        *config.Store
	getSSHConfig func() config.SSHConfig
}

func NewSSHService(app *application.App, mgr *ssh.Manager, sftpMgr *sftp.Manager, store *config.Store) *SSHService {
	s := &SSHService{app: app, mgr: mgr, sftp: sftpMgr, store: store}
	mgr.SetOnCwd(s.onTerminalCwd)
	return s
}

func (s *SSHService) onTerminalCwd(id, path string) {
	if s.app == nil || id == "" || path == "" {
		return
	}
	data, err := json.Marshal(map[string]string{
		"id":   id,
		"path": path,
	})
	if err != nil {
		return
	}
	s.app.Event.Emit(EventTerminalCwd, string(data))
}

// TerminalCwd is the SFTP follow-cwd RPC: last OSC path, or "" if the remote never reported.
func (s *SSHService) TerminalCwd(id string) string {
	return s.mgr.Cwd(id)
}

func (s *SSHService) setSSHConfigGetter(fn func() config.SSHConfig) {
	s.getSSHConfig = fn
}

func (s *SSHService) exec(ctx context.Context, linkID, command string, timeout time.Duration, maxOut int) (string, error) {
	if s == nil || s.mgr == nil {
		return "", fmt.Errorf("ssh manager unavailable")
	}
	return s.mgr.Exec(ctx, linkID, command, timeout, maxOut)
}

func (s *SSHService) annotateSession(linkID, notice string) error {
	if s == nil || s.mgr == nil {
		return nil
	}
	return s.mgr.AnnotateSession(linkID, notice)
}

func (s *SSHService) StartSftp(id, ownerKey string) error {
	return s.sftp.Connect(id, ownerKey)
}

func (s *SSHService) SelectKeyFile() (string, error) {
	return promptOpenFile(s.app, s.store, i18n.T("dialog.select_private_key"))
}

func (s *SSHService) SelectCertificateFile() (string, error) {
	return promptOpenFile(s.app, s.store, i18n.T("dialog.select_certificate"))
}

func (s *SSHService) SetHostKeyDecision(host string, accept bool) error {
	return s.mgr.SetHostKeyDecision(host, accept)
}

func (s *SSHService) ListKnownHosts() ([]ssh.KnownHostEntry, error) {
	return s.mgr.ListKnownHosts()
}

func (s *SSHService) DeleteKnownHost(host, keyType string) error {
	return s.mgr.DeleteKnownHost(host, keyType)
}

func (s *SSHService) AnswerKeyboardInteractive(id string, answers []string) error {
	return s.mgr.AnswerKeyboardInteractive(id, answers)
}

func (s *SSHService) CancelKeyboardInteractive(id string) error {
	return s.mgr.CancelKeyboardInteractive(id)
}

func (s *SSHService) GetOrFetchRemoteSystemInfo(linkID, host string) *ssh.RemoteSystemInfo {
	return s.mgr.GetOrFetchRemoteSystemInfo(linkID, host)
}

func (s *SSHService) GetHostResources(linkID string) (ssh.HostResources, error) {
	r, err := s.mgr.FetchHostResources(context.Background(), linkID)
	return r, mapSSHUserError(err)
}
