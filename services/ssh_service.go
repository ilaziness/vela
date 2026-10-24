package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termws"
	"github.com/ilaziness/vela/internal/tunnel"
)

const EventHostKeyPrompt = "eventHostKeyPrompt"
const EventKeyboardInteractive = "eventKeyboardInteractive"
const EventKeyboardInteractiveClose = "eventKeyboardInteractiveClose"
const EventSSHSessionClosed = "eventSSHSessionClosed"
const EventSSHSessionLogStopped = "eventSSHSessionLogStopped"

func init() {
	application.RegisterEvent[string](EventHostKeyPrompt)
	application.RegisterEvent[string](EventKeyboardInteractive)
	application.RegisterEvent[string](EventKeyboardInteractiveClose)
	application.RegisterEvent[string](EventSSHSessionClosed)
	application.RegisterEvent[string](EventSSHSessionLogStopped)
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

// ConnectRequest 临时直连参数；会话选项（TERM/env/startup/ForwardAgent）仅书签支持。
type ConnectRequest struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Password    string `json:"password"`
	Key         string `json:"key"`
	KeyPassword string `json:"keyPassword"`
	ProxyJumpID string `json:"proxyJumpID"`
	Certificate string `json:"certificate"`
}

func (r ConnectRequest) endpoint() ssh.Endpoint {
	return ssh.Endpoint{
		Host: r.Host, Port: r.Port, User: r.User,
		Password: r.Password, Key: r.Key, KeyPassword: r.KeyPassword,
		Certificate: r.Certificate,
	}
}

type SSHService struct {
	app                  *application.App
	mgr                  *ssh.Manager
	sftp                 *sftp.Manager
	tunnels              *tunnel.Manager
	termws               *termws.Server
	bookmarks            *bookmark.Service
	getSSHConfig         func() config.SSHConfig
	closing              sync.Map
	onSFTPSessionEnd     func(sessionID string)
	onSFTPSessionCleanup func(sessionID string)
	onSFTPCloseAll       func()
	onSFTPCloseAllClean  func()
}

// setSFTPCloseHooks runs edit cleanup around SFTP client close.
// onSession may block to confirm an upload and runs before the client closes.
// onSessionCleanup deletes temp files after the client closes and must not show a dialog.
// onAll runs on app exit before clients close and must not show a dialog.
// onAllCleanup deletes temp files after clients close and must not show a dialog.
func (s *SSHService) setSFTPCloseHooks(onSession, onSessionCleanup func(sessionID string), onAll, onAllCleanup func()) {
	s.onSFTPSessionEnd = onSession
	s.onSFTPSessionCleanup = onSessionCleanup
	s.onSFTPCloseAll = onAll
	s.onSFTPCloseAllClean = onAllCleanup
}

func NewSSHService(app *application.App, mgr *ssh.Manager, sftpMgr *sftp.Manager, tunnels *tunnel.Manager) *SSHService {
	s := &SSHService{app: app, mgr: mgr, sftp: sftpMgr, tunnels: tunnels}
	mgr.SetOnSessionLogStopped(s.onSessionLogStopped)
	return s
}

func (s *SSHService) onSessionLogStopped(id, reason string) {
	if s.app == nil || id == "" {
		return
	}
	data, err := json.Marshal(map[string]string{
		"id":     id,
		"reason": reason,
	})
	if err != nil {
		return
	}
	s.app.Event.Emit(EventSSHSessionLogStopped, string(data))
}

func (s *SSHService) setSSHConfigGetter(fn func() config.SSHConfig) {
	s.getSSHConfig = fn
}

func (s *SSHService) onSessionClosed(id string, reason ssh.CloseReason) {
	if s.app != nil {
		data, err := json.Marshal(map[string]string{
			"id":     id,
			"reason": string(reason),
		})
		if err == nil {
			s.app.Event.Emit(EventSSHSessionClosed, string(data))
		}
	}
	_ = s.CloseByID(id)
}

func (s *SSHService) bind(term *termws.Server, bookmarks *bookmark.Service) {
	s.termws = term
	s.bookmarks = bookmarks
}

func (s *SSHService) hops(req ConnectRequest) ([]ssh.Endpoint, error) {
	target := req.endpoint()
	if req.ProxyJumpID == "" {
		return []ssh.Endpoint{target}, nil
	}
	if s.bookmarks == nil {
		return nil, errors.New("bookmark service not initialized")
	}
	return s.bookmarks.ResolveHops(target, req.ProxyJumpID)
}

func (s *SSHService) connectHops(hops []ssh.Endpoint, proxy ssh.ProxyConfig) (string, error) {
	return s.mgr.Connect(hops, proxy)
}

func (s *SSHService) testHops(hops []ssh.Endpoint, proxy ssh.ProxyConfig) error {
	return s.mgr.TestConnect(hops, proxy)
}

func (s *SSHService) hasSession(id string) bool {
	return s.mgr.HasSession(id)
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

func (s *SSHService) Connect(req ConnectRequest) (string, error) {
	hops, err := s.hops(req)
	if err != nil {
		return "", err
	}
	proxy, err := s.globalDialProxy()
	if err != nil {
		return "", err
	}
	return s.connectHops(hops, proxy)
}

func (s *SSHService) Start(id string, cols, rows int) error {
	if err := s.mgr.Start(id, cols, rows); err != nil {
		_ = s.CloseByID(id)
		return err
	}
	return nil
}

func (s *SSHService) TestConnectInfo(req ConnectRequest) error {
	hops, err := s.hops(req)
	if err != nil {
		return err
	}
	proxy, err := s.globalDialProxy()
	if err != nil {
		return err
	}
	return s.testHops(hops, proxy)
}

func (s *SSHService) StartSftp(id, ownerKey string) error {
	return s.sftp.Connect(id, ownerKey)
}

func (s *SSHService) Resize(id string, cols, rows int) error {
	return s.mgr.Resize(id, cols, rows)
}

func (s *SSHService) Close() {
	if s.termws != nil {
		s.termws.CloseAllClients()
	}
	s.tunnels.StopAllAfter(func() {
		if s.onSFTPCloseAll != nil {
			s.onSFTPCloseAll()
		}
		s.sftp.CloseAll()
		if s.onSFTPCloseAllClean != nil {
			s.onSFTPCloseAllClean()
		}
		s.mgr.CloseAll()
	})
}

func (s *SSHService) CloseByID(id string) error {
	if _, loaded := s.closing.LoadOrStore(id, struct{}{}); loaded {
		return nil
	}
	defer s.closing.Delete(id)
	if s.termws != nil {
		s.termws.CloseClient(id)
	}
	var err error
	s.tunnels.StopSessionAfter(id, func() {
		if s.onSFTPSessionEnd != nil {
			s.onSFTPSessionEnd(id)
		}
		s.sftp.CloseSession(id)
		if s.onSFTPSessionCleanup != nil {
			s.onSFTPSessionCleanup(id)
		}
		err = s.mgr.CloseSession(id)
	})
	return err
}

func (s *SSHService) SelectKeyFile() (string, error) {
	return s.app.Dialog.OpenFile().SetTitle("选择私钥文件").PromptForSingleSelection()
}

func (s *SSHService) SelectCertificateFile() (string, error) {
	return s.app.Dialog.OpenFile().SetTitle("选择证书文件").PromptForSingleSelection()
}

func (s *SSHService) GetActiveSessions() []map[string]any {
	return s.mgr.ActiveSessions()
}

func (s *SSHService) SendToSession(sessionID, command string) error {
	return s.mgr.SendToSession(sessionID, command)
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
	return s.mgr.FetchHostResources(context.Background(), linkID)
}

// StartSessionLog opens a save dialog and begins recording session output.
// Cancelled dialog returns started=false with no error.
func (s *SSHService) StartSessionLog(sessionID string) (path string, started bool, err error) {
	if sessionID == "" {
		return "", false, fmt.Errorf("session id is required")
	}
	if !s.mgr.HasSession(sessionID) {
		return "", false, fmt.Errorf(ssh.ErrConnectionNotFound, sessionID)
	}
	if s.mgr.IsSessionLogging(sessionID) {
		return "", false, fmt.Errorf("session log already recording")
	}
	prefix := sessionID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	filename := fmt.Sprintf("vela-session-%s-%s.log", prefix, time.Now().Format("20060102-150405"))
	path, err = promptSaveFile(s.app, "保存会话日志", filename)
	if dialogCancelled(err) || path == "" {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err = s.mgr.StartSessionLog(sessionID, path); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func (s *SSHService) StopSessionLog(sessionID string) error {
	return s.mgr.StopSessionLog(sessionID)
}
