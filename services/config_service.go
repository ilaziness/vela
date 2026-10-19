package services

import (
	"errors"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/secret"
	internalsync "github.com/ilaziness/vela/internal/sync"
)

const (
	EventInputPassword      = "eventInputPassword"
	EventInputPasswordClose = "eventInputPasswordClose"
	EventAppearance         = "eventAppearance"
)

func init() {
	application.RegisterEvent[string](EventInputPassword)
	application.RegisterEvent[string](EventInputPasswordClose)
	application.RegisterEvent[Appearance](EventAppearance)
}

// Appearance is the UI font broadcast after general settings are saved.
type Appearance struct {
	Font     string
	FontSize int
}

type Config = config.Config
type GeneralConfig = config.GeneralConfig
type TerminalConfig = config.TerminalConfig
type SSHConfig = config.SSHConfig
type AIConfig = ai.Config

type ConfigService struct {
	app          *application.App
	windows      *Windows
	store        *config.Store
	vault        *secret.Vault
	passwordChan chan struct{}
	chanMutex    sync.Mutex
	onSSHSaved   func(config.SSHConfig)
}

func NewConfigService(app *application.App, windows *Windows, store *config.Store, vault *secret.Vault) *ConfigService {
	return &ConfigService{app: app, windows: windows, store: store, vault: vault}
}

func (cs *ConfigService) current() *config.Config {
	return cs.store.Config
}

func (cs *ConfigService) userDataDir() string {
	return cs.store.Config.General.UserDataDir
}

func (cs *ConfigService) ShowWindow()  { cs.windows.ShowSetting() }
func (cs *ConfigService) CloseWindow() { cs.windows.CloseSetting() }

// IsFrameless reports the frame mode this process was started with.
func (cs *ConfigService) IsFrameless() bool {
	return cs.windows.IsFrameless()
}

func (cs *ConfigService) SetTheme(theme string) {
	cs.current().General.Theme = theme
	_ = cs.store.Save()
}

func (cs *ConfigService) ReadConfig() (*Config, error) {
	return cs.current(), nil
}

func (cs *ConfigService) SaveConfig(cfg Config) error {
	if err := cs.store.ApplyUserDataDir(cfg.General.UserDataDir); err != nil {
		return err
	}
	*cs.store.Config = cfg
	return cs.store.Save()
}

func (cs *ConfigService) UpdateAppConfig() error {
	return cs.store.SaveAppConfig()
}

func (cs *ConfigService) GetSyncConfig() *internalsync.SyncConfig {
	return &cs.current().Sync
}

func (cs *ConfigService) SaveSyncConfig(syncConfig internalsync.SyncConfig) error {
	cs.current().Sync = syncConfig
	return cs.store.Save()
}

func (cs *ConfigService) SaveGeneralConfig(generalConfig GeneralConfig) error {
	generalConfig.Normalize()
	// 通用设置不编辑主题。沿用内存里的值，避免设置页打开后在主窗口改过的主题被旧快照覆盖。
	generalConfig.Theme = cs.current().General.Theme
	if err := cs.store.ApplyUserDataDir(generalConfig.UserDataDir); err != nil {
		return err
	}
	cs.current().General = generalConfig
	if err := cs.store.Save(); err != nil {
		return err
	}
	cs.app.Event.Emit(EventAppearance, Appearance{
		Font:     generalConfig.Font,
		FontSize: generalConfig.FontSize,
	})
	if generalConfig.Frameless != cs.windows.IsFrameless() {
		cs.showFramelessRestartDialog()
	}
	return nil
}

func (cs *ConfigService) showFramelessRestartDialog() {
	dlg := cs.app.Dialog.Info().
		SetTitle("Vela").
		SetMessage("无边框模式已更改，需要重新启动应用后生效。")
	ok := dlg.AddButton("确定")
	dlg.SetDefaultButton(ok)
	dlg.SetCancelButton(ok)
	if cs.windows.Setting != nil {
		dlg.AttachToWindow(cs.windows.Setting)
	}
	dlg.Show()
}

func (cs *ConfigService) SaveTerminalConfig(terminalConfig TerminalConfig) error {
	cs.current().Terminal = terminalConfig
	return cs.store.Save()
}

func (cs *ConfigService) SaveSSHConfig(sshConfig SSHConfig) error {
	sshConfig.Normalize()
	if err := sshConfig.ValidateProxy(); err != nil {
		return err
	}
	cs.current().SSH = sshConfig
	if err := cs.store.Save(); err != nil {
		return err
	}
	if cs.onSSHSaved != nil {
		cs.onSSHSaved(sshConfig)
	}
	return nil
}

func (cs *ConfigService) aiConfig() ai.Config {
	return cs.current().AI
}

func (cs *ConfigService) saveAI(cfg ai.Config) error {
	cs.current().AI = cfg
	return cs.store.Save()
}

func (cs *ConfigService) SetUserPassword(password string) {
	cs.vault.Set(password)
	cs.chanMutex.Lock()
	ch := cs.passwordChan
	cs.passwordChan = nil
	cs.chanMutex.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (cs *ConfigService) waitForPassword(reason string) error {
	if _, err := cs.vault.Get(); err == nil {
		return nil
	}
	cs.chanMutex.Lock()
	if cs.passwordChan == nil {
		cs.passwordChan = make(chan struct{})
		cs.app.Event.Emit(EventInputPassword, reason)
	}
	ch := cs.passwordChan
	cs.chanMutex.Unlock()
	select {
	case <-ch:
		if _, err := cs.vault.Get(); err != nil {
			return errors.New("password not entered")
		}
		cs.app.Event.Emit(EventInputPasswordClose, "")
		return nil
	case <-time.After(60 * time.Second):
		cs.chanMutex.Lock()
		if cs.passwordChan == ch {
			cs.passwordChan = nil
			close(ch)
		}
		cs.chanMutex.Unlock()
		return errors.New("password input timeout")
	}
}

func (cs *ConfigService) getPasswordWithPrompt(reason string) (string, error) {
	password, err := cs.vault.Get()
	if err != nil {
		if err := cs.waitForPassword(reason); err != nil {
			return "", err
		}
		password, err = cs.vault.Get()
		if err != nil {
			return "", err
		}
	}
	return password, nil
}

func (cs *ConfigService) encryptValue(plain, reason string) (string, error) {
	password, err := cs.getPasswordWithPrompt(reason)
	if err != nil {
		return "", err
	}
	return secret.Encrypt(password, plain)
}

func (cs *ConfigService) decryptValue(enc, reason string) (string, error) {
	password, err := cs.getPasswordWithPrompt(reason)
	if err != nil {
		return "", err
	}
	return secret.Decrypt(password, enc)
}

func (cs *ConfigService) encryptAPIKey(apiKey string) (string, error) {
	return cs.encryptValue(apiKey, "需要密码来加密 API Key")
}

func (cs *ConfigService) decryptAPIKey(encrypted string) (string, error) {
	return cs.decryptValue(encrypted, "需要密码来解密 API Key")
}
