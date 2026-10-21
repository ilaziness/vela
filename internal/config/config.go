package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/sync"
	"github.com/ilaziness/vela/internal/system"
)

const FileName = "config.toml"

var defaultFontFamily = []string{
	"'Noto Sans Mono'",
	"'ui-monospace'",
	"'Cascadia Code'",
	"Consolas",
	"Menlo",
	"Monaco",
	"'DejaVu Sans Mono'",
	"'Ubuntu Mono'",
	"'Liberation Mono'",
	"'Courier New'",
	"'Microsoft YaHei'",
	"'PingFang SC'",
	"'Heiti SC'",
	"'WenQuanYi Micro Hei'",
	"monospace",
}

type Config struct {
	General  GeneralConfig   `toml:"general"`
	Terminal TerminalConfig  `toml:"terminal"`
	SSH      SSHConfig       `toml:"ssh"`
	Sync     sync.SyncConfig `toml:"sync"`
	AI       ai.Config       `toml:"ai"`
}

type GeneralConfig struct {
	UserDataDir string `toml:"user_data_dir"`
	Theme       string `toml:"theme"`
	Frameless   bool   `toml:"frameless"`
	Font        string `toml:"font"`
	FontSize    int    `toml:"font_size"`
}

type TerminalConfig struct {
	Font            string  `toml:"font" json:"fontFamily"`
	FontSize        int     `toml:"font_size" json:"fontSize"`
	LineHeight      float64 `toml:"line_height" json:"lineHeight"`
	Scrollback      int     `toml:"scrollback" json:"scrollback"`
	Bell            bool    `toml:"bell" json:"bell"`
	CopyOnSelect    bool    `toml:"copy_on_select" json:"copyOnSelect"`
	RightClickPaste bool    `toml:"right_click_paste" json:"rightClickPaste"`
}

const (
	defaultTerminalScrollback = 10000
	minTerminalScrollback     = 100
	maxTerminalScrollback     = 100000
)

// SSHConfig holds global SSH client options (ServerAliveInterval-style keepalive, dial timeout, auto-reconnect, dial proxy).
type SSHConfig struct {
	ServerAliveInterval int    `toml:"server_alive_interval" json:"serverAliveInterval"`
	DialTimeoutSec      int    `toml:"dial_timeout_sec" json:"dialTimeoutSec"`
	AutoReconnect       bool   `toml:"auto_reconnect" json:"autoReconnect"`
	ProxyType           string `toml:"proxy_type" json:"proxyType"`
	ProxyHost           string `toml:"proxy_host" json:"proxyHost"`
	ProxyPort           int    `toml:"proxy_port" json:"proxyPort"`
	ProxyUser           string `toml:"proxy_user" json:"proxyUser"`
	ProxyPassword       string `toml:"proxy_password" json:"proxyPassword"`
}

type AppConfig struct {
	General GeneralConfig `toml:"general"`
}

type Store struct {
	Config         *Config
	appConfig      *AppConfig
	userConfigFile string
	appConfigFile  string
	logger         *zap.Logger
}

func Default() *Config {
	execDir := system.GetExecutableDir()
	return &Config{
		General: GeneralConfig{
			UserDataDir: filepath.Join(execDir, "data"),
			Theme:       "dark",
			FontSize:    14,
		},
		Terminal: TerminalConfig{
			Font:            strings.Join(defaultFontFamily, ","),
			FontSize:        14,
			LineHeight:      1,
			Scrollback:      defaultTerminalScrollback,
			Bell:            true,
			CopyOnSelect:    false,
			RightClickPaste: false,
		},
		SSH: SSHConfig{
			ServerAliveInterval: 30,
			DialTimeoutSec:      30,
			AutoReconnect:       true,
		},
	}
}

// Normalize clamps the UI font size and strips characters that would break CSS.
// An empty font keeps the system stack.
func (c *GeneralConfig) Normalize() {
	c.Font = sanitizeUIFont(c.Font)
	if c.FontSize < 8 || c.FontSize > 72 {
		c.FontSize = 14
	}
}

func sanitizeUIFont(font string) string {
	font = strings.TrimSpace(font)
	if font == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(font))
	for _, r := range font {
		switch r {
		case ';', '{', '}', '<', '>', '\\', '"', '\'':
			continue
		default:
			if r < 0x20 {
				continue
			}
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// Normalize clamps terminal display options. Does not force Bell/CopyOnSelect/
// RightClickPaste defaults — absent TOML keys keep Default() values via merge.
func (c *TerminalConfig) Normalize() {
	def := Default().Terminal
	if c.FontSize < 1 || c.FontSize > 72 {
		c.FontSize = def.FontSize
	}
	if c.LineHeight <= 0 {
		c.LineHeight = def.LineHeight
	}
	switch {
	case c.Scrollback <= 0:
		c.Scrollback = def.Scrollback
	case c.Scrollback < minTerminalScrollback:
		c.Scrollback = minTerminalScrollback
	case c.Scrollback > maxTerminalScrollback:
		c.Scrollback = maxTerminalScrollback
	}
	if strings.TrimSpace(c.Font) == "" {
		c.Font = def.Font
	}
}

// Normalize fills invalid/missing fields with defaults after loading from disk.
func (c *SSHConfig) Normalize() {
	def := Default().SSH
	if *c == (SSHConfig{}) {
		*c = def
		return
	}
	if c.DialTimeoutSec <= 0 {
		c.DialTimeoutSec = def.DialTimeoutSec
	}
	if c.ServerAliveInterval < 0 {
		c.ServerAliveInterval = 0
	}
	switch strings.ToLower(strings.TrimSpace(c.ProxyType)) {
	case "", "http", "socks5":
		c.ProxyType = strings.ToLower(strings.TrimSpace(c.ProxyType))
	default:
		c.ProxyType = ""
	}
	if c.ProxyPort < 0 {
		c.ProxyPort = 0
	}
}

// ValidateProxy returns an error when a dial proxy is enabled but misconfigured.
func (c *SSHConfig) ValidateProxy() error {
	if strings.TrimSpace(c.ProxyType) == "" {
		return nil
	}
	typ := strings.ToLower(strings.TrimSpace(c.ProxyType))
	if typ != "http" && typ != "socks5" {
		return fmt.Errorf("不支持的代理类型: %s", c.ProxyType)
	}
	if strings.TrimSpace(c.ProxyHost) == "" {
		return fmt.Errorf("代理主机不能为空")
	}
	if c.ProxyPort < 1 || c.ProxyPort > 65535 {
		return fmt.Errorf("代理端口必须在 1–65535 之间")
	}
	return nil
}

func Load(logger *zap.Logger) (*Store, error) {
	execDir := system.GetExecutableDir()
	appConfigPath := filepath.Join(execDir, FileName)
	defaultConfig := Default()

	appConfig := &AppConfig{General: defaultConfig.General}
	if data, err := os.ReadFile(appConfigPath); err == nil {
		_ = toml.Unmarshal(data, appConfig)
	} else if os.IsNotExist(err) {
		if data, err := toml.Marshal(appConfig); err == nil {
			_ = os.WriteFile(appConfigPath, data, 0600)
		}
	}

	userDataDir := appConfig.General.UserDataDir
	if userDataDir == "" {
		userDataDir = defaultConfig.General.UserDataDir
	}
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return nil, fmt.Errorf("create user data dir: %w", err)
	}

	userConfigPath := filepath.Join(userDataDir, FileName)
	finalConfig := defaultConfig
	finalConfig.General.UserDataDir = userDataDir

	if data, err := os.ReadFile(userConfigPath); err == nil {
		if err = toml.Unmarshal(data, finalConfig); err != nil {
			logger.Error("unmarshal user config failed", zap.Error(err))
		}
		finalConfig.SSH.Normalize()
	} else if os.IsNotExist(err) {
		if data, err := toml.Marshal(finalConfig); err == nil {
			_ = os.WriteFile(userConfigPath, data, 0600)
		}
	}

	finalConfig.General.Normalize()
	finalConfig.Terminal.Normalize()

	logger.Debug("config loaded", zap.String("appConfigPath", appConfigPath), zap.String("userConfigPath", userConfigPath))

	return &Store{
		Config:         finalConfig,
		appConfig:      appConfig,
		userConfigFile: userConfigPath,
		appConfigFile:  appConfigPath,
		logger:         logger,
	}, nil
}

func (s *Store) UserDataDir() string {
	return s.Config.General.UserDataDir
}

func (s *Store) Save() error {
	s.logger.Debug("save config to file", zap.String("path", s.userConfigFile))
	data, err := toml.Marshal(s.Config)
	if err != nil {
		s.logger.Error("save config to file failed", zap.Error(err))
		return fmt.Errorf("marshal config failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.userConfigFile), 0755); err != nil {
		s.logger.Error("create user config dir failed", zap.Error(err))
		return fmt.Errorf("create user config dir failed: %w", err)
	}
	if err := os.WriteFile(s.userConfigFile, data, 0600); err != nil {
		s.logger.Error("save config to file failed", zap.Error(err))
		return fmt.Errorf("write user config file failed: %w", err)
	}
	return nil
}

func (s *Store) SaveAppConfig() error {
	data, err := toml.Marshal(s.appConfig)
	if err != nil {
		return err
	}
	return os.WriteFile(s.appConfigFile, data, 0600)
}

func (s *Store) ApplyUserDataDir(dir string) error {
	if dir == s.Config.General.UserDataDir {
		return nil
	}
	s.appConfig = &AppConfig{General: GeneralConfig{UserDataDir: dir}}
	if err := s.SaveAppConfig(); err != nil {
		return fmt.Errorf("write app config file failed: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create user data dir failed: %w", err)
	}
	s.userConfigFile = filepath.Join(dir, FileName)
	s.Config.General.UserDataDir = dir
	return nil
}
