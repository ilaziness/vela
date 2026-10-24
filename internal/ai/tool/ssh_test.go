package tool

import (
	"strings"
	"testing"
)

func TestTruncateEcho(t *testing.T) {
	if got := truncateEcho("abc", 10); got != "abc" {
		t.Fatalf("short=%q", got)
	}
	got := truncateEcho("你好世界", 4)
	if !strings.HasSuffix(got, "…[truncated]") {
		t.Fatalf("got %q", got)
	}
}

func TestToCRLF(t *testing.T) {
	if got := toCRLF("a\nb\r\nc\rd"); got != "a\r\nb\r\nc\r\nd" {
		t.Fatalf("got %q", got)
	}
}
