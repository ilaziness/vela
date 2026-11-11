package sshui

import (
	"fmt"
	"sync"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/ssh"
)

// HostKeyPrompter shows a native confirm dialog and answers the manager.
type HostKeyPrompter struct {
	mu  sync.Mutex
	win *gui.Window
	mgr *ssh.Manager
}

// NewHostKeyPrompter creates a prompter bound to win and mgr.
func NewHostKeyPrompter(win *gui.Window, mgr *ssh.Manager) *HostKeyPrompter {
	return &HostKeyPrompter{win: win, mgr: mgr}
}

// Bind attaches the window and manager used for dialogs.
func (p *HostKeyPrompter) Bind(win *gui.Window, mgr *ssh.Manager) {
	p.mu.Lock()
	p.win = win
	if mgr != nil {
		p.mgr = mgr
	}
	p.mu.Unlock()
}

// SetWindow updates the window used for dialogs.
func (p *HostKeyPrompter) SetWindow(win *gui.Window) {
	p.Bind(win, nil)
}

// Prompt implements ssh.HostKeyPrompter.
// It returns after scheduling the dialog; the SSH dialer waits on SetHostKeyDecision.
func (p *HostKeyPrompter) Prompt(hp ssh.HostKeyPrompt) error {
	p.mu.Lock()
	win := p.win
	mgr := p.mgr
	p.mu.Unlock()
	if win == nil || mgr == nil {
		return fmt.Errorf("host key prompter not ready")
	}

	body := fmt.Sprintf("Host: %s\nAddress: %s\nType: %s\nFingerprint:\n%s",
		hp.Host, hp.Address, hp.KeyType, hp.Fingerprint)
	title := "Trust host key?"
	level := gui.AlertWarning
	if hp.Mismatch {
		title = "Host key changed!"
		level = gui.AlertCritical
		body = fmt.Sprintf("%s\n\nPrevious fingerprint:\n%s", body, hp.OldFingerprint)
	}

	win.QueueCommand(func(w *gui.Window) {
		w.NativeConfirmDialog(gui.NativeConfirmDialogCfg{
			Title: title,
			Body:  body,
			Level: level,
			OnDone: func(res gui.NativeAlertResult, _ *gui.Window) {
				_ = mgr.SetHostKeyDecision(hp.Host, res.Status == gui.DialogOK)
			},
		})
	})
	return nil
}
