package services

import (
	"testing"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/ssh"
)

func TestResolveDialProxy(t *testing.T) {
	global := config.SSHConfig{
		ProxyType: "http", ProxyHost: "g.example", ProxyPort: 8080,
		ProxyUser: "gu", ProxyPassword: "gp",
	}
	t.Run("inherit", func(t *testing.T) {
		p, err := resolveDialProxy("inherit", ssh.ProxyConfig{}, global)
		if err != nil {
			t.Fatal(err)
		}
		if !p.Enabled() || p.Type != "http" || p.Host != "g.example" || p.Port != 8080 || p.User != "gu" || p.Password != "gp" {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("none", func(t *testing.T) {
		p, err := resolveDialProxy("none", ssh.ProxyConfig{Type: "socks5", Host: "x", Port: 1080}, global)
		if err != nil {
			t.Fatal(err)
		}
		if p.Enabled() {
			t.Fatalf("expected direct, got %+v", p)
		}
	})
	t.Run("custom", func(t *testing.T) {
		p, err := resolveDialProxy("custom", ssh.ProxyConfig{
			Type: "socks5", Host: "127.0.0.1", Port: 1080, User: "u", Password: "p",
		}, global)
		if err != nil {
			t.Fatal(err)
		}
		if p.Type != "socks5" || p.Host != "127.0.0.1" || p.Port != 1080 || p.User != "u" || p.Password != "p" {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("empty mode inherits", func(t *testing.T) {
		p, err := resolveDialProxy("", ssh.ProxyConfig{}, global)
		if err != nil {
			t.Fatal(err)
		}
		if p.Host != "g.example" {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("custom incomplete errors", func(t *testing.T) {
		_, err := resolveDialProxy("custom", ssh.ProxyConfig{Type: "socks5", Port: 1080}, global)
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("inherit incomplete global errors", func(t *testing.T) {
		_, err := resolveDialProxy("inherit", ssh.ProxyConfig{}, config.SSHConfig{ProxyType: "http", ProxyPort: 8080})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("inherit empty global is direct", func(t *testing.T) {
		p, err := resolveDialProxy("inherit", ssh.ProxyConfig{}, config.SSHConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if p.Enabled() {
			t.Fatalf("%+v", p)
		}
	})
}

func TestValidateBookmarkProxy(t *testing.T) {
	if err := validateBookmarkProxy(SSHBookmark{ProxyMode: "inherit"}); err != nil {
		t.Fatal(err)
	}
	if err := validateBookmarkProxy(SSHBookmark{ProxyMode: "none"}); err != nil {
		t.Fatal(err)
	}
	if err := validateBookmarkProxy(SSHBookmark{
		ProxyMode: "custom", ProxyType: "http", ProxyHost: "p", ProxyPort: 3128,
	}); err != nil {
		t.Fatal(err)
	}
	if err := validateBookmarkProxy(SSHBookmark{ProxyMode: "custom", ProxyType: "http"}); err == nil {
		t.Fatal("expected error")
	}
}
