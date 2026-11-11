package bookmark

import (
	"strings"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/usererr"
)

// ValidateProxy checks ProxyMode / custom proxy fields on a bookmark.
func ValidateProxy(b Bookmark) error {
	mode := strings.ToLower(strings.TrimSpace(b.ProxyMode))
	if mode == "" {
		mode = "inherit"
	}
	switch mode {
	case "inherit", "none":
		return nil
	case "custom":
		p := ssh.ProxyConfig{Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort}
		return p.Validate(i18n.T("bookmark.custom_proxy"))
	default:
		return usererr.Coded("bookmark.proxy_mode_invalid", "Invalid proxy mode: %s", b.ProxyMode)
	}
}
