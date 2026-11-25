package tool

import (
	"context"
	"strings"
	"testing"
)

func TestTruncateTail(t *testing.T) {
	if got := truncateTail("abc", 10); got != "abc" {
		t.Fatalf("short=%q", got)
	}
	got := truncateTail("hello world", 5)
	if !strings.HasSuffix(got, "world") || !strings.HasPrefix(got, "…[truncated]") {
		t.Fatalf("got %q", got)
	}
	// UTF-8 boundary: a cut inside a multi-byte rune must not produce invalid UTF-8.
	got = truncateTail(strings.Repeat("界", 100), 10)
	if !strings.HasSuffix(got, "界界界") {
		t.Fatalf("utf8 tail=%q", got)
	}
}

func TestTerminalExecSet(t *testing.T) {
	s := NewTerminalExecSet()
	if s.Has("c1") {
		t.Fatal("empty set has c1")
	}
	s.Mark("c1")
	if !s.Has("c1") || s.Has("c2") {
		t.Fatal("mark/has mismatch")
	}
	var nilSet *TerminalExecSet
	nilSet.Mark("x") // must not panic
	if nilSet.Has("x") {
		t.Fatal("nil set reports hit")
	}
}

func TestCallIDRoundTrip(t *testing.T) {
	ctx := WithCallID(context.Background(), "call-1")
	if got := CallIDFromCtx(ctx); got != "call-1" {
		t.Fatalf("CallIDFromCtx=%q", got)
	}
	if got := CallIDFromCtx(context.Background()); got != "" {
		t.Fatalf("empty ctx=%q", got)
	}
}
