package services

import (
	"strings"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/ssh"
)

// resolveDialProxy picks the effective dial proxy: bookmark custom > none > inherit global.
func resolveDialProxy(mode string, custom ssh.ProxyConfig, global config.SSHConfig) (ssh.ProxyConfig, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return ssh.ProxyConfig{}, nil
	case "custom":
		custom.Type = strings.ToLower(strings.TrimSpace(custom.Type))
		custom.Host = strings.TrimSpace(custom.Host)
		if err := custom.Validate(i18n.T("bookmark.custom_proxy")); err != nil {
			return ssh.ProxyConfig{}, err
		}
		return custom, nil
	default: // inherit
		if strings.TrimSpace(global.ProxyType) == "" {
			return ssh.ProxyConfig{}, nil
		}
		p := ssh.ProxyConfig{
			Type:     strings.ToLower(strings.TrimSpace(global.ProxyType)),
			Host:     strings.TrimSpace(global.ProxyHost),
			Port:     global.ProxyPort,
			User:     global.ProxyUser,
			Password: global.ProxyPassword,
		}
		if err := p.Validate(i18n.T("proxy.label_default")); err != nil {
			return ssh.ProxyConfig{}, err
		}
		return p, nil
	}
}

func (s *SSHService) globalDialProxy() (ssh.ProxyConfig, error) {
	if s == nil || s.getSSHConfig == nil {
		return ssh.ProxyConfig{}, nil
	}
	return resolveDialProxy("inherit", ssh.ProxyConfig{}, s.getSSHConfig())
}
