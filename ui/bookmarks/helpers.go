package bookmarks

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/internal/vaultcheck"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/vaultprompt"
)

func formRow(id, label, text, placeholder string, password bool, onChange func(string)) gui.View {
	return gui.Input(gui.InputCfg{
		ID:          id,
		Label:       label,
		Text:        text,
		Placeholder: placeholder,
		IsPassword:  password,
		Sizing:      gui.FillFit,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			onChange(v)
			ctx.Consume()
		},
	})
}

func secretPlaceholder(hasStored bool, fallback string) string {
	if hasStored {
		return "••••••••"
	}
	return fallback
}

func pathBrowseRow(id, label, path, placeholder, browseLabel string, onBrowse func(gui.EventCtx)) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		HAlign:  gui.HAlignStart,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: th.TextStyleLabel}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Input(gui.InputCfg{
						ID:          id,
						Text:        path,
						Placeholder: placeholder,
						ReadOnly:    true,
						Sizing:      gui.FillFit,
					}),
					gui.Button(gui.ButtonCfg{
						ID:      id + "-browse",
						Variant: gui.ButtonSecondary,
						Label:   browseLabel,
						OnClick: onBrowse,
					}),
				},
			})),
		},
	}))
}

func actionsRight(btns ...gui.View) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		HAlign:  gui.HAlignEnd,
		Content: btns,
	}))
}

// primaryBusyBtn shows a spinner + loadingLabel while busy (go-gui has no Button.Loading).
func primaryBusyBtn(id, label, loadingLabel string, busy bool, onClick func(gui.EventCtx)) gui.View {
	th := gui.CurrentTheme()
	cfg := gui.ButtonCfg{
		ID:       id,
		Variant:  gui.ButtonPrimary,
		Disabled: busy,
		OnClick:  onClick,
	}
	if busy {
		labelStyle := th.TextStyleBody
		labelStyle.Color = th.ColorTextOnAccent
		cfg.Content = []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				Spacing: gui.SpacingTight,
				Padding: gui.PaddingNone,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.SvgSpinner(gui.SvgSpinnerCfg{
						ID:     id + "-spin",
						Kind:   gui.SvgSpinner90Ring,
						Width:  16,
						Height: 16,
						Color:  th.ColorTextOnAccent,
					}),
					gui.Text(gui.TextCfg{Text: loadingLabel, TextStyle: labelStyle}),
				},
			}),
		}
	} else {
		cfg.Label = label
	}
	return gui.Button(cfg)
}

func toastOK(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastSuccess, Duration: 2 * time.Second})
}

func toastErr(w *gui.Window, err error) {
	w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
}

func toastErrText(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastError, Duration: 4 * time.Second})
}

func toastInfo(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastInfo, Duration: 3 * time.Second})
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	if ue, ok := usererr.As(err); ok {
		if ue.Code != "" {
			if len(ue.Params) == 0 {
				return i18n.T(ue.Code)
			}
			args := make([]any, len(ue.Params))
			for i, p := range ue.Params {
				args[i] = p
			}
			return i18n.Tf(ue.Code, args...)
		}
		return ue.Message
	}
	return err.Error()
}

func withVault(w *gui.Window, d *Deps, reason string, fn func(), onAbort func(*gui.Window)) {
	cfg := vaultprompt.UnlockCfg{
		Vault:       d.Vault,
		UserDataDir: d.Store.UserDataDir(),
		Reason:      reason,
		OnUnlocked: func(*gui.Window) {
			fn()
		},
		OnFail: func(win *gui.Window, err error) {
			if onAbort != nil {
				onAbort(win)
			}
			toastErr(win, err)
		},
		OnCancel: onAbort,
	}
	if vaultcheck.HasEncryptedSecrets(d.Store, d.DB) {
		cfg.Probe = func(pw string) bool {
			return vaultcheck.ProbePassword(d.Store, d.DB, pw)
		}
	}
	vaultprompt.Unlock(w, cfg)
}

func displayGroupName(name string) string {
	switch name {
	case bookmark.DefaultGroupName, bookmark.LegacyDefaultGroupName:
		return i18n.T("bookmark.group.default")
	case bookmark.ImportedGroupName:
		return i18n.T("bookmark.group.imported")
	default:
		return name
	}
}

func isDefaultGroup(name string) bool {
	return name == bookmark.DefaultGroupName || name == bookmark.LegacyDefaultGroupName
}

func secretInputValue(draftVal string) string {
	if draftVal == bookmark.PasswordMask {
		return ""
	}
	return draftVal
}

// applySecretInput updates a masked secret field. Empty while still holding
// PasswordMask is ignored (spurious sync / untouched); empty after the user
// typed plaintext clears the stored secret on save.
func applySecretInput(dst *string, hasStored *bool, v string) (changed bool) {
	if v == "" {
		if *dst == bookmark.PasswordMask {
			return false
		}
		if *dst == "" && !*hasStored {
			return false
		}
		*dst = ""
		*hasStored = false
		return true
	}
	if *dst == v {
		return false
	}
	*dst = v
	return true
}
