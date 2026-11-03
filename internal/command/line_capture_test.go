package command

import (
	"testing"
)

func TestLineCaptureSimpleCommand(t *testing.T) {
	c := &LineCapture{}
	got := c.FeedInput([]byte("echo hi\r"))
	if len(got) != 1 || got[0] != "echo hi" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureBackspace(t *testing.T) {
	c := &LineCapture{}
	got := c.FeedInput([]byte("echx\x08o hi\n"))
	if len(got) != 1 || got[0] != "echo hi" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureCtrlU(t *testing.T) {
	c := &LineCapture{}
	got := c.FeedInput([]byte("wrong\x15ls\r"))
	if len(got) != 1 || got[0] != "ls" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureCtrlW(t *testing.T) {
	c := &LineCapture{}
	got := c.FeedInput([]byte("ls /tmp\x17/var\r"))
	if len(got) != 1 || got[0] != "ls /var" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureSkipEmpty(t *testing.T) {
	c := &LineCapture{}
	got := c.FeedInput([]byte("\r\n  \n"))
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCapturePasswordPrompt(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("[sudo] password for user: "))
	got := c.FeedInput([]byte("s3cret\r"))
	if len(got) != 0 {
		t.Fatalf("password must not be recorded, got %#v", got)
	}
	got = c.FeedInput([]byte("whoami\r"))
	if len(got) != 1 || got[0] != "whoami" {
		t.Fatalf("after password got %#v", got)
	}
}

func TestLineCaptureSecretClearedByNewPrompt(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("Password: "))
	// Prompt cancelled / timed out; shell returns a normal prompt without Enter on secret line.
	c.FeedOutput([]byte("\r\nuser@host:~$ "))
	got := c.FeedInput([]byte("ls\r"))
	if len(got) != 1 || got[0] != "ls" {
		t.Fatalf("command after cleared prompt got %#v", got)
	}
}

func TestLineCaptureSecretNotRearmedBySamePrompt(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("Password: "))
	got := c.FeedInput([]byte("s3cret\r"))
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
	// ANSI-only output keeps the same last line; must not re-arm after Enter cleared the flag.
	c.FeedOutput([]byte("\x1b[0m"))
	got = c.FeedInput([]byte("whoami\r"))
	if len(got) != 1 || got[0] != "whoami" {
		t.Fatalf("must not re-arm secret on same prompt line, got %#v", got)
	}
}

func TestLineCaptureChinesePasswordPrompt(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("请输入密码："))
	got := c.FeedInput([]byte("密文\r"))
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureIgnoreCSI(t *testing.T) {
	c := &LineCapture{}
	// Arrow-up CSI then type command
	got := c.FeedInput([]byte("\x1b[Aecho ok\r"))
	if len(got) != 1 || got[0] != "echo ok" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureUTF8(t *testing.T) {
	c := &LineCapture{}
	got := c.FeedInput([]byte("echo 你好\r"))
	if len(got) != 1 || got[0] != "echo 你好" {
		t.Fatalf("got %#v", got)
	}
	// backspace one rune
	c = &LineCapture{}
	got = c.FeedInput([]byte("ab好\x08c\r"))
	if len(got) != 1 || got[0] != "abc" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCapturePasswordWithANSI(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("\x1b[01;31mPassword:\x1b[0m "))
	got := c.FeedInput([]byte("x\r"))
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureCRLF(t *testing.T) {
	c := &LineCapture{}
	got := c.FeedInput([]byte("pwd\r\n"))
	if len(got) != 1 || got[0] != "pwd" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCaptureAltScreenIgnoresKeys(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("\x1b[?1049h"))
	got := c.FeedInput([]byte("G\r"))
	if len(got) != 0 {
		t.Fatalf("G in alt screen must not record, got %#v", got)
	}
	got = c.FeedInput([]byte("ihello\r"))
	if len(got) != 0 {
		t.Fatalf("insert text in alt screen must not record, got %#v", got)
	}
	c.FeedOutput([]byte("\x1b[?1049l"))
	got = c.FeedInput([]byte("ls\r"))
	if len(got) != 1 || got[0] != "ls" {
		t.Fatalf("after leave alt screen got %#v", got)
	}
}

func TestLineCaptureAltScreenSplitCSI(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("\x1b[?1049"))
	c.FeedOutput([]byte("h"))
	got := c.FeedInput([]byte("G\r"))
	if len(got) != 0 {
		t.Fatalf("split CSI enter must ignore keys, got %#v", got)
	}
	c.FeedOutput([]byte("\x1b[?1049"))
	c.FeedOutput([]byte("l"))
	got = c.FeedInput([]byte("pwd\r"))
	if len(got) != 1 || got[0] != "pwd" {
		t.Fatalf("split CSI leave got %#v", got)
	}
}

func TestLineCaptureAltScreenModes(t *testing.T) {
	for _, seq := range []string{"\x1b[?47h", "\x1b[?1047h", "\x1b[?1;1049h"} {
		c := &LineCapture{}
		c.FeedOutput([]byte(seq))
		got := c.FeedInput([]byte("x\r"))
		if len(got) != 0 {
			t.Fatalf("%q must enter alt screen, got %#v", seq, got)
		}
	}
}

func TestLineCaptureAltScreenClearsPendingBuf(t *testing.T) {
	c := &LineCapture{}
	_ = c.FeedInput([]byte("partial"))
	c.FeedOutput([]byte("\x1b[?1049h"))
	c.FeedOutput([]byte("\x1b[?1049l"))
	got := c.FeedInput([]byte("\r"))
	if len(got) != 0 {
		t.Fatalf("buf should be cleared on alt enter, got %#v", got)
	}
}

func TestLineCaptureAltScreenIgnoresPasswordInRedraw(t *testing.T) {
	c := &LineCapture{}
	c.FeedOutput([]byte("\x1b[?1049h"))
	c.FeedOutput([]byte("password = secret\r\n"))
	c.FeedOutput([]byte("\x1b[?1049l"))
	c.FeedOutput([]byte("user@host:~$ "))
	got := c.FeedInput([]byte("ls\r"))
	if len(got) != 1 || got[0] != "ls" {
		t.Fatalf("TUI output must not arm password skip, got %#v", got)
	}
}
