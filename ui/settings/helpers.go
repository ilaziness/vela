package settings

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
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

// secretPlaceholder matches old Wails secret fields: show a mask when a
// value is already stored encrypted and the draft plaintext is empty.
func secretPlaceholder(plain, encrypted, fallback string) string {
	if strings.TrimSpace(plain) == "" && strings.TrimSpace(encrypted) != "" {
		return "••••••••"
	}
	return fallback
}

// formRowHelp is formRow plus a secondary helper line under the field
// (old Wails FormHelperText / helperText).
func formRowHelp(id, label, text, placeholder, help string, password bool, onChange func(string)) gui.View {
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		HAlign:  gui.HAlignStart,
		Content: []gui.View{
			formRow(id, label, text, placeholder, password, onChange),
			gui.Label(help, gui.CurrentTheme().TextStyleSecondary),
		},
	}))
}

// actionsRight right-aligns one or more action buttons (test + save, etc.).
func actionsRight(btns ...gui.View) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		HAlign:  gui.HAlignEnd,
		Content: btns,
	}))
}

// saveBtn right-aligns the primary action like the old Wails settings.
func saveBtn(id, label string, busy bool, onClick func(gui.EventCtx)) gui.View {
	return actionsRight(gui.Button(gui.ButtonCfg{
		ID:       id,
		Variant:  gui.ButtonPrimary,
		Label:    label,
		Disabled: busy,
		OnClick:  onClick,
	}))
}

// pathBrowseRow is a labelled read-only path + browse button on one baseline.
// Input must not carry Label here: labelled Input is a column, so a sibling
// button would sit beside the label instead of the field.
func pathBrowseRow(id, label, path, placeholder, browseLabel string, onBrowse func(gui.EventCtx)) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingSmall,
		HAlign:  gui.HAlignStart,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: th.TextStyleLabel,
			}),
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

func toastOK(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastSuccess, Duration: 2 * time.Second})
}

func toastErr(w *gui.Window, err error) {
	w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
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

func panelColumn(title string, body []gui.View) gui.View {
	th := gui.CurrentTheme()
	content := []gui.View{
		gui.Text(gui.TextCfg{Text: title, TextStyle: th.TextStyleTitle}),
	}
	content = append(content, body...)
	// Extra bottom pad so a FillFill table stops short of the window
	// edge; overflow then scrolls inside the table, not the page.
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingMedium,
		Padding: gui.NewPadding(gui.PadMedium, gui.PadMedium, gui.PadLarge, gui.PadMedium),
		HAlign:  gui.HAlignStart,
		VAlign:  gui.VAlignTop,
		Content: content,
	}))
}

// withVault runs fn once the process vault has a password.
// If locked, shows the shared masked unlock dialog.
func withVault(w *gui.Window, d *Deps, reason string, fn func()) {
	cfg := vaultprompt.UnlockCfg{
		Vault:       d.Vault,
		UserDataDir: d.Store.UserDataDir(),
		Reason:      reason,
		OnUnlocked: func(*gui.Window) {
			fn()
		},
		OnFail: func(win *gui.Window, err error) {
			toastErr(win, err)
		},
	}
	if vaultcheck.HasEncryptedSecrets(d.Store, d.DB) {
		cfg.Probe = func(pw string) bool {
			return vaultcheck.ProbePassword(d.Store, d.DB, pw)
		}
	}
	vaultprompt.Unlock(w, cfg)
}

func encryptOrKeep(d *Deps, plain, incomingEnc, previousEnc string) (string, error) {
	plain = strings.TrimSpace(plain)
	if plain != "" {
		pw, err := d.Vault.Get()
		if err != nil {
			return "", usererr.Coded("error.password_not_entered", "Password not entered")
		}
		enc, err := secret.Encrypt(pw, plain)
		if err != nil {
			return "", err
		}
		_ = secret.EnsureVerifier(d.Store.UserDataDir(), pw)
		return enc, nil
	}
	if strings.TrimSpace(incomingEnc) != "" {
		return strings.TrimSpace(incomingEnc), nil
	}
	return strings.TrimSpace(previousEnc), nil
}

func switchRow(id, label string, on bool, toggle func(gui.EventCtx)) gui.View {
	return gui.Switch(gui.SwitchCfg{
		ID:       id,
		Label:    label,
		Selected: on,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			toggle(ctx)
		},
	})
}

// settingsNavWidth is the settings left-nav column width (nav.go).
const settingsNavWidth float32 = 180

// tableCharPx approximates body glyph width for Value probes and
// ellipsis. Table measures cell.Value only — Content is ignored.
const tableCharPx float32 = 8

// settingsTableInnerWidth is the width available for table columns
// inside the settings content pane (window − nav − rule − pad −
// table scrollbar gutter).
func settingsTableInnerWidth(w *gui.Window) float32 {
	ww, _ := w.WindowSize()
	if ww < 1 {
		ww = 960
	}
	// panelColumn: PadMedium left/right (bottom is PadLarge, irrelevant here).
	pad := float32(gui.PadMedium) * 2
	sb := gui.DefaultScrollbarStyle.Size + float32(gui.PadXSmall)
	inner := float32(ww) - settingsNavWidth - 1 - pad - sb
	if inner < 320 {
		return 320
	}
	return inner
}

// resolveColWidths starts from per-column minimums. When the pane is
// narrower than the mins sum, columns shrink proportionally so the
// table still fills the width. Leftover width goes to flexIdx (long
// columns like fingerprint). gui.Table has no flex columns of its own.
func resolveColWidths(w *gui.Window, mins []float32, flexIdx int) []float32 {
	out := append([]float32(nil), mins...)
	var sum float32
	for _, m := range out {
		sum += m
	}
	if sum <= 0 {
		return out
	}
	avail := settingsTableInnerWidth(w)
	switch {
	case avail < sum:
		scale := avail / sum
		for i := range out {
			out[i] *= scale
		}
	case flexIdx >= 0 && flexIdx < len(out):
		out[flexIdx] += avail - sum
	}
	return out
}

// widthProbe is what gui.Table measures for column width. Cells that
// render via Content still need a Value probe (action buttons, etc.).
func widthProbe(px float32) string {
	n := int(px / tableCharPx)
	if n < 2 {
		n = 2
	}
	return strings.Repeat("0", n)
}

func truncChars(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func maxCharsFor(width float32) int {
	n := int(width / tableCharPx)
	if n < 4 {
		return 4
	}
	return n
}

func thW(label string, width float32) gui.TableCellCfg {
	th := gui.CurrentTheme()
	return gui.TableCellCfg{
		Value:    widthProbe(width),
		HeadCell: true,
		Content: gui.Text(gui.TextCfg{
			Text:      label,
			TextStyle: th.TextStyleBody.Bold(),
		}),
	}
}

func tdW(value string, width float32) gui.TableCellCfg {
	th := gui.CurrentTheme()
	return gui.TableCellCfg{
		Value: widthProbe(width),
		Content: gui.Text(gui.TextCfg{
			Text:      truncChars(value, maxCharsFor(width)),
			TextStyle: th.TextStyleBody,
		}),
	}
}

func actionCellW(content gui.View, width float32) gui.TableCellCfg {
	return gui.TableCellCfg{
		Value:   widthProbe(width),
		Content: content,
	}
}

func clickCellW(label string, width float32, onClick func(gui.EventCtx)) gui.TableCellCfg {
	th := gui.CurrentTheme()
	return gui.TableCellCfg{
		Value: widthProbe(width),
		Content: gui.Text(gui.TextCfg{
			Text:      truncChars(label, maxCharsFor(width)),
			TextStyle: th.TextStyleBody,
		}),
		OnClick: onClick,
	}
}

// settingsTable fills the remaining panel (width + height).
//
// go-gui Table rules we follow here:
//   - Sizing FillFill: table chrome fills the content pane (not FixedFit).
//   - FreezeHeader: header stays put; body is FillFill+Scrollable and
//     takes leftover height — no fixed MaxHeight (that left blank space
//     below while the table scrolled early).
//   - Column widths come only from cell.Value probes (see thW/tdW).
func settingsTable(id string, rows []gui.TableRowCfg) gui.View {
	return gui.Table(gui.TableCfg{
		ID:           id,
		Data:         rows,
		Sizing:       gui.FillFill,
		FreezeHeader: true,
		BorderStyle:  gui.TableBorderHorizontal,
		SizeBorder:   1,
	})
}
