package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/termws"
	"github.com/ilaziness/vela/internal/tunnel"
	"github.com/ilaziness/vela/internal/usererr"
)

const EventSessionClosed = "eventSessionClosed"
const EventSessionLogStopped = "eventSessionLogStopped"

func init() {
	application.RegisterEvent[string](EventSessionClosed)
	application.RegisterEvent[string](EventSessionLogStopped)
}

// ConnectRequest is a flat multi-protocol connect payload. Protocol is required.
type ConnectRequest struct {
	Protocol string `json:"protocol"`

	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Password    string `json:"password"`
	Key         string `json:"key"`
	KeyPassword string `json:"keyPassword"`
	ProxyJumpID string `json:"proxyJumpID"`
	Certificate string `json:"certificate"`

	SerialDevice string `json:"serialDevice"`
	BaudRate     int    `json:"baudRate"`
	DataBits     int    `json:"dataBits"`
	Parity       string `json:"parity"`
	StopBits     int    `json:"stopBits"`
	FlowControl  string `json:"flowControl"`
	Charset      string `json:"charset"`
}

func (r ConnectRequest) sshEndpoint() ssh.Endpoint {
	return ssh.Endpoint{
		Host: r.Host, Port: r.Port, User: r.User,
		Password: r.Password, Key: r.Key, KeyPassword: r.KeyPassword,
		Certificate: r.Certificate,
		Charset:     r.Charset,
	}
}

func (r ConnectRequest) serialConfig() serial.OpenConfig {
	return serial.OpenConfig{
		Device:      r.SerialDevice,
		BaudRate:    r.BaudRate,
		DataBits:    r.DataBits,
		Parity:      r.Parity,
		StopBits:    r.StopBits,
		FlowControl: r.FlowControl,
		Charset:     r.Charset,
	}
}

type SessionService struct {
	app                  *application.App
	hub                  *termio.Hub
	sshMgr               *ssh.Manager
	serialMgr            *serial.Manager
	sftp                 *sftp.Manager
	tunnels              *tunnel.Manager
	store                *config.Store
	termws               *termws.Server
	bookmarks            *bookmark.Service
	getSSHConfig         func() config.SSHConfig
	closing              sync.Map
	onSFTPSessionEnd     func(sessionID string)
	onSFTPSessionCleanup func(sessionID string)
	onSFTPCloseAll       func()
	onSFTPCloseAllClean  func()
}

func NewSessionService(
	app *application.App,
	hub *termio.Hub,
	sshMgr *ssh.Manager,
	serialMgr *serial.Manager,
	sftpMgr *sftp.Manager,
	tunnels *tunnel.Manager,
	store *config.Store,
) *SessionService {
	s := &SessionService{
		app:       app,
		hub:       hub,
		sshMgr:    sshMgr,
		serialMgr: serialMgr,
		sftp:      sftpMgr,
		tunnels:   tunnels,
		store:     store,
	}
	sshMgr.SetOnSessionLogStopped(s.onSessionLogStopped)
	serialMgr.SetOnSessionLogStopped(s.onSessionLogStopped)
	return s
}

func (s *SessionService) setSFTPCloseHooks(onSession, onSessionCleanup func(sessionID string), onAll, onAllCleanup func()) {
	s.onSFTPSessionEnd = onSession
	s.onSFTPSessionCleanup = onSessionCleanup
	s.onSFTPCloseAll = onAll
	s.onSFTPCloseAllClean = onAllCleanup
}

func (s *SessionService) setSSHConfigGetter(fn func() config.SSHConfig) {
	s.getSSHConfig = fn
}

func (s *SessionService) bind(term *termws.Server, bookmarks *bookmark.Service) {
	s.termws = term
	s.bookmarks = bookmarks
}

func (s *SessionService) onSessionLogStopped(id, reason string) {
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
	s.app.Event.Emit(EventSessionLogStopped, string(data))
}

func (s *SessionService) onSSHSessionClosed(id string, reason ssh.CloseReason) {
	if s.app != nil {
		data, err := json.Marshal(map[string]string{
			"id":     id,
			"reason": string(reason),
		})
		if err == nil {
			s.app.Event.Emit(EventSessionClosed, string(data))
		}
	}
	_ = s.CloseByID(id)
}

func (s *SessionService) onSerialSessionClosed(id string) {
	if s.app != nil {
		data, err := json.Marshal(map[string]string{
			"id":     id,
			"reason": "unexpected",
		})
		if err == nil {
			s.app.Event.Emit(EventSessionClosed, string(data))
		}
	}
	_ = s.CloseByID(id)
}

func (s *SessionService) hops(req ConnectRequest) ([]ssh.Endpoint, error) {
	target := req.sshEndpoint()
	if req.ProxyJumpID == "" {
		return []ssh.Endpoint{target}, nil
	}
	if s.bookmarks == nil {
		return nil, usererr.Coded("ssh.bookmark_service_nil", "Bookmark service is not initialized")
	}
	return s.bookmarks.ResolveHops(target, req.ProxyJumpID)
}

func (s *SessionService) connectHops(hops []ssh.Endpoint, proxy ssh.ProxyConfig) (string, error) {
	id, err := s.sshMgr.Connect(hops, proxy)
	if err != nil {
		return "", mapSSHUserError(err)
	}
	sess, err := s.sshMgr.GetSession(id)
	if err != nil {
		_ = s.sshMgr.CloseSession(id)
		return "", mapSSHUserError(err)
	}
	s.hub.Register(ssh.AsTermio(sess))
	return id, nil
}

func (s *SessionService) testHops(hops []ssh.Endpoint, proxy ssh.ProxyConfig) error {
	return mapSSHUserError(s.sshMgr.TestConnect(hops, proxy))
}

func (s *SessionService) connectSerial(cfg serial.OpenConfig) (string, error) {
	id, err := s.serialMgr.Connect(cfg)
	if err != nil {
		return "", err
	}
	sess, err := s.serialMgr.GetSession(id)
	if err != nil {
		_ = s.serialMgr.CloseSession(id)
		return "", err
	}
	s.hub.Register(sess)
	return id, nil
}

func (s *SessionService) parseProtocol(raw string) (termio.Protocol, error) {
	p, ok := termio.ParseProtocol(strings.TrimSpace(raw))
	if !ok {
		return "", usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", raw)
	}
	return p, nil
}

func (s *SessionService) Connect(req ConnectRequest) (string, error) {
	proto, err := s.parseProtocol(req.Protocol)
	if err != nil {
		return "", err
	}
	switch proto {
	case termio.ProtocolSSH:
		hops, err := s.hops(req)
		if err != nil {
			return "", err
		}
		proxy, err := s.globalDialProxy()
		if err != nil {
			return "", err
		}
		return s.connectHops(hops, proxy)
	case termio.ProtocolSerial:
		return s.connectSerial(req.serialConfig())
	default:
		return "", usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", req.Protocol)
	}
}

func (s *SessionService) TestConnect(req ConnectRequest) error {
	proto, err := s.parseProtocol(req.Protocol)
	if err != nil {
		return err
	}
	switch proto {
	case termio.ProtocolSSH:
		hops, err := s.hops(req)
		if err != nil {
			return err
		}
		proxy, err := s.globalDialProxy()
		if err != nil {
			return err
		}
		return s.testHops(hops, proxy)
	case termio.ProtocolSerial:
		return s.testSerial(req.serialConfig())
	default:
		return usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", req.Protocol)
	}
}

func (s *SessionService) globalDialProxy() (ssh.ProxyConfig, error) {
	if s == nil || s.getSSHConfig == nil {
		return ssh.ProxyConfig{}, nil
	}
	return resolveDialProxy("inherit", ssh.ProxyConfig{}, s.getSSHConfig())
}

func (s *SessionService) Resize(id string, cols, rows int) error {
	sess, ok := s.hub.Get(id)
	if !ok || sess == nil {
		return usererr.Coded("session.not_found", "Session not found")
	}
	return sess.Resize(cols, rows)
}

func (s *SessionService) Close() {
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
		s.sshMgr.CloseAll()
		s.serialMgr.CloseAll()
		s.hub.Clear()
	})
}

func (s *SessionService) CloseByID(id string) error {
	if _, loaded := s.closing.LoadOrStore(id, struct{}{}); loaded {
		return nil
	}
	defer s.closing.Delete(id)
	if s.termws != nil {
		s.termws.CloseClient(id)
	}

	sess, ok := s.hub.Get(id)
	proto := termio.Protocol("")
	switch {
	case ok && sess != nil:
		proto = sess.Protocol()
	case s.sshMgr.HasSession(id):
		proto = termio.ProtocolSSH
	case s.serialMgr.HasSession(id):
		proto = termio.ProtocolSerial
	}

	var err error
	switch proto {
	case termio.ProtocolSSH:
		s.tunnels.StopSessionAfter(id, func() {
			if s.onSFTPSessionEnd != nil {
				s.onSFTPSessionEnd(id)
			}
			s.sftp.CloseSession(id)
			if s.onSFTPSessionCleanup != nil {
				s.onSFTPSessionCleanup(id)
			}
			err = s.sshMgr.CloseSession(id)
		})
		s.hub.Unregister(id)
	case termio.ProtocolSerial:
		err = s.serialMgr.CloseSession(id)
		s.hub.Unregister(id)
	default:
		s.hub.Unregister(id)
	}
	return err
}

func (s *SessionService) GetActiveSessions() []map[string]any {
	infos := s.hub.Active()
	out := make([]map[string]any, 0, len(infos))
	for _, info := range infos {
		out = append(out, map[string]any{
			"id":       info.ID,
			"protocol": string(info.Protocol),
			"label":    info.Label,
		})
	}
	return out
}

func (s *SessionService) SendToSession(sessionID, command string) error {
	sess, ok := s.hub.Get(sessionID)
	if !ok || sess == nil {
		return usererr.Coded("session.not_found", "Session not found")
	}
	stdin := sess.Stdin()
	if stdin == nil {
		return usererr.Coded("session.stdin_unavailable", "Standard input is not available")
	}
	// Serial consoles commonly expect CR; SSH shells expect LF.
	ending := "\n"
	if sess.Protocol() == termio.ProtocolSerial {
		ending = "\r"
	}
	_, err := stdin.Write([]byte(command + ending))
	return err
}

// serialPromptInfo returns device/baud for AI Current session (serial only).
func (s *SessionService) serialPromptInfo(id string) (device string, baud int, ok bool) {
	ser, err := s.serialMgr.GetSession(id)
	if err != nil {
		return "", 0, false
	}
	return ser.Device(), ser.BaudRate(), true
}

func (s *SessionService) ListSerialPorts() ([]string, error) {
	return s.serialMgr.ListPorts()
}

func (s *SessionService) testSerial(cfg serial.OpenConfig) error {
	return s.serialMgr.TestConnect(cfg)
}

// protocolOf is package-private so Wails does not export Hub protocol lookup.
func (s *SessionService) protocolOf(id string) (termio.Protocol, bool) {
	sess, ok := s.hub.Get(id)
	if !ok || sess == nil {
		return "", false
	}
	return sess.Protocol(), true
}

// StartSessionLog opens a save dialog and begins recording session output.
func (s *SessionService) StartSessionLog(sessionID string) (path string, started bool, err error) {
	if sessionID == "" {
		return "", false, usererr.Coded("session.required", "Session is required")
	}
	sess, ok := s.hub.Get(sessionID)
	if !ok || sess == nil {
		return "", false, usererr.Coded("session.not_found", "Session not found")
	}
	proto := sess.Protocol()
	logging := false
	switch proto {
	case termio.ProtocolSSH:
		logging = s.sshMgr.IsSessionLogging(sessionID)
	case termio.ProtocolSerial:
		logging = s.serialMgr.IsSessionLogging(sessionID)
	}
	if logging {
		return "", false, usererr.Coded("session.log_already_recording", "Session log is already recording")
	}
	prefix := sessionID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	filename := fmt.Sprintf("vela-session-%s-%s.log", prefix, time.Now().Format("20060102-150405"))
	path, err = promptSaveFile(s.app, s.store, i18n.T("dialog.save_session_log"), filename)
	if dialogCancelled(err) || path == "" {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	switch proto {
	case termio.ProtocolSSH:
		err = s.sshMgr.StartSessionLog(sessionID, path)
	case termio.ProtocolSerial:
		err = s.serialMgr.StartSessionLog(sessionID, path)
	default:
		err = usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", string(proto))
	}
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func (s *SessionService) StopSessionLog(sessionID string) error {
	sess, ok := s.hub.Get(sessionID)
	if !ok || sess == nil {
		return nil
	}
	switch sess.Protocol() {
	case termio.ProtocolSSH:
		return s.sshMgr.StopSessionLog(sessionID)
	case termio.ProtocolSerial:
		return s.serialMgr.StopSessionLog(sessionID)
	default:
		return nil
	}
}
