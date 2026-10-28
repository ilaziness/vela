package services

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/internal/termws"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/tunnel"
)

// BootstrapLanguage detects and persists language on first launch, then applies it.
// Call after config.Load; not inside config.Load.
func BootstrapLanguage(store *config.Store, log *zap.Logger) {
	if store == nil || store.Config == nil {
		return
	}
	if log == nil {
		log = zap.NewNop()
	}
	stored := strings.TrimSpace(store.Config.General.Language)
	detected := i18n.DetectSystem()
	lang := stored
	if lang == "" {
		lang = detected
	}
	lang = i18n.Normalize(lang)
	log.Debug("bootstrap language",
		zap.String("stored", stored),
		zap.String("detected", detected),
		zap.String("applied", lang),
	)
	if lang != stored {
		store.Config.General.Language = lang
		if err := store.Save(); err != nil {
			log.Error("persist language failed", zap.Error(err))
		} else {
			log.Debug("persisted language", zap.String("language", lang))
		}
	}
	i18n.SetLanguage(lang)
}

func RegisterServices(a *application.App, mainWindow *application.WebviewWindow, cfgStore *config.Store, log *zap.Logger, commandsLoader command.LocaleLoader) error {
	if log == nil {
		log = zap.NewNop()
	}
	if cfgStore == nil || cfgStore.Config == nil {
		return fmt.Errorf("config store is nil")
	}
	BootstrapLanguage(cfgStore, log)
	windows := NewWindows(a, mainWindow, cfgStore.Config.General.Frameless)
	vault := secret.NewVault()
	configService := NewConfigService(a, windows, cfgStore, vault)

	db := database.NewDatabase(cfgStore.UserDataDir(), log)
	if err := db.Initialize(); err != nil {
		return err
	}

	hostPrompter := &hostKeyPrompter{app: a}
	keyboardPrompter := &keyboardInteractivePrompter{app: a}
	sshMgr := ssh.NewManager(log, filepath.Join(cfgStore.UserDataDir(), "known_hosts"), hostPrompter, keyboardPrompter)
	sshMgr.SetZmodem(newZmodemPicker(a))
	sshMgr.SetOptions(ssh.Options{
		ServerAliveInterval: time.Duration(cfgStore.Config.SSH.ServerAliveInterval) * time.Second,
		DialTimeout:         time.Duration(cfgStore.Config.SSH.DialTimeoutSec) * time.Second,
	})
	configService.onSSHSaved = func(c config.SSHConfig) {
		sshMgr.SetOptions(ssh.Options{
			ServerAliveInterval: time.Duration(c.ServerAliveInterval) * time.Second,
			DialTimeout:         time.Duration(c.DialTimeoutSec) * time.Second,
		})
	}
	transfers := transfer.NewRegistry(func(p transfer.ProgressData) {
		a.Event.Emit(EventProgress, p)
	})
	sftpMgr := sftp.NewManager(log, sshMgr, transfers, db.SftpTransferRepo)
	sftpMgr.SetOnQueued(func(sessionID string, count int) {
		a.Event.Emit(EventTransferQueued, TransferQueuedData{
			SessionID: sessionID,
			Count:     count,
		})
	})
	tunnelMgr := tunnel.NewManager(log, sshMgr)

	sftpService := NewSftpService(a, sftpMgr, log)
	sshService := NewSSHService(a, sshMgr, sftpMgr, tunnelMgr)
	sshService.setSFTPCloseHooks(sftpService.CloseSessionEdits, sftpService.CleanupSessionEditDir, sftpService.CloseAllEdits, sftpService.CleanupAllEditDirs)
	sshService.setSSHConfigGetter(func() config.SSHConfig {
		return cfgStore.Config.SSH
	})
	termWS := termws.NewServer(log, sshMgr, func(id string) { _ = sshService.CloseByID(id) })
	sshMgr.SetOnClose(sshService.onSessionClosed)

	keys := sshkey.New(log, db, configService.getPasswordWithPrompt, vault.Clear)
	bookmarks := bookmark.New(log, db, configService.getPasswordWithPrompt, func() {
		a.Event.Emit(EventBookmarkUpdate, BookmarkUpdateMsg)
	}, vault.Clear, keys.PrivatePEM)
	sshService.bind(termWS, bookmarks)
	bookmarkService := NewBookmarkService(a, bookmarks, sshService)

	commandSvc := command.New(log, db, commandsLoader, sshService.SendToSession)
	termWS.SetOnCommand(func(cmd string) {
		if err := commandSvc.AddHistory(cmd); err != nil {
			log.Error("add shell command history failed", zap.Error(err))
		}
	})
	commandService := NewCommandService(windows, commandSvc)
	configService.onLanguageChanged = func(lang string) {
		if err := commandSvc.ReloadBuiltin(lang); err != nil {
			log.Error("reload builtin commands failed", zap.String("language", lang), zap.Error(err))
		}
	}
	syncService := NewSyncService(configService, db, log)
	toolService := NewToolService(windows)
	aiService := NewAIService(a, log, configService, sshService, db)
	aiService.RecoverOrphanTurns()
	appService := NewAppService(a, windows, termWS, log)
	tunnelService := NewSSHTunnelService(tunnelMgr)

	a.RegisterService(application.NewService(appService))
	a.RegisterService(application.NewService(sshService))
	a.RegisterService(application.NewService(sftpService))
	a.RegisterService(application.NewService(configService))
	a.RegisterService(application.NewService(bookmarkService))
	a.RegisterService(application.NewService(tunnelService))
	a.RegisterService(application.NewService(commandService))
	a.RegisterService(application.NewService(syncService))
	a.RegisterService(application.NewService(toolService))
	a.RegisterService(application.NewService(NewKeyService(a, keys, log)))
	a.RegisterService(application.NewService(aiService))

	appService.startBackgroundUpdateCheck()
	kbMgr := NewKeyBindingManager(a, windows, cfgStore, log)
	appService.keybindings = kbMgr
	kbMgr.Register()

	if err := termWS.Start(); err != nil {
		return err
	}

	a.OnShutdown(func() {
		log.Debug("run app OnShutdown...")
		aiService.FlushAndStop()
		aiService.ShutdownMCP()
		termWS.Stop()
		sshService.Close()
		if err := db.Close(); err != nil {
			log.Error("close db failed", zap.Error(err))
		}
	})
	return nil
}
