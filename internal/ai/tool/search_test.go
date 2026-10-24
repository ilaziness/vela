package tool

import "testing"

func TestYuanbaoCnt(t *testing.T) {
	if got := yuanbaoCnt(8); got != 10 {
		t.Fatalf("cnt 8 -> %d, want 10", got)
	}
	if got := yuanbaoCnt(20); got != 20 {
		t.Fatalf("cnt 20 -> %d, want 20", got)
	}
}
