package ssh

import (
	"strings"
	"testing"
)

func TestParseOSC7(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"file:///home/u", "/home/u"},
		{"file://host/home/u", "/home/u"},
		{"file:///", "/"},
		{"file://", ""},
		{"file:///tmp/a%20b", "/tmp/a b"},
		{"http:///home", ""},
		{"file://host", ""},
		{"file:///relative", "/relative"},
	}
	for _, tc := range cases {
		if got := parseOSC7(tc.in); got != tc.want {
			t.Fatalf("parseOSC7(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseOSC1337(t *testing.T) {
	if got := parseOSC1337("CurrentDir=/var/log"); got != "/var/log" {
		t.Fatal(got)
	}
	if got := parseOSC1337("RemoteHost=x;CurrentDir=/tmp;Shell=bash"); got != "/tmp" {
		t.Fatal(got)
	}
	if parseOSC1337("Shell=bash") != "" {
		t.Fatal("expected empty")
	}
	if parseOSC1337("CurrentDir=relative") != "" {
		t.Fatal("relative path rejected")
	}
}

func TestParseOSC633(t *testing.T) {
	if got := parseOSC633("P;Cwd=/opt/app"); got != "/opt/app" {
		t.Fatal(got)
	}
	if got := parseOSC633(`P;Cwd=/tmp/a\x20b`); got != "/tmp/a b" {
		t.Fatal(got)
	}
	if got := parseOSC633(`P;Cwd=/tmp/a\\b`); got != `/tmp/a\b` {
		t.Fatal(got)
	}
	if got := parseOSC633(`P;Cwd=/tmp/a\x3bb`); got != "/tmp/a;b" {
		t.Fatal(got)
	}
	if got := parseOSC633("P;Cwd=/opt;IsWindows=True"); got != "/opt" {
		t.Fatal(got)
	}
	if parseOSC633("A") != "" {
		t.Fatal("not a property")
	}
	if parseOSC633("P;IsWindows=True") != "" {
		t.Fatal("other property")
	}
}

func TestCwdParserBELAndST(t *testing.T) {
	var p cwdParser
	seq := "\x1b]7;file:///home/me\x07"
	if got := p.Feed([]byte(seq)); got != "/home/me" {
		t.Fatalf("BEL: %q", got)
	}
	p = cwdParser{}
	seq = "\x1b]7;file:///home/me\x1b\\"
	if got := p.Feed([]byte(seq)); got != "/home/me" {
		t.Fatalf("ST: %q", got)
	}
}

func TestCwdParserFragmented(t *testing.T) {
	var p cwdParser
	chunks := []string{
		"hello\x1b",
		"]7;file://",
		"/tmp/x",
		"\x07tail",
	}
	var got string
	for _, c := range chunks {
		if path := p.Feed([]byte(c)); path != "" {
			got = path
		}
	}
	if got != "/tmp/x" {
		t.Fatalf("got %q", got)
	}
}

func TestCwdParserFragmentedST(t *testing.T) {
	var p cwdParser
	if p.Feed([]byte("\x1b]1337;CurrentDir=/a\x1b")) != "" {
		t.Fatal("incomplete ST")
	}
	if got := p.Feed([]byte("\\more")); got != "/a" {
		t.Fatalf("got %q", got)
	}
}

func TestCwdParserOSC633And1337(t *testing.T) {
	var p cwdParser
	if got := p.Feed([]byte("\x1b]633;P;Cwd=/srv\x07")); got != "/srv" {
		t.Fatal(got)
	}
	if got := p.Feed([]byte("\x1b]1337;CurrentDir=/srv/www\x07")); got != "/srv/www" {
		t.Fatal(got)
	}
}

func TestCwdParserIgnoresNoise(t *testing.T) {
	var p cwdParser
	if p.Feed([]byte("\x1b[32mgreen\x1b[0m")) != "" {
		t.Fatal("CSI should not yield cwd")
	}
	if p.Feed([]byte("\x1b]0;title\x07")) != "" {
		t.Fatal("OSC 0 ignored")
	}
	if p.Feed([]byte("\x1b]7;file://\x07")) != "" {
		t.Fatal("invalid OSC 7 ignored")
	}
}

func TestCwdParserLastWins(t *testing.T) {
	var p cwdParser
	data := []byte("\x1b]7;file:///a\x07\x1b]7;file:///b\x07")
	if got := p.Feed(data); got != "/b" {
		t.Fatal(got)
	}
}

func TestShellStartCommand(t *testing.T) {
	s := shellStartCommand()
	for _, needle := range []string{
		"VTE_VERSION=" + vteVersionValue,
		"PROMPT_COMMAND=",
		`\033]7;file://`,
		`"${SHELL:-bash}" -l`,
	} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %q in %q", needle, s)
		}
	}
}
