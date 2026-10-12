package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/ilaziness/vexo/internal/system"
	"github.com/ilaziness/vexo/internal/utils"
	"github.com/ilaziness/vexo/internal/zmodem"
)

const ErrConnectionNotFound = "SSH connection with ID %s not found"

// CloseReason distinguishes clean shell exits from unexpected drops for auto-reconnect.
type CloseReason string

const (
	CloseReasonClean      CloseReason = "clean"
	CloseReasonUnexpected CloseReason = "unexpected"
)

func closeReasonFromWait(err error) CloseReason {
	if err == nil {
		return CloseReasonClean
	}
	var exitErr *cryptossh.ExitError
	if errors.As(err, &exitErr) {
		return CloseReasonClean
	}
	return CloseReasonUnexpected
}

type Endpoint struct {
	Host         string
	Port         int
	User         string
	Password     string
	Key          string
	KeyPassword  string
	KeyPEM       string
	Certificate  string
	ForwardAgent bool
	StartupCmd   string
	Env          map[string]string
	Term         string // empty → xterm-256color at RequestPty
}

const defaultTermType = "xterm-256color"

// Wait for MOTD/prompt to finish before stdin injection (avoids double echo).
const (
	shellReadyQuiet   = 300 * time.Millisecond
	shellReadyTimeout = 3 * time.Second
	shellReadyPoll    = 20 * time.Millisecond
)

func (e Endpoint) Addr() string {
	return fmt.Sprintf("%s:%d", e.Host, e.Port)
}

func (e Endpoint) ClientKey() string {
	return fmt.Sprintf("%s@%s:%d", e.User, e.Host, e.Port)
}

type HostKeyPrompt struct {
	Host           string
	Address        string
	Fingerprint    string
	KeyType        string
	Mismatch       bool
	OldFingerprint string
}

type HostKeyPrompter interface {
	Prompt(p HostKeyPrompt) error
}

type Session struct {
	ID             string
	ClientKey      string
	client         *cryptossh.Client
	session        *cryptossh.Session
	isClosed       bool
	closeMu        sync.Mutex
	OutputChan     chan []byte
	outputBuffSize int
	outputWg       sync.WaitGroup
	outputOnce     sync.Once
	Stdin          io.WriteCloser
	stopOutput     chan struct{}
	stopOutputOnce sync.Once
	manager        *Manager
	forwardAgent   bool
	startupCmd     string
	env            map[string]string
	term           string
	outputMu       sync.Mutex
	gotOutput      bool
	lastOutputAt   time.Time
	logMu          sync.Mutex
	logFile        *os.File
	zmFilter       *zmodem.Filter
}

type hopClient struct {
	target         *cryptossh.Client
	jumps          []*cryptossh.Client
	forwardMu      sync.Mutex
	agentForwarded bool
	agentClosers   []io.Closer
}

func (h *hopClient) Close() {
	if h == nil {
		return
	}
	// 先关 SSH，停掉 auth-agent 通道上的 ServeAgent，再关本机 Agent 连接。
	if h.target != nil {
		_ = h.target.Close()
	}
	for i := len(h.jumps) - 1; i >= 0; i-- {
		_ = h.jumps[i].Close()
	}
	h.forwardMu.Lock()
	for _, c := range h.agentClosers {
		if c != nil {
			_ = c.Close()
		}
	}
	h.agentClosers = nil
	h.agentForwarded = false
	h.forwardMu.Unlock()
}

// Options are runtime SSH client settings applied on next dial / new keepalive.
type Options struct {
	ServerAliveInterval time.Duration
	DialTimeout         time.Duration
}

type Manager struct {
	logger                *zap.Logger
	knownHostsPath        string
	knownHostsMu          sync.Mutex
	prompter              HostKeyPrompter
	keyboardPrompter      KeyboardInteractivePrompter
	onClose               func(sessionID string, reason CloseReason)
	onSessionLogStopped   func(sessionID, reason string)
	clients               *sync.Map
	sessions              *sync.Map
	clientLocks           sync.Map
	remoteInfoCache       sync.Map
	remoteInfoFetchLocks  sync.Map
	hostMetricsCache      sync.Map
	hostMetricsFetchLocks sync.Map
	hostKey               *hostKeyStore
	keyboard              *keyboardStore
	keepAlives            sync.Map
	optsMu                sync.RWMutex
	opts                  Options
	zmodemPicker          zmodem.FilePicker
}

func NewManager(logger *zap.Logger, knownHostsPath string, prompter HostKeyPrompter, keyboard KeyboardInteractivePrompter) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:           logger,
		knownHostsPath:   knownHostsPath,
		prompter:         prompter,
		keyboardPrompter: keyboard,
		clients:          new(sync.Map),
		sessions:         new(sync.Map),
		hostKey:          newHostKeyStore(),
		keyboard:         newKeyboardStore(),
		opts: Options{
			ServerAliveInterval: 30 * time.Second,
			DialTimeout:         30 * time.Second,
		},
	}
}

// SetZmodem enables rz/sz detection for new sessions. Nil disables.
func (m *Manager) SetZmodem(picker zmodem.FilePicker) {
	m.zmodemPicker = picker
}

func (m *Manager) SetOptions(opts Options) {
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 30 * time.Second
	}
	if opts.ServerAliveInterval < 0 {
		opts.ServerAliveInterval = 0
	}
	m.optsMu.Lock()
	m.opts = opts
	m.optsMu.Unlock()
}

func (m *Manager) dialTimeout() time.Duration {
	m.optsMu.RLock()
	defer m.optsMu.RUnlock()
	if m.opts.DialTimeout <= 0 {
		return 30 * time.Second
	}
	return m.opts.DialTimeout
}

func (m *Manager) aliveInterval() time.Duration {
	m.optsMu.RLock()
	defer m.optsMu.RUnlock()
	return m.opts.ServerAliveInterval
}

func (m *Manager) SetOnClose(fn func(sessionID string, reason CloseReason)) {
	m.onClose = fn
}

func (m *Manager) GetSession(id string) (*Session, error) {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil, fmt.Errorf(ErrConnectionNotFound, id)
	}
	return v.(*Session), nil
}

func (m *Manager) GetClient(sessionID string) (*cryptossh.Client, error) {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if sess.client == nil {
		return nil, fmt.Errorf(ErrConnectionNotFound, sessionID)
	}
	return sess.client, nil
}

func (m *Manager) HasSession(id string) bool {
	_, ok := m.sessions.Load(id)
	return ok
}

func (m *Manager) clientLock(key string) *sync.Mutex {
	mu, _ := m.clientLocks.LoadOrStore(key, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func hopsClientKey(hops []Endpoint, proxy ProxyConfig) string {
	if len(hops) == 0 {
		return ""
	}
	key := hops[len(hops)-1].ClientKey()
	for i := 0; i < len(hops)-1; i++ {
		key += fmt.Sprintf("via:%s@%s", hops[i].User, hops[i].Addr())
	}
	key += "|proxy:" + proxy.cacheKey()
	return key
}

func (m *Manager) Connect(hops []Endpoint, proxy ProxyConfig) (string, error) {
	if len(hops) == 0 {
		return "", fmt.Errorf("empty hop list")
	}
	target := hops[len(hops)-1]
	m.logger.Debug("Connecting to SSH server", zap.String("host", target.Host), zap.Int("port", target.Port))

	clientKey := hopsClientKey(hops, proxy)
	lock := m.clientLock(clientKey)
	lock.Lock()
	defer lock.Unlock()

	var client *cryptossh.Client
	if v, ok := m.clients.Load(clientKey); ok {
		m.logger.Debug("Using existing SSH client", zap.String("clientKey", clientKey))
		client = v.(*hopClient).target
	} else {
		hc, err := m.dialHops(hops, m.dialTimeout(), proxy)
		if err != nil {
			return "", err
		}
		m.clients.Store(clientKey, hc)
		client = hc.target
		m.startKeepAlive(clientKey, client)
		m.logger.Debug("ssh connect ok and stored in cache", zap.String("clientKey", clientKey))
	}

	sess := newSession(m, clientKey, client, target)
	m.sessions.Store(sess.ID, sess)
	return sess.ID, nil
}

func (m *Manager) TestConnect(hops []Endpoint, proxy ProxyConfig) error {
	if len(hops) == 0 {
		return fmt.Errorf("empty hop list")
	}
	target := hops[len(hops)-1]
	m.logger.Debug("Testing SSH connection", zap.String("host", target.Host), zap.Int("port", target.Port))
	hc, err := m.dialHops(hops, m.dialTimeout(), proxy)
	if err != nil {
		return err
	}
	defer hc.Close()
	m.logger.Debug("Test SSH connect successful", zap.String("host", target.Host), zap.Int("port", target.Port))
	return nil
}

func (m *Manager) Start(id string, cols, rows int) error {
	m.logger.Debug("Starting SSH connection", zap.String("id", id))
	sess, err := m.GetSession(id)
	if err != nil {
		return err
	}
	return sess.Start(cols, rows)
}

func (m *Manager) Resize(id string, cols, rows int) error {
	sess, err := m.GetSession(id)
	if err != nil {
		return err
	}
	return sess.Resize(cols, rows)
}

func (m *Manager) SendToSession(sessionID, command string) error {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("SSH session %s not found", sessionID)
	}
	if sess.Stdin == nil {
		return fmt.Errorf("stdin not available")
	}
	_, err = sess.Stdin.Write([]byte(command + "\n"))
	if err != nil {
		m.logger.Error("Failed to write to stdin", zap.Error(err))
	}
	return err
}

func (m *Manager) ActiveSessions() []map[string]any {
	sessions := make([]map[string]any, 0)
	m.sessions.Range(func(_, value any) bool {
		sess := value.(*Session)
		sessions = append(sessions, map[string]any{
			"id":        sess.ID,
			"clientKey": sess.ClientKey,
		})
		return true
	})
	return sessions
}

func (m *Manager) notifyClosed(id string, reason CloseReason) {
	if m.onClose != nil {
		m.onClose(id, reason)
		return
	}
	_ = m.CloseSession(id)
}

func (m *Manager) CloseAll() {
	m.logger.Debug("Close ssh all")
	m.sessions.Range(func(_, value any) bool {
		_ = value.(*Session).closePTY()
		return true
	})
	m.clients.Range(func(key, value any) bool {
		m.stopKeepAlive(key.(string))
		value.(*hopClient).Close()
		m.clients.Delete(key)
		return true
	})
	m.sessions = new(sync.Map)
}

func (m *Manager) CloseSession(id string) error {
	m.logger.Debug("CloseByID", zap.String("ID", id))
	sess, err := m.GetSession(id)
	if err != nil {
		return err
	}
	clientKey := sess.ClientKey
	_ = sess.closePTY()
	m.sessions.Delete(id)
	m.closeClientIfNoConnections(clientKey)
	return nil
}

func (m *Manager) closeClientIfNoConnections(clientKey string) {
	lock := m.clientLock(clientKey)
	lock.Lock()
	defer lock.Unlock()
	hasOther := false
	m.sessions.Range(func(_, value any) bool {
		if value.(*Session).ClientKey == clientKey {
			hasOther = true
			return false
		}
		return true
	})
	m.logger.Debug("closeClientIfNoConnections", zap.String("clientKey", clientKey), zap.Bool("hasOtherConnections", hasOther))
	if hasOther {
		return
	}
	m.stopKeepAlive(clientKey)
	if client, ok := m.clients.LoadAndDelete(clientKey); ok {
		client.(*hopClient).Close()
	}
}

func (m *Manager) SetHostKeyDecision(host string, accept bool) error {
	return m.hostKey.decide(host, accept)
}

func newSession(m *Manager, clientKey string, client *cryptossh.Client, target Endpoint) *Session {
	sc := &Session{
		manager:        m,
		ClientKey:      clientKey,
		client:         client,
		ID:             utils.GenerateRandomID(),
		OutputChan:     make(chan []byte, 200),
		stopOutput:     make(chan struct{}),
		outputBuffSize: 1024 * 10,
		forwardAgent:   target.ForwardAgent,
		startupCmd:     target.StartupCmd,
		env:            target.Env,
		term:           target.Term,
	}
	if m.zmodemPicker != nil {
		sc.zmFilter = zmodem.NewFilter(sc.ID, zmodem.Deps{
			Picker:   m.zmodemPicker,
			Logger:   m.logger,
			Annotate: sc.annotateOutput,
		})
	}
	return sc
}

// annotateOutput injects local terminal text (Zmodem progress). Non-blocking.
func (sc *Session) annotateOutput(data []byte) {
	if len(data) == 0 {
		return
	}
	sc.teeOutput(data)
	select {
	case sc.OutputChan <- data:
	case <-sc.stopOutput:
	default:
		sc.manager.logger.Debug("annotateOutput dropped: output buffer full", zap.String("id", sc.ID))
	}
}

func (sc *Session) Start(cols, rows int) error {
	sc.manager.logger.Debug("Starting SSH session", zap.String("id", sc.ID), zap.String("size", fmt.Sprintf("%dx%d", cols, rows)))
	if sc.forwardAgent {
		if err := sc.manager.enableAgentForward(sc.ClientKey); err != nil {
			sc.manager.logger.Error("SSH agent forwarding enable failed", zap.Error(err), zap.String("id", sc.ID))
			return fmt.Errorf("SSH agent 转发失败: %w", err)
		}
	}
	var err error
	sc.session, err = sc.client.NewSession()
	if err != nil {
		return fmt.Errorf("Start session fail")
	}
	if sc.forwardAgent {
		if err = agent.RequestAgentForwarding(sc.session); err != nil {
			sc.manager.logger.Error("SSH agent forwarding request denied", zap.Error(err), zap.String("id", sc.ID))
			return sc.failStart(fmt.Errorf("SSH agent 转发请求被拒绝: %w", err))
		}
	}
	term := sc.term
	if term == "" {
		term = defaultTermType
	}
	if err = sc.session.RequestPty(term, rows, cols, cryptossh.TerminalModes{
		cryptossh.ECHO:          1,
		cryptossh.TTY_OP_ISPEED: 14400,
		cryptossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return sc.failStart(err)
	}
	// Best-effort Setenv (often rejected without AcceptEnv). Env is also
	// applied via stdin after the shell prompt is ready.
	for k, v := range filterEnv(sc.env) {
		if setErr := sc.session.Setenv(k, v); setErr != nil {
			sc.manager.logger.Debug("SSH Setenv skipped",
				zap.String("id", sc.ID), zap.String("key", k), zap.Error(setErr))
		}
	}
	if err = sc.startInput(); err != nil {
		return sc.failStart(err)
	}
	if err = sc.startOutput(); err != nil {
		return sc.failStart(err)
	}
	if err = sc.session.Shell(); err != nil {
		return sc.failStart(err)
	}
	if payload := buildStdinBootstrap(sc.env, sc.startupCmd); payload != "" {
		stdin := sc.Stdin
		id := sc.ID
		logger := sc.manager.logger
		system.SafeGo(func() {
			sc.waitShellReady(shellReadyTimeout)
			sc.closeMu.Lock()
			closed := sc.isClosed
			sc.closeMu.Unlock()
			if closed || stdin == nil {
				return
			}
			if writeErr := writeStdinLine(stdin, payload); writeErr != nil {
				logger.Warn("session stdin bootstrap failed",
					zap.String("id", id), zap.Error(writeErr))
			}
		})
	}
	pty := sc.session
	go func() {
		waitErr := pty.Wait()
		sc.closeMu.Lock()
		alreadyClosed := sc.isClosed
		sc.closeMu.Unlock()
		// CloseByID/closePTY already ran — do not emit a second close notification.
		if alreadyClosed {
			return
		}
		reason := closeReasonFromWait(waitErr)
		if waitErr != nil {
			sc.manager.logger.Debug("SSH session ended with error:",
				zap.String("msg", waitErr.Error()),
				zap.String("id", sc.ID),
				zap.String("reason", string(reason)),
			)
		} else {
			sc.manager.logger.Debug("SSH session ended",
				zap.String("ID", sc.ID),
				zap.String("reason", string(reason)),
			)
		}
		sc.manager.notifyClosed(sc.ID, reason)
	}()
	return nil
}

func (sc *Session) failStart(err error) error {
	if sc.session != nil {
		_ = sc.session.Close()
		sc.session = nil
	}
	sc.Stdin = nil
	return err
}

func (sc *Session) startOutput() error {
	stdout, err := sc.session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := sc.session.StderrPipe()
	if err != nil {
		return err
	}
	sc.outputWg.Go(sc.readFromPipe(stdout, "stdout"))
	sc.outputWg.Go(sc.readFromPipe(stderr, "stderr"))
	return nil
}

func (sc *Session) readFromPipe(pipe io.Reader, pipeName string) func() {
	return func() {
		defer system.RecoverFromPanic()
		buf := make([]byte, sc.outputBuffSize)
		for {
			n, err := pipe.Read(buf)
			if err != nil {
				if err == io.EOF {
					sc.manager.logger.Debug(fmt.Sprintf("%s EOF reached", pipeName), zap.String("id", sc.ID))
					return
				}
				sc.manager.logger.Error(fmt.Sprintf("Error reading from %s:", pipeName), zap.Error(err), zap.String("id", sc.ID))
				return
			}
			if n > 0 {
				sc.noteOutput()
				data := make([]byte, n)
				copy(data, buf[:n])
				if sc.zmFilter != nil {
					data = sc.zmFilter.FeedOut(data)
					if len(data) == 0 {
						continue
					}
				}
				sc.teeOutput(data)
				select {
				case sc.OutputChan <- data:
				case <-sc.stopOutput:
					return
				}
			}
		}
	}
}

func (sc *Session) closeOutputChan() {
	sc.outputOnce.Do(func() {
		close(sc.OutputChan)
	})
}

func (sc *Session) startInput() error {
	stdin, err := sc.session.StdinPipe()
	if err != nil {
		return err
	}
	if sc.zmFilter != nil {
		sc.zmFilter.BindStdin(stdin)
		sc.Stdin = sc.zmFilter.WrapStdin()
	} else {
		sc.Stdin = stdin
	}
	return nil
}

func (sc *Session) closePTY() error {
	sc.closeMu.Lock()
	sc.manager.logger.Debug("Closing SSH connection", zap.String("ID", sc.ID))
	if sc.isClosed {
		sc.closeMu.Unlock()
		return nil
	}
	sc.isClosed = true
	if sc.zmFilter != nil {
		sc.zmFilter.Reset()
	}
	// Close log under locks; emit after unlock so callbacks cannot deadlock on closeMu.
	sc.logMu.Lock()
	logID, logStopped := sc.closeLogFileLocked()
	sc.logMu.Unlock()
	sc.stopOutputOnce.Do(func() { close(sc.stopOutput) })
	if sc.session != nil {
		_ = sc.session.Signal(cryptossh.SIGTERM)
		_ = sc.session.Close()
		sc.session = nil
	}
	sc.closeMu.Unlock()
	if logStopped {
		sc.emitLogStopped(logID, SessionLogStopReasonClosed)
	}
	go func() {
		sc.outputWg.Wait()
		sc.closeOutputChan()
		sc.manager.logger.Debug("Output channel closed in Close()", zap.String("ID", sc.ID))
	}()
	return nil
}

func (sc *Session) Resize(cols, rows int) error {
	sc.manager.logger.Debug("Resizing SSH session", zap.String("id", sc.ID), zap.Int("cols", cols), zap.Int("rows", rows))
	if sc.session == nil {
		return fmt.Errorf("no active session")
	}
	return sc.session.WindowChange(rows, cols)
}

func (sc *Session) noteOutput() {
	sc.outputMu.Lock()
	sc.gotOutput = true
	sc.lastOutputAt = time.Now()
	sc.outputMu.Unlock()
}

// waitShellReady waits until remote output has gone quiet after the first
// chunk (MOTD/prompt), or until timeout — then stdin injection is safer.
func (sc *Session) waitShellReady(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sc.closeMu.Lock()
		closed := sc.isClosed
		sc.closeMu.Unlock()
		if closed {
			return
		}
		sc.outputMu.Lock()
		got := sc.gotOutput
		last := sc.lastOutputAt
		sc.outputMu.Unlock()
		if got && time.Since(last) >= shellReadyQuiet {
			return
		}
		time.Sleep(shellReadyPoll)
	}
}

// filterEnv drops empty keys and TERM (PTY term owns TERM).
func filterEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		k = strings.TrimSpace(k)
		if k == "" || strings.EqualFold(k, "TERM") {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildStdinBootstrap builds one shell line: exports + optional startup command.
// Empty when neither is configured.
func buildStdinBootstrap(env map[string]string, startup string) string {
	env = filterEnv(env)
	startup = strings.TrimSpace(startup)
	if len(env) == 0 && startup == "" {
		return ""
	}
	var parts []string
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, "export "+bashSingleQuote(k)+"="+bashSingleQuote(env[k]))
	}
	if startup != "" {
		parts = append(parts, startup)
	}
	return strings.Join(parts, "; ")
}

func bashSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func writeStdinLine(stdin io.Writer, line string) error {
	if stdin == nil {
		return fmt.Errorf("stdin not available")
	}
	_, err := io.WriteString(stdin, line+"\n")
	return err
}
