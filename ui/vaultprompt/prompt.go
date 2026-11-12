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
	// Validate runs before dismiss. Non-nil keeps the dialog open and
	// calls OnFail when set.
	Validate func(password string) error
	OnFail   func(w *gui.Window, err error)
	OnOK     func(password string, w *gui.Window)
	OnCancel func(w *gui.Window)
}

// Prompt shows a modal password field (masked). Escape / Cancel dismiss.
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

	reply := ""
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      title,
		Width:      width,
		FocusID:    inputLeafID,
		OnCancelNo: func(win *gui.Window) {
			if cfg.OnCancel != nil {
				cfg.OnCancel(win)
			}
		},
		CustomView: func(*gui.Window) gui.View {
			return promptBody(&reply, cfg, placeholder)
		},
	})
}

func promptBody(reply *string, cfg PromptCfg, placeholder string) gui.View {
	th := gui.CurrentTheme()
	content := make([]gui.View, 0, 4)
	if cfg.Body != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      cfg.Body,
			TextStyle: th.TextStyleBody,
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
			ctx.Consume()
		},
		OnEnter: func(ctx gui.EventCtx) {
			ctx.Consume()
			submitPassword(ctx.Window, reply, cfg)
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
					submitPassword(ctx.Window, reply, cfg)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "vault-unlock-cancel",
				Variant: gui.ButtonSecondary,
				Label:   i18n.T("common.cancel"),
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

func submitPassword(w *gui.Window, reply *string, cfg PromptCfg) {
	text := strings.TrimSpace(*reply)
	if text == "" {
		return // OK is disabled when empty; Enter should match
	}
	if cfg.Validate != nil {
		if err := cfg.Validate(text); err != nil {
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

// UnlockCfg configures vault unlock.
type UnlockCfg struct {
	Vault       *secret.Vault
	UserDataDir string
	Reason      string
	Title       string // empty → password.unlock_title
	OnUnlocked  func(w *gui.Window)
	OnCancel    func(w *gui.Window)
	OnFail      func(w *gui.Window, err error)
}

// Unlock runs OnUnlocked once the vault has a password. If locked,
// shows a masked password dialog, verifies the stored verifier when
// present, then sets the vault.
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
		Validate: func(text string) error {
			if ct, err := secret.LoadVerifierCiphertext(dir); err == nil && ct != "" {
				if err := secret.CheckPassword(ct, text); err != nil {
					return usererr.Coded("error.password_invalid", "Invalid password")
				}
			}
			return nil
		},
		OnOK: func(text string, win *gui.Window) {
			cfg.Vault.Set(text)
			_ = secret.EnsureVerifier(dir, text)
			if cfg.OnUnlocked != nil {
				cfg.OnUnlocked(win)
			}
		},
	})
}
