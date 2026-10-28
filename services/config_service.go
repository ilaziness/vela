package services

import (
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	internalsync "github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/internal/usererr"
)

const (
	EventInputPassword      = "eventInputPassword"
	EventInputPasswordClose = "eventInputPasswordClose"
	EventAppearance         = "eventAppearance"
	EventTerminal           = "eventTerminal"
	EventLanguage           = "eventLanguage"
)

func init() {
	application.RegisterEvent[string](EventInputPassword)
	application.RegisterEvent[string](EventInputPasswordClose)
	application.RegisterEvent[Appearance](EventAppearance)
	application.RegisterEvent[TerminalConfig](EventTerminal)
	application.RegisterEvent[string](EventLanguage)
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

// I18nBootstrap is returned once per WebView to seed language + catalog.
type I18nBootstrap struct {
	Language string            `json:"language"`
	Catalog  map[string]string `json:"catalog"`
}

type ConfigService struct {
	app               *application.App
	windows           *Windows
	store             *config.Store
	vault             *secret.Vault
	passwordChan      chan struct{}
	chanMutex         sync.Mutex
	onSSHSaved        func(config.SSHConfig)
	onLanguageChanged func(lang string)
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

// GetI18nBootstrap returns the active language and flattened catalog for the frontend.
func (cs *ConfigService) GetI18nBootstrap() I18nBootstrap {
	lang := i18n.Language()
	return I18nBootstrap{
		Language: lang,
		Catalog:  i18n.Catalog(lang),
	}
}

// SetLanguage is the sole write path for General.Language.
func (cs *ConfigService) SetLanguage(lang string) error {
	lang = i18n.Normalize(lang)
	cs.current().General.Language = lang
	if err := cs.store.Save(); err != nil {
		return err
	}
	i18n.SetLanguage(lang)
	cs.windows.RefreshTitles()
	if cs.onLanguageChanged != nil {
		cs.onLanguageChanged(lang)
	}
	cs.app.Event.Emit(EventLanguage, lang)
	// Keybinding labels/categories are resolved at List() time.
	cs.app.Event.Emit(EventKeybindings, struct{}{})
	return nil
}

func (cs *ConfigService) ReadConfig() (*Config, error) {
	return cs.current(), nil
}

func (cs *ConfigService) SaveConfig(cfg Config) error {
	if err := cs.store.ApplyUserDataDir(cfg.General.UserDataDir); err != nil {
		return err
	}
	// Theme / Language have dedicated write paths.
	cfg.General.Theme = cs.current().General.Theme
	cfg.General.Language = cs.current().General.Language
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
	// Theme / Language are owned by SetTheme / SetLanguage; keep in-memory values.
	generalConfig.Theme = cs.current().General.Theme
	generalConfig.Language = cs.current().General.Language
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
		SetMessage(i18n.T("dialog.frameless_restart"))
	ok := dlg.AddButton(i18n.T("dialog.ok"))
	dlg.SetDefaultButton(ok)
	dlg.SetCancelButton(ok)
	if cs.windows.Setting != nil {
		dlg.AttachToWindow(cs.windows.Setting)
	}
	dlg.Show()
}

func (cs *ConfigService) SaveTerminalConfig(terminalConfig TerminalConfig) error {
	terminalConfig.Normalize()
	cs.current().Terminal = terminalConfig
	if err := cs.store.Save(); err != nil {
		return err
	}
	cs.app.Event.Emit(EventTerminal, terminalConfig)
	return nil
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
			return usererr.Coded("error.password_not_entered", "Password not entered")
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
		return usererr.Coded("error.password_timeout", "Password input timed out")
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
	return cs.encryptValue(apiKey, i18n.T("password.encrypt_api_key"))
}

func (cs *ConfigService) decryptAPIKey(encrypted string) (string, error) {
	return cs.decryptValue(encrypted, i18n.T("password.decrypt_api_key"))
}
