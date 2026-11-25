package mcpserver

import "testing"

func TestHashAccessKey(t *testing.T) {
	h1 := HashAccessKey("secret-token")
	h2 := HashAccessKey(" secret-token ")
	h3 := HashAccessKey("other")
	if h1 == "" || h1 != h2 {
		t.Fatalf("hash should be stable and trim whitespace: %q vs %q", h1, h2)
	}
	if h1 == h3 {
		t.Fatal("different keys must hash differently")
	}
}

func TestAuthorized(t *testing.T) {
	hash := HashAccessKey("s3cret")
	cases := []struct {
		name   string
		header string
		hash   string
		want   bool
	}{
		{"no key configured allows all", "", "", true},
		{"no key configured ignores header", "Bearer x", "", true},
		{"missing header", "", hash, false},
		{"wrong scheme", "Basic abc", hash, false},
		{"wrong token", "Bearer wrong", hash, false},
		{"correct token", "Bearer s3cret", hash, true},
		{"correct token trimmed", "Bearer  s3cret ", hash, true},
	}
	for _, c := range cases {
		if got := authorized(c.header, c.hash); got != c.want {
			t.Errorf("%s: authorized(%q, %q) = %v, want %v", c.name, c.header, c.hash, got, c.want)
		}
	}
}
