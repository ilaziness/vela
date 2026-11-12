package backup

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/vaultprompt"
)

func toastOK(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastSuccess, Duration: 2 * time.Second})
}

func toastErr(w *gui.Window, err error) {
	w.Toast(gui.ToastCfg{Body: errText(err), Severity: gui.ToastError, Duration: 4 * time.Second})
}

func toastErrMsg(w *gui.Window, body string) {
	w.Toast(gui.ToastCfg{Body: body, Severity: gui.ToastError, Duration: 4 * time.Second})
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

func withVault(w *gui.Window, d *Deps, reason string, fn func()) {
	vaultprompt.Unlock(w, vaultprompt.UnlockCfg{
		Vault:       d.Vault,
		UserDataDir: d.Store.UserDataDir(),
		Reason:      reason,
		OnUnlocked: func(*gui.Window) {
			fn()
		},
		OnFail: func(win *gui.Window, err error) {
			toastErr(win, err)
		},
	})
}

func resolveSyncPlain(d *Deps, cfg sync.SyncConfig) (sync.SyncConfig, error) {
	out := cfg
	decrypt := func(enc string) (string, error) {
		pw, err := d.Vault.Get()
		if err != nil {
			return "", usererr.Coded("error.password_not_entered", "Password not entered")
		}
		return secret.Decrypt(pw, enc)
	}
	switch out.Provider {
	case sync.ProviderSelfHosted:
		if strings.TrimSpace(out.SelfHosted.UserKey) == "" && strings.TrimSpace(out.SelfHosted.UserKeyEncrypted) != "" {
			plain, err := decrypt(out.SelfHosted.UserKeyEncrypted)
			if err != nil {
				return out, err
			}
			out.SelfHosted.UserKey = plain
		}
	case sync.ProviderWebDAV:
		if strings.TrimSpace(out.WebDAV.Password) == "" && strings.TrimSpace(out.WebDAV.PasswordEncrypted) != "" {
			plain, err := decrypt(out.WebDAV.PasswordEncrypted)
			if err != nil {
				return out, err
			}
			out.WebDAV.Password = plain
		}
	case sync.ProviderS3:
		if strings.TrimSpace(out.S3.SecretKey) == "" && strings.TrimSpace(out.S3.SecretKeyEncrypted) != "" {
			plain, err := decrypt(out.S3.SecretKeyEncrypted)
			if err != nil {
				return out, err
			}
			out.S3.SecretKey = plain
		}
	}
	return out, nil
}

func openManager(d *Deps) (*sync.SyncManager, error) {
	cfg := d.Store.Config.Sync
	if !cfg.IsConfigured() {
		return nil, usererr.Coded("sync.not_configured", "Sync is not configured")
	}
	plain, err := resolveSyncPlain(d, cfg)
	if err != nil {
		return nil, err
	}
	st, err := sync.OpenStore(&plain)
	if err != nil {
		return nil, err
	}
	return sync.NewSyncManager(st), nil
}

func vaultPassword(d *Deps) (string, error) {
	pw, err := d.Vault.Get()
	if err != nil {
		return "", usererr.Coded("error.password_not_entered", "Password not entered")
	}
	return pw, nil
}

func formatFileSize(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func dialogButtons(btns ...gui.View) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		HAlign:  gui.HAlignEnd,
		Content: btns,
	}))
}
