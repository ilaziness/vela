package settings

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/internal/usererr"
)

func panelSync(d *Deps, st *State) gui.View {
	c := &st.Sync
	provider := string(c.Provider)
	opts := []gui.SelectOption{
		{Label: i18n.T("settings.sync.provider_selfhosted"), Value: string(sync.ProviderSelfHosted)},
		{Label: i18n.T("settings.sync.provider_webdav"), Value: string(sync.ProviderWebDAV)},
		{Label: i18n.T("settings.sync.provider_s3"), Value: string(sync.ProviderS3)},
	}

	rows := []gui.View{
		gui.Select(gui.SelectCfg{
			ID:       "settings-sync-provider",
			Label:    i18n.T("settings.sync.provider"),
			Selected: []string{provider},
			Options:  opts,
			Sizing:   gui.FillFit,
			OnSelect: func(vals []string, ctx gui.EventCtx) {
				ctx.Consume()
				if len(vals) > 0 {
					gui.State[State](ctx.Window).Sync.Provider = sync.Provider(vals[0])
				}
			},
		}),
	}

	switch c.Provider {
	case sync.ProviderWebDAV:
		rows = append(rows,
			formRow("sync-webdav-url", i18n.T("settings.sync.webdav_url"), c.WebDAV.URL,
				i18n.T("settings.sync.webdav_url_placeholder"), false, func(v string) { c.WebDAV.URL = v }),
			formRow("sync-webdav-user", i18n.T("settings.sync.webdav_username"), c.WebDAV.Username,
				"", false, func(v string) { c.WebDAV.Username = v }),
			formRow("sync-webdav-path", i18n.T("settings.sync.webdav_remote_path"), c.WebDAV.RemotePath,
				i18n.T("settings.sync.webdav_remote_path_placeholder"), false, func(v string) { c.WebDAV.RemotePath = v }),
			formRow("sync-webdav-pass", i18n.T("settings.sync.webdav_password"), c.WebDAV.Password,
				secretPlaceholder(c.WebDAV.Password, c.WebDAV.PasswordEncrypted, i18n.T("settings.sync.webdav_password")),
				true, func(v string) { c.WebDAV.Password = v }),
		)
	case sync.ProviderS3:
		rows = append(rows,
			formRow("sync-s3-endpoint", i18n.T("settings.sync.s3_endpoint"), c.S3.Endpoint,
				i18n.T("settings.sync.s3_endpoint_placeholder"), false, func(v string) { c.S3.Endpoint = v }),
			formRow("sync-s3-region", i18n.T("settings.sync.s3_region"), c.S3.Region,
				i18n.T("settings.sync.s3_region_placeholder"), false, func(v string) { c.S3.Region = v }),
			formRow("sync-s3-bucket", i18n.T("settings.sync.s3_bucket"), c.S3.Bucket,
				"", false, func(v string) { c.S3.Bucket = v }),
			formRow("sync-s3-prefix", i18n.T("settings.sync.s3_prefix"), c.S3.Prefix,
				i18n.T("settings.sync.s3_prefix_placeholder"), false, func(v string) { c.S3.Prefix = v }),
			formRow("sync-s3-ak", i18n.T("settings.sync.s3_access_key"), c.S3.AccessKey,
				"", false, func(v string) { c.S3.AccessKey = v }),
			formRow("sync-s3-sk", i18n.T("settings.sync.s3_secret_key"), c.S3.SecretKey,
				secretPlaceholder(c.S3.SecretKey, c.S3.SecretKeyEncrypted, i18n.T("settings.sync.s3_secret_key")),
				true, func(v string) { c.S3.SecretKey = v }),
		)
	default:
		rows = append(rows,
			formRow("sync-sh-url", i18n.T("settings.sync.server_url"), c.SelfHosted.ServerURL,
				i18n.T("settings.sync.server_url_placeholder"), false, func(v string) { c.SelfHosted.ServerURL = v }),
			formRow("sync-sh-id", i18n.T("settings.sync.sync_id"), c.SelfHosted.SyncID,
				i18n.T("settings.sync.sync_id_placeholder"), false, func(v string) { c.SelfHosted.SyncID = v }),
			formRow("sync-sh-key", i18n.T("settings.sync.user_key"), c.SelfHosted.UserKey,
				secretPlaceholder(c.SelfHosted.UserKey, c.SelfHosted.UserKeyEncrypted, i18n.T("settings.sync.user_key_placeholder")),
				true, func(v string) { c.SelfHosted.UserKey = v }),
		)
	}

	rows = append(rows, actionsRight(
		gui.Button(gui.ButtonCfg{
			ID:       "settings-sync-test",
			Variant:  gui.ButtonSecondary,
			Label:    i18n.T("settings.sync.test_connection"),
			Disabled: st.Busy,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				testSyncConnection(ctx.Window, d)
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID:       "settings-sync-save",
			Variant:  gui.ButtonPrimary,
			Label:    i18n.T("common.save"),
			Disabled: st.Busy,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				saveSync(ctx.Window, d)
			},
		}),
	))
	return panelColumn(i18n.T("settings.nav.sync"), rows)
}

func testSyncConnection(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	draft := st.Sync
	if draft.Provider == "" {
		draft.Provider = sync.ProviderSelfHosted
	}
	if !draft.IsConfigured() {
		toastErr(w, usererr.Coded("sync.not_configured", "Backup & restore is not configured"))
		return
	}
	run := func() {
		st.Busy = true
		go func() {
			err := probeSync(d, draft)
			w.QueueCommand(func(win *gui.Window) {
				gui.State[State](win).Busy = false
				if err != nil {
					toastErr(win, err)
					return
				}
				toastOK(win, i18n.T("settings.sync.connected"))
			})
		}()
	}
	if syncNeedsDecrypt(draft) {
		withVault(w, d, i18n.T("password.decrypt_sync"), run)
		return
	}
	run()
}

func syncNeedsDecrypt(cfg sync.SyncConfig) bool {
	switch cfg.Provider {
	case sync.ProviderSelfHosted:
		return strings.TrimSpace(cfg.SelfHosted.UserKey) == "" &&
			strings.TrimSpace(cfg.SelfHosted.UserKeyEncrypted) != ""
	case sync.ProviderWebDAV:
		return strings.TrimSpace(cfg.WebDAV.Password) == "" &&
			strings.TrimSpace(cfg.WebDAV.PasswordEncrypted) != ""
	case sync.ProviderS3:
		return strings.TrimSpace(cfg.S3.SecretKey) == "" &&
			strings.TrimSpace(cfg.S3.SecretKeyEncrypted) != ""
	default:
		return false
	}
}

func probeSync(d *Deps, draft sync.SyncConfig) error {
	plain, err := resolveSyncPlain(d, draft)
	if err != nil {
		return err
	}
	st, err := sync.OpenStore(&plain)
	if err != nil {
		return err
	}
	return sync.NewSyncManager(st).HealthCheck()
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

func saveSync(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	incoming := st.Sync
	if incoming.Provider == "" {
		incoming.Provider = sync.ProviderSelfHosted
	}
	needsSecret := false
	switch incoming.Provider {
	case sync.ProviderSelfHosted:
		needsSecret = strings.TrimSpace(incoming.SelfHosted.UserKey) != ""
	case sync.ProviderWebDAV:
		needsSecret = strings.TrimSpace(incoming.WebDAV.Password) != ""
	case sync.ProviderS3:
		needsSecret = strings.TrimSpace(incoming.S3.SecretKey) != ""
	}
	run := func() {
		st.Busy = true
		draft := incoming
		prev := d.Store.Config.Sync
		go func() {
			cfg, err := applySyncSecrets(d, draft, prev)
			if err == nil {
				d.Store.Config.Sync = cfg
				err = d.Store.Save()
			}
			w.QueueCommand(func(win *gui.Window) {
				s := gui.State[State](win)
				s.Busy = false
				if err != nil {
					toastErr(win, err)
					return
				}
				cfg.SelfHosted.UserKey = ""
				cfg.WebDAV.Password = ""
				cfg.S3.SecretKey = ""
				s.Sync = cfg
				toastOK(win, i18n.T("settings.sync.save_success"))
			})
		}()
	}
	if needsSecret {
		withVault(w, d, i18n.T("password.encrypt_sync"), run)
		return
	}
	run()
}

func applySyncSecrets(d *Deps, cfg, prev sync.SyncConfig) (sync.SyncConfig, error) {
	var err error
	switch cfg.Provider {
	case sync.ProviderSelfHosted:
		cfg.SelfHosted.UserKeyEncrypted, err = encryptOrKeep(d,
			cfg.SelfHosted.UserKey, cfg.SelfHosted.UserKeyEncrypted, prev.SelfHosted.UserKeyEncrypted)
		cfg.SelfHosted.UserKey = ""
	case sync.ProviderWebDAV:
		cfg.WebDAV.PasswordEncrypted, err = encryptOrKeep(d,
			cfg.WebDAV.Password, cfg.WebDAV.PasswordEncrypted, prev.WebDAV.PasswordEncrypted)
		cfg.WebDAV.Password = ""
	case sync.ProviderS3:
		cfg.S3.SecretKeyEncrypted, err = encryptOrKeep(d,
			cfg.S3.SecretKey, cfg.S3.SecretKeyEncrypted, prev.S3.SecretKeyEncrypted)
		cfg.S3.SecretKey = ""
	}
	return cfg, err
}
