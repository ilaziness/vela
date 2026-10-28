package ssh

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

type fakeKeyboardPrompter struct {
	fn        func(KeyboardChallenge) error
	calls     int
	dismissed []string
}

func (f *fakeKeyboardPrompter) Prompt(c KeyboardChallenge) error {
	f.calls++
	if f.fn != nil {
		return f.fn(c)
	}
	return nil
}

func (f *fakeKeyboardPrompter) Dismiss(id string) {
	f.dismissed = append(f.dismissed, id)
}

type deadlineSpy struct {
	net.Conn
	deadlines []time.Time
}

func (c *deadlineSpy) SetDeadline(t time.Time) error {
	c.deadlines = append(c.deadlines, t)
	return c.Conn.SetDeadline(t)
}

func newDeadlineSpy(t *testing.T) (*deadlineSpy, func()) {
	t.Helper()
	a, b := net.Pipe()
	spy := &deadlineSpy{Conn: a}
	return spy, func() {
		_ = a.Close()
		_ = b.Close()
	}
}

func TestKeyboardStoreAnswerCancelAndFinish(t *testing.T) {
	s := newKeyboardStore()
	if err := s.answer("missing", nil); err == nil {
		t.Fatal("expected error when no pending prompt")
	}
	ch, err := s.begin("a", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.begin("a", 1); err == nil {
		t.Fatal("expected duplicate prompt to fail")
	}
	if err := s.answer("a", []string{"one", "two"}); err == nil || !strings.Contains(err.Error(), "答案数量不匹配") {
		t.Fatalf("err = %v", err)
	}
	if err := s.answer("a", []string{"otp"}); err != nil {
		t.Fatal(err)
	}
	if err := s.answer("a", []string{"other"}); err == nil {
		t.Fatal("expected second answer to fail")
	}
	if err := s.cancel("a"); err == nil {
		t.Fatal("expected cancel after answer to fail")
	}
	select {
	case got := <-ch:
		if got.cancel || len(got.answers) != 1 || got.answers[0] != "otp" {
			t.Fatalf("result = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for answer")
	}
	s.finish("a")
	if err := s.cancel("a"); err == nil {
		t.Fatal("expected cancel after finish to fail")
	}

	ch, err = s.begin("b", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cancel("b"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-ch:
		if !got.cancel {
			t.Fatal("expected cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cancel")
	}
	s.finish("b")
}

func TestAutoPasswordAnswer(t *testing.T) {
	ep := Endpoint{Host: "h", User: "u", Password: "secret"}
	prompter := &fakeKeyboardPrompter{fn: func(KeyboardChallenge) error {
		t.Fatal("password prompt should not reach the UI")
		return nil
	}}
	m := NewManager(nil, "", nil, prompter)
	var used bool
	got, err := m.answerKeyboardChallenge(ep, &used, func() net.Conn {
		t.Fatal("auto answer should not touch the connection")
		return nil
	}, time.Second, time.Second, "", "", []string{"Password:"}, []bool{false})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "secret" || !used || prompter.calls != 0 || len(prompter.dismissed) != 0 {
		t.Fatalf("answers=%v used=%v calls=%d dismissed=%v", got, used, prompter.calls, prompter.dismissed)
	}

	cases := []struct {
		prompt string
		used   bool
		want   bool
	}{
		{"password:", false, true},
		{"请输入密码", false, true},
		{"Verification code:", false, false},
		{"One-time password:", false, false},
		{"一次性密码", false, false},
		{"Password:", true, false},
	}
	for _, tc := range cases {
		answers, ok := autoPasswordAnswer(ep, tc.used, []string{tc.prompt}, []bool{false})
		if ok != tc.want {
			t.Fatalf("prompt %q ok=%v want %v", tc.prompt, ok, tc.want)
		}
		if tc.want && answers[0] != "secret" {
			t.Fatalf("prompt %q answers=%v", tc.prompt, answers)
		}
	}
	if _, ok := autoPasswordAnswer(ep, false, []string{"Password:"}, []bool{true}); ok {
		t.Fatal("echoing prompt must not auto answer")
	}
	if _, ok := autoPasswordAnswer(ep, false, []string{"Password:", "Code:"}, []bool{false, false}); ok {
		t.Fatal("multiple questions must not auto answer")
	}
	if _, ok := autoPasswordAnswer(Endpoint{}, false, []string{"Password:"}, []bool{false}); ok {
		t.Fatal("empty password must not auto answer")
	}
}

func TestKeyboardChallengePromptAnswer(t *testing.T) {
	spy, cleanup := newDeadlineSpy(t)
	defer cleanup()
	var m *Manager
	prompter := &fakeKeyboardPrompter{}
	m = NewManager(nil, "", nil, prompter)
	var prompted string
	prompter.fn = func(c KeyboardChallenge) error {
		prompted = c.ID
		if c.Host != "h" || c.User != "u" || c.Name != "Duo" || len(c.Questions) != 1 || c.Questions[0].Echo {
			t.Fatalf("challenge = %+v", c)
		}
		go func() {
			if err := m.AnswerKeyboardInteractive(c.ID, []string{"bad", "count"}); err == nil {
				t.Error("expected answer count error")
			}
			if err := m.AnswerKeyboardInteractive(c.ID, []string{"654321"}); err != nil {
				t.Error(err)
			}
		}()
		return nil
	}
	var used bool
	got, err := m.answerKeyboardChallenge(
		Endpoint{Host: "h", User: "u", Password: "secret"},
		&used,
		func() net.Conn { return spy },
		5*time.Second,
		time.Second,
		"Duo",
		"Enter the code",
		[]string{"Verification code:"},
		[]bool{false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if used || len(got) != 1 || got[0] != "654321" {
		t.Fatalf("answers=%v used=%v", got, used)
	}
	if len(spy.deadlines) != 2 || !spy.deadlines[0].IsZero() || spy.deadlines[1].IsZero() {
		t.Fatalf("deadlines = %v", spy.deadlines)
	}
	if len(prompter.dismissed) != 1 || prompter.dismissed[0] != prompted {
		t.Fatalf("dismissed = %v, prompted = %s", prompter.dismissed, prompted)
	}
}

func TestKeyboardChallengeCancelAndTimeout(t *testing.T) {
	var m *Manager
	prompter := &fakeKeyboardPrompter{}
	m = NewManager(nil, "", nil, prompter)
	prompter.fn = func(c KeyboardChallenge) error {
		go func() {
			if err := m.CancelKeyboardInteractive(c.ID); err != nil {
				t.Error(err)
			}
		}()
		return nil
	}
	_, err := m.answerKeyboardChallenge(Endpoint{Host: "h"}, nil, nil, time.Second, time.Second, "", "info", nil, nil)
	if !errors.Is(err, ErrAuthCancelled) {
		t.Fatalf("err = %v", err)
	}
	if len(prompter.dismissed) != 1 {
		t.Fatalf("dismissed after cancel = %v", prompter.dismissed)
	}

	prompter.fn = func(KeyboardChallenge) error { return nil }
	_, err = m.answerKeyboardChallenge(Endpoint{Host: "h"}, nil, nil, time.Second, 20*time.Millisecond, "", "", []string{"Code:"}, []bool{false})
	if !errors.Is(err, ErrAuthTimeout) {
		t.Fatalf("err = %v", err)
	}
	if len(prompter.dismissed) != 2 {
		t.Fatalf("dismissed after timeout = %v", prompter.dismissed)
	}
}

func TestKeyboardChallengeEmptyRound(t *testing.T) {
	prompter := &fakeKeyboardPrompter{fn: func(KeyboardChallenge) error {
		t.Fatal("empty round should not prompt")
		return nil
	}}
	m := NewManager(nil, "", nil, prompter)
	got, err := m.answerKeyboardChallenge(Endpoint{Host: "h"}, nil, func() net.Conn {
		t.Fatal("empty round should not touch the connection")
		return nil
	}, time.Second, time.Second, "  ", "\n", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || prompter.calls != 0 {
		t.Fatalf("answers=%v calls=%d", got, prompter.calls)
	}
}

func TestKeyboardChallengeRejectsBadRounds(t *testing.T) {
	prompter := &fakeKeyboardPrompter{fn: func(KeyboardChallenge) error {
		t.Fatal("invalid round should not prompt")
		return nil
	}}
	m := NewManager(nil, "", nil, prompter)
	_, err := m.answerKeyboardChallenge(Endpoint{}, nil, nil, time.Second, time.Second, "", "", []string{"Password:"}, nil)
	if err == nil || !strings.Contains(err.Error(), "格式无效") {
		t.Fatalf("err = %v", err)
	}
	questions := make([]string, maxKeyboardQuestions+1)
	echos := make([]bool, len(questions))
	_, err = m.answerKeyboardChallenge(Endpoint{}, nil, nil, time.Second, time.Second, "", "", questions, echos)
	if err == nil || !strings.Contains(err.Error(), "问题过多") {
		t.Fatalf("err = %v", err)
	}
	if prompter.calls != 0 {
		t.Fatalf("calls = %d", prompter.calls)
	}
}

func TestKeyboardChallengePromptError(t *testing.T) {
	prompter := &fakeKeyboardPrompter{fn: func(KeyboardChallenge) error {
		return fmt.Errorf("emit failed")
	}}
	m := NewManager(nil, "", nil, prompter)
	_, err := m.answerKeyboardChallenge(Endpoint{Host: "h"}, nil, func() net.Conn {
		t.Fatal("prompt error should not touch the connection")
		return nil
	}, time.Second, time.Second, "", "", []string{"Code:"}, []bool{true})
	if err == nil || !strings.Contains(err.Error(), "emit failed") {
		t.Fatalf("err = %v", err)
	}
	if len(prompter.dismissed) != 0 {
		t.Fatalf("dismissed = %v", prompter.dismissed)
	}
}
