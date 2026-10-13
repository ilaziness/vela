package ssh

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ilaziness/vela/internal/utils"
)

const (
	keyboardChallengeTimeout = 2 * time.Minute
	maxKeyboardQuestions     = 16
)

// KeyboardQuestion 是服务器一轮 keyboard-interactive 里的单个提问。
type KeyboardQuestion struct {
	Prompt string `json:"prompt"`
	Echo   bool   `json:"echo"`
}

// KeyboardChallenge 是交给界面的一轮提问。同一轮认证里服务器可以多次提问，每轮使用新的 ID。
type KeyboardChallenge struct {
	ID          string             `json:"id"`
	Host        string             `json:"host"`
	User        string             `json:"user"`
	Name        string             `json:"name"`
	Instruction string             `json:"instruction"`
	Questions   []KeyboardQuestion `json:"questions"`
}

// KeyboardInteractivePrompter 把提问通知到界面。实现只负责通知，不解析 SSH 协议。
type KeyboardInteractivePrompter interface {
	Prompt(c KeyboardChallenge) error
	// Dismiss 通知界面关闭这一轮。回答、取消、超时后都会调用，避免其他窗口再取消同一次提问。
	Dismiss(id string)
}

type keyboardResult struct {
	answers []string
	cancel  bool
}

type keyboardPending struct {
	ch       chan keyboardResult
	expected int
	settled  bool
}

type keyboardStore struct {
	mu      sync.Mutex
	pending map[string]*keyboardPending
}

func newKeyboardStore() *keyboardStore {
	return &keyboardStore{pending: make(map[string]*keyboardPending)}
}

func (s *keyboardStore) begin(id string, expected int) (chan keyboardResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[id]; ok {
		return nil, fmt.Errorf("keyboard-interactive prompt already in progress")
	}
	ch := make(chan keyboardResult, 1)
	s.pending[id] = &keyboardPending{ch: ch, expected: expected}
	return ch, nil
}

func (s *keyboardStore) finish(id string) {
	s.mu.Lock()
	if p := s.pending[id]; p != nil {
		p.settled = true
	}
	delete(s.pending, id)
	s.mu.Unlock()
}

func (s *keyboardStore) answer(id string, answers []string) error {
	return s.settle(id, keyboardResult{answers: answers})
}

func (s *keyboardStore) cancel(id string) error {
	return s.settle(id, keyboardResult{cancel: true})
}

func (s *keyboardStore) settle(id string, result keyboardResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pending[id]
	if p == nil || p.settled {
		return fmt.Errorf("没有待回答的 keyboard-interactive 提问")
	}
	if !result.cancel && len(result.answers) != p.expected {
		return fmt.Errorf("keyboard-interactive 答案数量不匹配")
	}
	copied := make([]string, len(result.answers))
	copy(copied, result.answers)
	result.answers = copied
	p.settled = true
	p.ch <- result
	return nil
}

func (m *Manager) AnswerKeyboardInteractive(id string, answers []string) error {
	if m == nil || m.keyboard == nil {
		return fmt.Errorf("没有待回答的 keyboard-interactive 提问")
	}
	return m.keyboard.answer(id, answers)
}

func (m *Manager) CancelKeyboardInteractive(id string) error {
	if m == nil || m.keyboard == nil {
		return fmt.Errorf("没有待回答的 keyboard-interactive 提问")
	}
	return m.keyboard.cancel(id)
}

func (m *Manager) keyboardAuth(ep Endpoint, connFn func() net.Conn, handshakeTimeout time.Duration) cryptossh.AuthMethod {
	var passwordUsed bool
	return cryptossh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		return m.answerKeyboardChallenge(ep, &passwordUsed, connFn, handshakeTimeout, keyboardChallengeTimeout, name, instruction, questions, echos)
	})
}

func (m *Manager) answerKeyboardChallenge(
	ep Endpoint,
	passwordUsed *bool,
	connFn func() net.Conn,
	handshakeTimeout time.Duration,
	waitTimeout time.Duration,
	name, instruction string,
	questions []string,
	echos []bool,
) ([]string, error) {
	if len(echos) != len(questions) {
		return nil, fmt.Errorf("keyboard-interactive 提问格式无效")
	}
	if len(questions) > maxKeyboardQuestions {
		return nil, fmt.Errorf("keyboard-interactive 问题过多")
	}
	// 没有提问、标题和说明时直接回空应答。PAM 常在认证末尾发这种空轮次。
	if len(questions) == 0 && strings.TrimSpace(name) == "" && strings.TrimSpace(instruction) == "" {
		return []string{}, nil
	}
	// 只自动回答本跳的第一轮单个不回显密码提示，避免把登录密码填进后续验证码。
	if answers, ok := autoPasswordAnswer(ep, passwordUsed != nil && *passwordUsed, questions, echos); ok {
		if passwordUsed != nil {
			*passwordUsed = true
		}
		m.logger.Debug("keyboard-interactive auto password", zap.String("host", ep.Host))
		return answers, nil
	}
	if m.keyboardPrompter == nil || m.keyboard == nil {
		return nil, fmt.Errorf("keyboard-interactive prompt unavailable")
	}

	id := utils.GenerateRandomID()
	qs := make([]KeyboardQuestion, len(questions))
	for i, prompt := range questions {
		qs[i] = KeyboardQuestion{Prompt: prompt, Echo: echos[i]}
	}
	ch, err := m.keyboard.begin(id, len(questions))
	if err != nil {
		return nil, err
	}
	defer m.keyboard.finish(id)

	m.logger.Debug("keyboard-interactive challenge",
		zap.String("id", id),
		zap.String("host", ep.Host),
		zap.Int("questions", len(questions)),
	)
	if err := m.keyboardPrompter.Prompt(KeyboardChallenge{
		ID:          id,
		Host:        ep.Host,
		User:        ep.User,
		Name:        name,
		Instruction: instruction,
		Questions:   qs,
	}); err != nil {
		m.logger.Debug("keyboard-interactive prompt", zap.Error(err))
		return nil, err
	}
	defer m.keyboardPrompter.Dismiss(id)

	// 提问等待期间清掉握手 deadline，避免用户输入验证码时连接被拨号超时掐断。
	var conn net.Conn
	if connFn != nil {
		conn = connFn()
	}
	restore := m.pauseHandshakeDeadline(conn, handshakeTimeout)
	defer restore()

	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()
	select {
	case result := <-ch:
		if result.cancel {
			return nil, fmt.Errorf("keyboard-interactive 已取消")
		}
		return result.answers, nil
	case <-timer.C:
		return nil, fmt.Errorf("keyboard-interactive 等待超时")
	}
}

func (m *Manager) pauseHandshakeDeadline(conn net.Conn, handshakeTimeout time.Duration) func() {
	if conn == nil {
		return func() {}
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		m.logger.Debug("clear ssh deadline", zap.Error(err))
	}
	return func() {
		if handshakeTimeout <= 0 {
			return
		}
		if err := conn.SetDeadline(time.Now().Add(handshakeTimeout)); err != nil {
			m.logger.Debug("restore ssh deadline", zap.Error(err))
		}
	}
}

func autoPasswordAnswer(ep Endpoint, used bool, questions []string, echos []bool) ([]string, bool) {
	if used || ep.Password == "" || len(questions) != 1 || len(echos) != 1 || echos[0] {
		return nil, false
	}
	if !looksLikePasswordPrompt(questions[0]) {
		return nil, false
	}
	return []string{ep.Password}, true
}

func looksLikePasswordPrompt(prompt string) bool {
	if looksLikeSecondFactor(prompt) {
		return false
	}
	if strings.Contains(prompt, "密码") {
		return true
	}
	return strings.Contains(strings.ToLower(prompt), "password")
}

func looksLikeSecondFactor(prompt string) bool {
	lower := strings.ToLower(prompt)
	for _, word := range []string{"otp", "totp", "one-time", "onetime", "verification", "passcode", "duo", "2fa", "token", "code"} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	for _, word := range []string{"验证", "动态", "一次性"} {
		if strings.Contains(prompt, word) {
			return true
		}
	}
	return false
}
