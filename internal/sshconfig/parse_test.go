package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHostStarMerge(t *testing.T) {
	// OpenSSH: first obtained value wins — put Host * after specifics.
	src := `
Host foo
  HostName 10.0.0.1
  User alice
  Port 22

Host bar baz
  HostName 10.0.0.2

Host *
  User default
  Port 2222
  IdentityFile /tmp/id_ed25519
`
	blocks, warns, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, rWarns := Resolve(blocks)
	warns = append(warns, rWarns...)
	byAlias := map[string]HostEntry{}
	for _, e := range entries {
		byAlias[e.Alias] = e
	}
	foo := byAlias["foo"]
	if foo.HostName != "10.0.0.1" || foo.User != "alice" || foo.Port != 22 {
		t.Fatalf("foo=%+v warns=%v", foo, warns)
	}
	if len(foo.IdentityFiles) != 1 {
		t.Fatalf("foo IdentityFiles=%v", foo.IdentityFiles)
	}
	bar := byAlias["bar"]
	if bar.User != "default" || bar.Port != 2222 || bar.HostName != "10.0.0.2" {
		t.Fatalf("bar=%+v", bar)
	}
	baz := byAlias["baz"]
	if baz.HostName != "10.0.0.2" {
		t.Fatalf("baz=%+v", baz)
	}
}

func TestSetEnvMergeAcrossHostBlocks(t *testing.T) {
	src := `
Host foo
  HostName 10.0.0.1
  SetEnv FOO=from-foo
  SetEnv KEEP=1

Host *
  SetEnv FOO=from-star
  SetEnv BAR=from-star
`
	blocks, _, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := Resolve(blocks)
	var foo HostEntry
	for _, e := range entries {
		if e.Alias == "foo" {
			foo = e
		}
	}
	got := map[string]string{}
	for _, kv := range foo.SetEnv {
		got[kv.Key] = kv.Value
	}
	if got["FOO"] != "from-foo" {
		t.Fatalf("FOO first-wins: %v", got)
	}
	if got["BAR"] != "from-star" || got["KEEP"] != "1" {
		t.Fatalf("expected merge from Host *: %v", got)
	}
}

func TestProxyJumpAndSetEnv(t *testing.T) {
	src := `
Host jump
  HostName j.example
  User j

Host target
  HostName t.example
  ProxyJump jump,other
  SetEnv FOO=bar
  SetEnv BAZ=1
  ForwardAgent yes
  RemoteCommand cd /tmp
  ProxyCommand nc %h %p
`
	blocks, warns, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, rWarns := Resolve(blocks)
	warns = append(warns, rWarns...)
	var target HostEntry
	for _, e := range entries {
		if e.Alias == "target" {
			target = e
		}
	}
	if target.ProxyJump != "jump,other" {
		t.Fatalf("ProxyJump=%q", target.ProxyJump)
	}
	if target.RemoteCommand != "cd /tmp" || target.ForwardAgent == nil || !*target.ForwardAgent {
		t.Fatalf("%+v", target)
	}
	if len(target.SetEnv) != 2 {
		t.Fatalf("SetEnv=%v", target.SetEnv)
	}
	foundProxyCmd := false
	for _, w := range warns {
		if strings.Contains(w.Message, "ProxyCommand") {
			foundProxyCmd = true
		}
	}
	if !foundProxyCmd {
		t.Fatalf("expected ProxyCommand warning: %v", warns)
	}
}

func TestStandardDirectivesDoNotWarn(t *testing.T) {
	// Standard OpenSSH options without a bookmark equivalent are skipped
	// quietly; only functionality that cannot be imported should warn.
	src := `
Host github.com
  HostName github.com
  PreferredAuthentications publickey
  IdentitiesOnly yes
  HostKeyAlgorithms ssh-ed25519
  PubkeyAcceptedKeyTypes ssh-ed25519
  ServerAliveInterval 30
`
	_, warns, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings = %v", warns)
	}
}

func TestWriteRoundTrip(t *testing.T) {
	yes := true
	entries := []HostEntry{{
		Alias: "foo", HostName: "1.2.3.4", User: "u", Port: 2222,
		IdentityFiles: []string{"/tmp/id"}, ForwardAgent: &yes,
		RemoteCommand: "uname", SetEnv: []KV{{Key: "A", Value: "b"}},
		ProxyJump: "jump",
	}}
	var b strings.Builder
	if err := Write(&b, entries); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "Host foo") || !strings.Contains(out, "ProxyJump jump") {
		t.Fatal(out)
	}
}

func TestWriteQuotedHostAlias(t *testing.T) {
	var b strings.Builder
	if err := Write(&b, []HostEntry{{
		Alias: "my host", HostName: "10.0.0.1",
		IdentityFiles: []string{`/tmp/my key`},
		RemoteCommand: "pwd && whoami",
		SetEnv:        []KV{{Key: "FOO", Value: "a b"}},
	}}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, `Host "my host"`) {
		t.Fatalf("host alias: %s", out)
	}
	if !strings.Contains(out, `IdentityFile "/tmp/my key"`) {
		t.Fatalf("identity: %s", out)
	}
	if !strings.Contains(out, `RemoteCommand "pwd && whoami"`) {
		t.Fatalf("remote: %s", out)
	}
	blocks, _, err := Parse(strings.NewReader(out), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := Resolve(blocks)
	if len(entries) != 1 || entries[0].Alias != "my host" {
		t.Fatalf("%+v", entries)
	}
	if entries[0].RemoteCommand != "pwd && whoami" {
		t.Fatalf("remote=%q", entries[0].RemoteCommand)
	}
	if len(entries[0].IdentityFiles) != 1 || entries[0].IdentityFiles[0] != "/tmp/my key" {
		t.Fatalf("id=%v", entries[0].IdentityFiles)
	}
}

func TestQuotedPortAndIdentity(t *testing.T) {
	src := `
Host foo
  HostName 10.0.0.1
  Port "2222"
  IdentityFile "/tmp/quoted key"
`
	blocks, _, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, warns := Resolve(blocks)
	var foo HostEntry
	for _, e := range entries {
		if e.Alias == "foo" {
			foo = e
		}
	}
	if foo.Port != 2222 {
		t.Fatalf("port=%d warns=%v", foo.Port, warns)
	}
	if len(foo.IdentityFiles) != 1 || foo.IdentityFiles[0] != "/tmp/quoted key" {
		t.Fatalf("identity=%v", foo.IdentityFiles)
	}
}

func TestQuoteHelpers(t *testing.T) {
	if quoteSSH("ab") != "ab" || quoteSSH("a b") != `"a b"` {
		t.Fatal(quoteSSH("a b"))
	}
	if unquoteSSH(`"a b"`) != "a b" {
		t.Fatal(unquoteSSH(`"a b"`))
	}
	got := splitSSHTokens(`foo "bar baz" qux`)
	if len(got) != 3 || got[0] != "foo" || got[1] != "bar baz" || got[2] != "qux" {
		t.Fatalf("%v", got)
	}
	only := splitSSHTokens(`"my host"`)
	if len(only) != 1 || only[0] != "my host" {
		t.Fatalf("trailing empty token: %q", only)
	}
}

func TestInclude(t *testing.T) {
	dir := t.TempDir()
	inc := filepath.Join(dir, "extra")
	if err := os.WriteFile(inc, []byte("Host inc\n  HostName 9.9.9.9\n  User iu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "config")
	if err := os.WriteFile(main, []byte("Include extra\nHost main\n  HostName 1.1.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocks, _, err := ParseFile(main)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := Resolve(blocks)
	found := map[string]bool{}
	for _, e := range entries {
		found[e.Alias] = true
	}
	if !found["inc"] || !found["main"] {
		t.Fatalf("%v", found)
	}
}

func TestBareHostAliasImports(t *testing.T) {
	src := "Host myserver\n"
	blocks, _, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := Resolve(blocks)
	if len(entries) != 1 || entries[0].Alias != "myserver" {
		t.Fatalf("%+v", entries)
	}
}

func TestHostNegationPattern(t *testing.T) {
	src := `
Host * !foo
  User default

Host foo
  HostName 1.1.1.1
  User alice

Host bar
  HostName 2.2.2.2
`
	blocks, _, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := Resolve(blocks)
	by := map[string]HostEntry{}
	for _, e := range entries {
		by[e.Alias] = e
	}
	if by["foo"].User != "alice" {
		t.Fatalf("foo should not inherit Host * !foo defaults: %+v", by["foo"])
	}
	if by["bar"].User != "default" {
		t.Fatalf("bar=%+v", by["bar"])
	}
}

func TestInvalidPortDoesNotBlockLater(t *testing.T) {
	src := `
Host foo
  Port notanumber
  HostName 1.1.1.1

Host foo
  Port 2222
`
	blocks, _, err := Parse(strings.NewReader(src), ".", "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := Resolve(blocks)
	if len(entries) != 1 || entries[0].Port != 2222 {
		t.Fatalf("%+v", entries)
	}
}

func TestExpandPathHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	got, err := ExpandPath("~/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "foo", "bar")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
