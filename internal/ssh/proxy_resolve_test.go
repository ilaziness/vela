package ssh

import "testing"

func TestResolveDialProxy(t *testing.T) {
	global := GlobalProxy{
		Type: "http", Host: "g.example", Port: 8080,
		User: "gu", Password: "gp",
	}
	t.Run("inherit", func(t *testing.T) {
		p, err := ResolveDialProxy("inherit", ProxyConfig{}, global)
		if err != nil {
			t.Fatal(err)
		}
		if !p.Enabled() || p.Type != "http" || p.Host != "g.example" || p.Port != 8080 || p.User != "gu" || p.Password != "gp" {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("none", func(t *testing.T) {
		p, err := ResolveDialProxy("none", ProxyConfig{Type: "socks5", Host: "x", Port: 1080}, global)
		if err != nil {
			t.Fatal(err)
		}
		if p.Enabled() {
			t.Fatalf("expected direct, got %+v", p)
		}
	})
	t.Run("custom", func(t *testing.T) {
		p, err := ResolveDialProxy("custom", ProxyConfig{
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
		p, err := ResolveDialProxy("", ProxyConfig{}, global)
		if err != nil {
			t.Fatal(err)
		}
		if p.Host != "g.example" {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("custom incomplete errors", func(t *testing.T) {
		_, err := ResolveDialProxy("custom", ProxyConfig{Type: "socks5", Port: 1080}, global)
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("inherit incomplete global errors", func(t *testing.T) {
		_, err := ResolveDialProxy("inherit", ProxyConfig{}, GlobalProxy{Type: "http", Port: 8080})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("inherit empty global is direct", func(t *testing.T) {
		p, err := ResolveDialProxy("inherit", ProxyConfig{}, GlobalProxy{})
		if err != nil {
			t.Fatal(err)
		}
		if p.Enabled() {
			t.Fatalf("%+v", p)
		}
	})
}
