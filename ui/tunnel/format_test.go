package tunnelui

import (
	"testing"

	"github.com/ilaziness/vela/internal/tunnel"
)

func TestInfoLine(t *testing.T) {
	cases := []struct {
		info tunnel.TunnelInfo
		want string
	}{
		{
			info: tunnel.TunnelInfo{TunnelType: tunnel.TypeLocal, LocalAddr: "127.0.0.1:8080", RemoteAddr: "db:3306"},
			want: "127.0.0.1:8080 → db:3306",
		},
		{
			info: tunnel.TunnelInfo{TunnelType: tunnel.TypeRemote, LocalAddr: "localhost:6379", RemoteAddr: "127.0.0.1:6380"},
			want: "localhost:6379 → 127.0.0.1:6380",
		},
		{
			info: tunnel.TunnelInfo{TunnelType: tunnel.TypeDynamic, LocalAddr: "127.0.0.1:1080"},
			want: "127.0.0.1:1080",
		},
	}
	for _, tc := range cases {
		if got := infoLine(tc.info); got != tc.want {
			t.Errorf("infoLine(%+v) = %q, want %q", tc.info, got, tc.want)
		}
	}
}

func TestTypeIcon(t *testing.T) {
	if typeIcon(tunnel.TypeLocal) == typeIcon(tunnel.TypeRemote) {
		t.Error("local and remote share an icon")
	}
	if typeIcon(tunnel.TypeDynamic) == "" {
		t.Error("dynamic icon empty")
	}
}
