package settings

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/ui/errtext"
	"github.com/ilaziness/vela/ui/layout"
)

func panelSync(w *gui.Window, d *Deps, st *State) gui.View {
	c := &st.Sync
	provider := string(c.Provider)
	opts := []gui.SelectOption{
		{Label: i18n.T("settings.sync.provider_selfhosted"), Value: string(sync.ProviderSelfHosted)},
		{Label: i18n.T("settings.sync.provider_webdav"), Value: string(sync.ProviderWebDAV)},
		{Label: i18n.T("settings.sync.provider_s3"), Value: string(sync.ProviderS3)},
	}
	providerHint := ""
	switch c.Provider {
	case sync.ProviderWebDAV:
		providerHint = i18n.T("settings.sync.provider_webdav_hint")
	case sync.ProviderS3:
		providerHint = i18n.T("settings.sync.provider_s3_hint")
	default:
		providerHint = i18n.T("settings.sync.provider_selfhosted_hint")
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
		gui.Label(providerHint, gui.CurrentTheme().TextStyleSecondary),
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

	if d.Store.Config.Sync.IsConfigured() {
		rows = append(rows, syncBackupsSection(w, d, st))
	}
	return panelColumn(i18n.T("settings.nav.sync"), rows)
}

// syncBackupsSection lists backups stored under the *saved* sync config.
// Draft edits do not affect it until Save succeeds.
func syncBackupsSection(w *gui.Window, d *Deps, st *State) gui.View {
	th := gui.CurrentTheme()
	header := gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFit,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: i18n.T("settings.sync.backup_list"), TextStyle: th.TextStyleLabel, Sizing: gui.FillFit}),
			gui.Button(gui.ButtonCfg{
				ID:       "settings-sync-backups-refresh",
				Variant:  gui.ButtonGhost,
				Label:    i18n.T("common.refresh"),
				Disabled: st.SyncBackupsLoading || st.Busy,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					withVault(ctx.Window, d, i18n.T("password.decrypt_sync"), func() {
						loadSyncBackups(ctx.Window, d)
					})
				},
			}),
		},
	}))

	var table gui.View
	switch {
	case st.SyncBackupsLoading && !st.SyncBackupsLoaded:
		table = gui.Text(gui.TextCfg{Text: i18n.T("common.loading"), TextStyle: th.TextStyleSecondary})
	case st.SyncBackupsErr != "":
		table = gui.Text(gui.TextCfg{Text: st.SyncBackupsErr, TextStyle: th.TextStyleError, Mode: gui.TextModeWrap, Sizing: gui.FillFit})
	case st.SyncBackupsLoaded && len(st.SyncBackups) == 0:
		table = gui.Text(gui.TextCfg{Text: i18n.T("settings.sync.backup_empty"), TextStyle: th.TextStyleSecondary})
	case len(st.SyncBackups) == 0:
		table = gui.Text(gui.TextCfg{Text: i18n.T("settings.sync.backup_unlock_hint"), TextStyle: th.TextStyleSecondary})
	default:
		table = syncBackupsTable(w, d, st)
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingSmall,
		Content: []gui.View{
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingTight,
				Content: []gui.View{
					header,
					gui.Label(i18n.T("settings.sync.backup_list_hint"), th.TextStyleSecondary),
				},
			})),
			table,
		},
	}))
}

var syncBackupsColMin = []float32{220, 90, 185, 72}

func syncBackupsTable(w *gui.Window, d *Deps, st *State) gui.View {
	cols := resolveColWidths(w, syncBackupsColMin, 0)
	rows := []gui.TableRowCfg{
		gui.TR([]gui.TableCellCfg{
			thW(i18n.T("settings.sync.backup_col_name"), cols[0]),
			thW(i18n.T("settings.sync.backup_col_size"), cols[1]),
			thW(i18n.T("settings.sync.backup_col_time"), cols[2]),
			thW(i18n.T("settings.sync.backup_col_actions"), cols[3]),
		}),
	}
	for i := range st.SyncBackups {
		b := st.SyncBackups[i]
		name := b.Name
		rows = append(rows, gui.TR([]gui.TableCellCfg{
			tdW(name, cols[0]),
			tdW(formatFileSize(b.FileSize), cols[1]),
			tdW(formatDateTime(b.CreatedAt), cols[2]),
			actionCellW(gui.Button(gui.ButtonCfg{
				ID:      "settings-sync-backup-del-" + name,
				Variant: gui.ButtonGhost,
				Label:   i18n.T("common.delete"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					deleteSyncBackup(ctx.Window, d, name)
				},
			}), cols[3]),
		}))
	}
	return settingsTable("settings-sync-backups-table", rows)
}

// loadSyncBackups fetches the backup list for the saved sync config.
// Caller must ensure the vault is unlocked (Vault.Get succeeds) or wrap
// with withVault first.
func loadSyncBackups(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	if st.SyncBackupsLoading {
		return
	}
	cfg := d.Store.Config.Sync
	if cfg.Provider == "" {
		cfg.Provider = sync.ProviderSelfHosted
	}
	if !cfg.IsConfigured() {
		return
	}
	st.SyncBackupsLoading = true
	st.SyncBackupsErr = ""
	go func() {
		manager, err := openSyncManager(d, cfg)
		var list []sync.BackupInfo
		if err == nil {
			list, err = manager.ListBackups(0, 0)
		}
		w.QueueCommand(func(win *gui.Window) {
			s := gui.State[State](win)
			s.SyncBackupsLoading = false
			s.SyncBackupsLoaded = true
			if err != nil {
				s.SyncBackupsErr = i18n.Tf("settings.sync.backup_load_failed", errtext.Text(err))
				return
			}
			s.SyncBackups = list
			s.SyncBackupsErr = ""
		})
	}()
}

// maybeLoadSyncBackups silently refreshes the backup list when the vault
// is already unlocked; used on nav entry and after save.
func maybeLoadSyncBackups(w *gui.Window, d *Deps) {
	if _, err := d.Vault.Get(); err != nil {
		return
	}
	if !d.Store.Config.Sync.IsConfigured() {
		return
	}
	loadSyncBackups(w, d)
}

func deleteSyncBackup(w *gui.Window, d *Deps, name string) {
	dangerConfirmDialog(w,
		i18n.T("settings.sync.confirm_delete_title"),
		i18n.T("settings.sync.confirm_delete_body"),
		name,
		func(win *gui.Window) { runDeleteSyncBackup(win, d, name) })
}

// runDeleteSyncBackup performs the delete after the custom confirm dialog.
func runDeleteSyncBackup(w *gui.Window, d *Deps, name string) {
	withVault(w, d, i18n.T("password.decrypt_sync"), func() {
		cfg := d.Store.Config.Sync
		if cfg.Provider == "" {
			cfg.Provider = sync.ProviderSelfHosted
		}
		go func() {
			manager, err := openSyncManager(d, cfg)
			if err == nil {
				err = manager.DeleteBackup(name)
			}
			w.QueueCommand(func(w2 *gui.Window) {
				if err != nil {
					toastErr(w2, usererr.Coded("settings.sync.delete_backup_failed", "%s", errtext.Text(err)))
					return
				}
				toastOK(w2, i18n.T("settings.sync.delete_backup_success"))
				loadSyncBackups(w2, d)
			})
		}()
	})
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
	manager, err := openSyncManager(d, draft)
	if err != nil {
		return err
	}
	return manager.HealthCheck()
}

// openSyncManager decrypts provider secrets via the vault, then opens a store.
func openSyncManager(d *Deps, cfg sync.SyncConfig) (*sync.SyncManager, error) {
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

func resolveSyncPlain(d *Deps, cfg sync.SyncConfig) (sync.SyncConfig, error) {
	out := cfg
	decrypt := func(enc string) (string, error) {
		pw, err := d.Vault.Get()
		if err != nil || strings.TrimSpace(pw) == "" {
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

// validateSyncConfig enforces required fields per provider before saving.
// Secret fields accept either fresh plaintext or a previously stored
// ciphertext (same rule as SyncConfig.IsConfigured).
func validateSyncConfig(cfg sync.SyncConfig) error {
	hasSecret := func(plain, enc string) bool {
		return strings.TrimSpace(plain) != "" || strings.TrimSpace(enc) != ""
	}
	switch cfg.Provider {
	case sync.ProviderSelfHosted:
		sh := cfg.SelfHosted
		if strings.TrimSpace(sh.ServerURL) == "" {
			return usererr.Coded("settings.sync.server_url_required", "Server URL is required")
		}
		if strings.TrimSpace(sh.SyncID) == "" {
			return usererr.Coded("settings.sync.sync_id_required", "Backup ID is required")
		}
		if !hasSecret(sh.UserKey, sh.UserKeyEncrypted) {
			return usererr.Coded("settings.sync.user_key_required", "User key is required")
		}
	case sync.ProviderWebDAV:
		wv := cfg.WebDAV
		if strings.TrimSpace(wv.URL) == "" {
			return usererr.Coded("settings.sync.webdav_url_required", "WebDAV URL is required")
		}
		if strings.TrimSpace(wv.Username) == "" {
			return usererr.Coded("settings.sync.webdav_username_required", "Username is required")
		}
		if strings.TrimSpace(wv.RemotePath) == "" {
			return usererr.Coded("settings.sync.webdav_remote_path_required", "Remote path is required")
		}
		if !hasSecret(wv.Password, wv.PasswordEncrypted) {
			return usererr.Coded("settings.sync.webdav_password_required", "Password is required")
		}
	case sync.ProviderS3:
		s3 := cfg.S3
		if strings.TrimSpace(s3.Endpoint) == "" {
			return usererr.Coded("settings.sync.s3_endpoint_required", "Endpoint is required")
		}
		if strings.TrimSpace(s3.Bucket) == "" {
			return usererr.Coded("settings.sync.s3_bucket_required", "Bucket is required")
		}
		if strings.TrimSpace(s3.AccessKey) == "" {
			return usererr.Coded("settings.sync.s3_access_key_required", "Access Key is required")
		}
		if !hasSecret(s3.SecretKey, s3.SecretKeyEncrypted) {
			return usererr.Coded("settings.sync.s3_secret_key_required", "Secret Key is required")
		}
	default:
		return usererr.Coded("settings.sync.provider_required", "Please select a storage provider")
	}
	return nil
}

func saveSync(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	incoming := st.Sync
	if incoming.Provider == "" {
		incoming.Provider = sync.ProviderSelfHosted
	}
	if err := validateSyncConfig(incoming); err != nil {
		toastErr(w, err)
		return
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
				maybeLoadSyncBackups(win, d)
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
