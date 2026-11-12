// Package vaultprompt provides a reusable masked password dialog and
// vault unlock flow. DialogPrompt in go-gui has no password mask, so
// this uses DialogCustom with an IsPassword Input.
package vaultprompt

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	inputLeafID = "vault-unlock-input"
	// dialogWidth is wider than theme Dialog MaxWidth (300) so the
	// password field is usable.
	dialogWidth float32 = 420
)

// PromptCfg configures a masked password dialog.
type PromptCfg struct {
	Title       string
	Body        string
	Placeholder string
	Width       float32 // 0 → dialogWidth
	// CancelLabel defaults to common.cancel.
	CancelLabel string
	// EscapeDisabled stops Escape from dismissing (startup hard gate).
	EscapeDisabled bool
	// InlineFail shows Validate errors in the dialog body.
	InlineFail bool
	// Validate runs before dismiss. Non-nil keeps the dialog open and
	// calls OnFail when set.
	Validate func(password string) error
	OnFail   func(w *gui.Window, err error)
	OnOK     func(password string, w *gui.Window)
	OnCancel func(w *gui.Window)
}

// Prompt shows a modal password field (masked). Escape / Cancel dismiss
// unless EscapeDisabled is set.
func Prompt(w *gui.Window, cfg PromptCfg) {
	title := strings.TrimSpace(cfg.Title)
	if title == "" {
		title = i18n.T("password.title")
	}
	placeholder := cfg.Placeholder
	if placeholder == "" {
		placeholder = i18n.T("password.placeholder")
	}
	width := cfg.Width
	if width <= 0 {
		width = dialogWidth
	}
	cancelLabel := strings.TrimSpace(cfg.CancelLabel)
	if cancelLabel == "" {
		cancelLabel = i18n.T("common.cancel")
	}

	reply := ""
	failMsg := ""
	w.Dialog(gui.DialogCfg{
		DialogType:     gui.DialogCustom,
		Title:          title,
		Width:          width,
		FocusID:        inputLeafID,
		EscapeDisabled: cfg.EscapeDisabled,
		OnCancelNo: func(win *gui.Window) {
			if cfg.OnCancel != nil {
				cfg.OnCancel(win)
			}
		},
		CustomView: func(*gui.Window) gui.View {
			return promptBody(&reply, &failMsg, cfg, placeholder, cancelLabel)
		},
	})
}

func promptBody(reply, failMsg *string, cfg PromptCfg, placeholder, cancelLabel string) gui.View {
	th := gui.CurrentTheme()
	content := make([]gui.View, 0, 5)
	if cfg.Body != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      cfg.Body,
			TextStyle: th.TextStyleBody,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}))
	}
	if cfg.InlineFail && *failMsg != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      *failMsg,
			TextStyle: th.TextStyleError,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}))
	}
	content = append(content, gui.Input(gui.InputCfg{
		ID:          inputLeafID,
		Text:        *reply,
		Placeholder: placeholder,
		IsPassword:  true,
		Sizing:      gui.FillFit,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			*reply = v
			if cfg.InlineFail {
				*failMsg = ""
			}
			ctx.Consume()
		},
		OnEnter: func(ctx gui.EventCtx) {
			ctx.Consume()
			submitPassword(ctx.Window, reply, failMsg, cfg)
		},
	}))
	content = append(content, gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		HAlign:  gui.HAlignEnd,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:       "vault-unlock-ok",
				Variant:  gui.ButtonPrimary,
				Label:    i18n.T("common.ok"),
				Disabled: strings.TrimSpace(*reply) == "",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					submitPassword(ctx.Window, reply, failMsg, cfg)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "vault-unlock-cancel",
				Variant: gui.ButtonSecondary,
				Label:   cancelLabel,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctx.Window.DialogDismiss()
					if cfg.OnCancel != nil {
						cfg.OnCancel(ctx.Window)
					}
				},
			}),
		},
	})))
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: content,
	}))
}

func submitPassword(w *gui.Window, reply, failMsg *string, cfg PromptCfg) {
	text := strings.TrimSpace(*reply)
	if text == "" {
		return // OK is disabled when empty; Enter should match
	}
	if cfg.Validate != nil {
		if err := cfg.Validate(text); err != nil {
			if cfg.InlineFail {
				*failMsg = failText(err)
			}
			if cfg.OnFail != nil {
				cfg.OnFail(w, err)
			}
			return
		}
	}
	w.DialogDismiss()
	if cfg.OnOK != nil {
		cfg.OnOK(text, w)
	}
}

func failText(err error) string {
	if ue, ok := usererr.As(err); ok && ue.Code != "" {
		return i18n.T(ue.Code)
	}
	return err.Error()
}

// UnlockCfg configures vault unlock.
type UnlockCfg struct {
	Vault       *secret.Vault
	UserDataDir string
	Reason      string
	Title       string // empty → password.unlock_title
	// Probe validates the password against stored ciphertexts when no
	// verifier exists yet. Pass only when HasEncryptedSecrets is true;
	// nil means first-time set (any non-empty password accepted).
	Probe      func(password string) bool
	OnUnlocked func(w *gui.Window)
	OnCancel   func(w *gui.Window)
	OnFail     func(w *gui.Window, err error)
}

// Unlock runs OnUnlocked once the vault has a password. If locked,
// shows a masked password dialog, verifies via verifier / Probe / first-time
// set, then sets the vault. Cancel dismisses only (lazy path).
func Unlock(w *gui.Window, cfg UnlockCfg) {
	if _, err := cfg.Vault.Get(); err == nil {
		if cfg.OnUnlocked != nil {
			cfg.OnUnlocked(w)
		}
		return
	}

	title := strings.TrimSpace(cfg.Title)
	if title == "" {
		title = i18n.T("password.unlock_title")
	}
	dir := cfg.UserDataDir
	Prompt(w, PromptCfg{
		Title:       title,
		Body:        cfg.Reason,
		Placeholder: i18n.T("password.placeholder"),
		OnCancel:    cfg.OnCancel,
		OnFail:      cfg.OnFail,
		Validate:    unlockValidate(dir, cfg.Probe),
		OnOK: func(text string, win *gui.Window) {
			finishUnlock(cfg.Vault, dir, text, cfg.OnUnlocked, win)
		},
	})
}

// StartupGateCfg configures the startup hard-gate unlock dialog.
type StartupGateCfg struct {
	Vault       *secret.Vault
	UserDataDir string
	// Probe validates when no verifier exists; only set when secrets exist.
	Probe      func(password string) bool
	OnUnlocked func(w *gui.Window)
	OnQuit     func(w *gui.Window)
}

// StartupGate shows a non-dismissible-by-Escape unlock dialog. Quit closes
// the app (via OnQuit); wrong passwords stay on the dialog with inline error.
func StartupGate(w *gui.Window, cfg StartupGateCfg) {
	dir := cfg.UserDataDir
	Prompt(w, PromptCfg{
		Title:          i18n.T("password.unlock_title"),
		Body:           i18n.T("password.unlock_message"),
		Placeholder:    i18n.T("password.placeholder"),
		CancelLabel:    i18n.T("common.quit"),
		EscapeDisabled: true,
		InlineFail:     true,
		OnCancel:       cfg.OnQuit,
		Validate:       unlockValidate(dir, cfg.Probe),
		OnOK: func(text string, win *gui.Window) {
			finishUnlock(cfg.Vault, dir, text, cfg.OnUnlocked, win)
		},
	})
}

func unlockValidate(dir string, probe func(string) bool) func(string) error {
	return func(text string) error {
		ct, err := secret.LoadVerifierCiphertext(dir)
		if err != nil {
			return err
		}
		if ct != "" {
			if err := secret.CheckPassword(ct, text); err != nil {
				return usererr.Coded("error.password_invalid", "Invalid password")
			}
			return nil
		}
		if probe != nil && !probe(text) {
			return usererr.Coded("error.password_invalid", "Invalid password")
		}
		return nil
	}
}

func finishUnlock(vault *secret.Vault, dir, text string, onUnlocked func(*gui.Window), win *gui.Window) {
	vault.Set(text)
	_ = secret.EnsureVerifier(dir, text)
	if onUnlocked != nil {
		onUnlocked(win)
	}
}
