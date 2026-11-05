package services

import (
	"strings"
	"testing"
)

func TestOverwriteConfirmMessage(t *testing.T) {
	one := overwriteConfirmMessage([]string{"a.txt"})
	if !strings.Contains(one, "a.txt") {
		t.Fatalf("expected name in one-item message, got %q", one)
	}
	many := overwriteConfirmMessage([]string{"a", "b", "c"})
	if !strings.Contains(many, "3") || !strings.Contains(many, "a, b, c") {
		t.Fatalf("expected count and names in many-item message, got %q", many)
	}
	long := overwriteConfirmMessage([]string{"1", "2", "3", "4", "5", "6", "7"})
	if !strings.Contains(long, "…") || strings.Contains(long, "6") {
		t.Fatalf("expected truncated many-item message, got %q", long)
	}
}
