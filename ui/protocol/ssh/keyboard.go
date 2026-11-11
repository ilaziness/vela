package sshui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/ssh"
)

// KeyboardPrompter presents keyboard-interactive challenges via go-gui dialogs.
type KeyboardPrompter struct {
	mu  sync.Mutex
	win *gui.Window
	mgr *ssh.Manager
}

// NewKeyboardPrompter creates a prompter bound to win and mgr.
func NewKeyboardPrompter(win *gui.Window, mgr *ssh.Manager) *KeyboardPrompter {
	return &KeyboardPrompter{win: win, mgr: mgr}
}

// Bind attaches the window and manager used for dialogs.
func (p *KeyboardPrompter) Bind(win *gui.Window, mgr *ssh.Manager) {
	p.mu.Lock()
	p.win = win
	if mgr != nil {
		p.mgr = mgr
	}
	p.mu.Unlock()
}

// SetWindow updates the window used for dialogs.
func (p *KeyboardPrompter) SetWindow(win *gui.Window) {
	p.Bind(win, nil)
}

// Prompt implements ssh.KeyboardInteractivePrompter.
func (p *KeyboardPrompter) Prompt(c ssh.KeyboardChallenge) error {
	p.mu.Lock()
	win := p.win
	mgr := p.mgr
	p.mu.Unlock()
	if win == nil || mgr == nil {
		return fmt.Errorf("keyboard prompter not ready")
	}
	if len(c.Questions) == 0 {
		return mgr.AnswerKeyboardInteractive(c.ID, nil)
	}

	answers := make([]string, len(c.Questions))
	var ask func(i int)
	ask = func(i int) {
		if i >= len(c.Questions) {
			_ = mgr.AnswerKeyboardInteractive(c.ID, answers)
			return
		}
		q := c.Questions[i]
		title := "SSH authentication"
		if c.Name != "" {
			title = c.Name
		}
		var b strings.Builder
		if c.Instruction != "" {
			b.WriteString(c.Instruction)
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s@%s\n%s", c.User, c.Host, q.Prompt)

		win.QueueCommand(func(w *gui.Window) {
			cfg := gui.DialogCfg{
				DialogType: gui.DialogPrompt,
				Title:      title,
				Body:       b.String(),
				OnOkYes: func(w *gui.Window) {
					// Reply is filled by dialog prompt path via OnReply.
				},
				OnReply: func(text string, _ *gui.Window) {
					answers[i] = text
					ask(i + 1)
				},
				OnCancelNo: func(w *gui.Window) {
					w.DialogDismiss()
					_ = mgr.CancelKeyboardInteractive(c.ID)
				},
			}
			if !q.Echo {
				// DialogPrompt has no password mask; keep as normal prompt for MVP.
				_ = q
			}
			w.Dialog(cfg)
		})
	}
	ask(0)
	return nil
}

// Dismiss implements ssh.KeyboardInteractivePrompter.
func (p *KeyboardPrompter) Dismiss(id string) {
	p.mu.Lock()
	win := p.win
	p.mu.Unlock()
	if win == nil {
		return
	}
	win.QueueCommand(func(w *gui.Window) {
		if w.DialogIsVisible() {
			w.DialogDismiss()
		}
	})
}
