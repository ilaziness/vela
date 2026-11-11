package ssh

import (
	"strings"

	"github.com/ilaziness/vela/internal/i18n"
)

// GlobalProxy is the dial-proxy subset of app SSH settings.
// Kept here (not config.SSHConfig) to avoid import cycles with config/ai.
type GlobalProxy struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
}

// ResolveDialProxy picks the effective dial proxy: bookmark custom > none > inherit global.
func ResolveDialProxy(mode string, custom ProxyConfig, global GlobalProxy) (ProxyConfig, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return ProxyConfig{}, nil
	case "custom":
		custom.Type = strings.ToLower(strings.TrimSpace(custom.Type))
		custom.Host = strings.TrimSpace(custom.Host)
		if err := custom.Validate(i18n.T("bookmark.custom_proxy")); err != nil {
			return ProxyConfig{}, err
		}
		return custom, nil
	default: // inherit
		if strings.TrimSpace(global.Type) == "" {
			return ProxyConfig{}, nil
		}
		p := ProxyConfig{
			Type:     strings.ToLower(strings.TrimSpace(global.Type)),
			Host:     strings.TrimSpace(global.Host),
			Port:     global.Port,
			User:     global.User,
			Password: global.Password,
		}
		if err := p.Validate(i18n.T("proxy.label_default")); err != nil {
			return ProxyConfig{}, err
		}
		return p, nil
	}
}
