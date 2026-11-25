package app

import (
	"testing"

	"github.com/ilaziness/vela/internal/tunnel"
)

func TestTunnelArgsFromDraft(t *testing.T) {
	args, err := tunnelArgsFromDraft(TunnelDraft{
		Type: tunnel.TypeLocal, LocalAddr: "127.0.0.1:8080", RemoteAddr: "db:3306",
	})
	if err != nil {
		t.Fatalf("local draft: %v", err)
	}
	if args.LocalAddr != "127.0.0.1:8080" || args.RemoteAddr != "db:3306" {
		t.Fatalf("args = %+v", args)
	}

	args, err = tunnelArgsFromDraft(TunnelDraft{
		Type: tunnel.TypeRemote, LocalAddr: "localhost:6379", RemotePort: "6380",
	})
	if err != nil {
		t.Fatalf("remote draft: %v", err)
	}
	if args.RemotePort != 6380 {
		t.Fatalf("remote port = %d", args.RemotePort)
	}

	if _, err = tunnelArgsFromDraft(TunnelDraft{
		Type: tunnel.TypeRemote, LocalAddr: "localhost:6379", RemotePort: "",
	}); err == nil {
		t.Fatal("empty remote port accepted")
	}
	if _, err = tunnelArgsFromDraft(TunnelDraft{
		Type: tunnel.TypeRemote, LocalAddr: "localhost:6379", RemotePort: "abc",
	}); err == nil {
		t.Fatal("non-numeric remote port accepted")
	}
	if _, err = tunnelArgsFromDraft(TunnelDraft{
		Type: tunnel.TypeLocal, LocalAddr: "localhost:8080", RemoteAddr: "db:3306",
	}); err == nil {
		t.Fatal("hostname listen addr accepted")
	}
	if _, err = tunnelArgsFromDraft(TunnelDraft{Type: "bogus"}); err == nil {
		t.Fatal("unknown type accepted")
	}
}

func TestValidateTunnelDraft(t *testing.T) {
	if msg := ValidateTunnelDraft(TunnelDraft{
		Type: tunnel.TypeDynamic, LocalAddr: "127.0.0.1:1080",
	}); msg != "" {
		t.Fatalf("dynamic draft invalid: %s", msg)
	}
	if msg := ValidateTunnelDraft(TunnelDraft{
		Type: tunnel.TypeLocal, LocalAddr: "127.0.0.1:8080",
	}); msg == "" {
		t.Fatal("missing remote addr accepted")
	}
}

func TestRuleMatchesInfo(t *testing.T) {
	rule := TunnelRule{
		Type: tunnel.TypeRemote, LocalAddr: "localhost:6379", RemotePort: 6380,
	}
	match := tunnel.TunnelInfo{
		TunnelType: tunnel.TypeRemote, LocalAddr: "localhost:6379", RemoteAddr: "127.0.0.1:6380",
	}
	if !ruleMatchesInfo(rule, match) {
		t.Fatal("remote rule should match")
	}
	match.RemoteAddr = "127.0.0.1:6381"
	if ruleMatchesInfo(rule, match) {
		t.Fatal("remote rule matched wrong port")
	}

	local := TunnelRule{Type: tunnel.TypeLocal, LocalAddr: "127.0.0.1:8080", RemoteAddr: "db:3306"}
	info := tunnel.TunnelInfo{TunnelType: tunnel.TypeLocal, LocalAddr: "127.0.0.1:8080", RemoteAddr: "db:3306"}
	if !ruleMatchesInfo(local, info) {
		t.Fatal("local rule should match")
	}
	info.RemoteAddr = "other:3306"
	if ruleMatchesInfo(local, info) {
		t.Fatal("local rule matched wrong target")
	}

	dyn := TunnelRule{Type: tunnel.TypeDynamic, LocalAddr: "127.0.0.1:1080"}
	dynInfo := tunnel.TunnelInfo{TunnelType: tunnel.TypeDynamic, LocalAddr: "127.0.0.1:1080"}
	if !ruleMatchesInfo(dyn, dynInfo) {
		t.Fatal("dynamic rule should match")
	}
}

func TestSortTunnelInfos(t *testing.T) {
	list := []tunnel.TunnelInfo{
		{TunnelType: tunnel.TypeDynamic, LocalAddr: "127.0.0.1:1080"},
		{TunnelType: tunnel.TypeRemote, LocalAddr: "a:1", RemoteAddr: "127.0.0.1:9"},
		{TunnelType: tunnel.TypeLocal, LocalAddr: "127.0.0.1:9000", RemoteAddr: "x:1"},
		{TunnelType: tunnel.TypeLocal, LocalAddr: "127.0.0.1:8000", RemoteAddr: "x:1"},
	}
	sortTunnelInfos(list)
	want := []string{tunnel.TypeLocal, tunnel.TypeLocal, tunnel.TypeRemote, tunnel.TypeDynamic}
	for i, w := range want {
		if list[i].TunnelType != w {
			t.Fatalf("position %d type = %s, want %s", i, list[i].TunnelType, w)
		}
	}
	if list[0].LocalAddr != "127.0.0.1:8000" {
		t.Fatalf("local addrs not sorted: %s", list[0].LocalAddr)
	}
}
