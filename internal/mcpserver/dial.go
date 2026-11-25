package mcpserver

import (
	"strings"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/usererr"
)

// resolveBookmarkDial mirrors the UI bookmark connect path
// (GetMasked → ValidateProxy → PrepareTest → ResolveHops → ResolveDialProxy)
// but stops before opening a terminal session, returning the hop chain and
// resolved proxy for ExecHops / TestConnect.
func (d *Deps) resolveBookmarkDial(id string) ([]ssh.Endpoint, ssh.ProxyConfig, error) {
	if d.Bookmarks == nil {
		return nil, ssh.ProxyConfig{}, usererr.Coded("mcp.bookmarks_unavailable", "Bookmark service is not available")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ssh.ProxyConfig{}, usererr.Coded("mcp.bookmark_id_required", "bookmark_id is required")
	}
	b, err := d.Bookmarks.GetMasked(id)
	if err != nil {
		return nil, ssh.ProxyConfig{}, err
	}
	if proto, ok := termio.ParseProtocol(b.Protocol); !ok || proto != termio.ProtocolSSH {
		return nil, ssh.ProxyConfig{}, usererr.Coded("mcp.bookmark_not_ssh", "Bookmark %q is not an SSH bookmark", b.Title)
	}
	if err := bookmark.ValidateProxy(*b); err != nil {
		return nil, ssh.ProxyConfig{}, err
	}
	prep, err := d.Bookmarks.PrepareTest(*b)
	if err != nil {
		return nil, ssh.ProxyConfig{}, err
	}
	hops, err := d.Bookmarks.ResolveHops(prep.Endpoint, prep.JumpID)
	if err != nil {
		return nil, ssh.ProxyConfig{}, err
	}
	proxy, err := ssh.ResolveDialProxy(b.ProxyMode, ssh.ProxyConfig{
		Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort,
		User: b.ProxyUser, Password: prep.ProxyPassword,
	}, d.globalProxy())
	if err != nil {
		return nil, ssh.ProxyConfig{}, err
	}
	return hops, proxy, nil
}

// adhocTarget is the connection info accepted by run_command / test_connection.
type adhocTarget struct {
	Host          string
	Port          int
	User          string
	Password      string
	PrivateKeyPEM string
	KeyID         string
}

// resolveAdhocDial builds a single-hop chain from caller-supplied connection
// info. Authentication accepts any combination of password, inline private
// key PEM, and a stored application key id.
func (d *Deps) resolveAdhocDial(t adhocTarget) ([]ssh.Endpoint, ssh.ProxyConfig, error) {
	host := strings.TrimSpace(t.Host)
	user := strings.TrimSpace(t.User)
	if host == "" || user == "" {
		return nil, ssh.ProxyConfig{}, usererr.Coded("mcp.ssh_fields_required", "host and user are required")
	}
	port := t.Port
	if port <= 0 {
		port = 22
	}
	if port > 65535 {
		return nil, ssh.ProxyConfig{}, usererr.Coded("mcp.port_invalid", "port must be between 1 and 65535")
	}
	ep := ssh.Endpoint{
		Host:     host,
		Port:     port,
		User:     user,
		Password: t.Password,
	}
	keyID := strings.TrimSpace(t.KeyID)
	keyPEM := strings.TrimSpace(t.PrivateKeyPEM)
	if keyID != "" {
		if d.Keys == nil {
			return nil, ssh.ProxyConfig{}, usererr.Coded("mcp.keys_unavailable", "SSH key service is not available")
		}
		pem, err := d.Keys.PrivatePEM(keyID)
		if err != nil {
			return nil, ssh.ProxyConfig{}, err
		}
		ep.KeyPEM = pem
	} else if keyPEM != "" {
		ep.KeyPEM = keyPEM
	}
	proxy, err := ssh.ResolveDialProxy("inherit", ssh.ProxyConfig{}, d.globalProxy())
	if err != nil {
		return nil, ssh.ProxyConfig{}, err
	}
	return []ssh.Endpoint{ep}, proxy, nil
}

func (d *Deps) globalProxy() ssh.GlobalProxy {
	if d.GlobalProxy == nil {
		return ssh.GlobalProxy{}
	}
	return d.GlobalProxy()
}
