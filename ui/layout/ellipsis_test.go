package layout

import "testing"

// fixedMeasure treats every rune as 8px wide, so expectations are exact.
func fixedMeasure(s string) float32 {
	n := 0
	for range s {
		n++
	}
	return float32(n) * 8
}

func TestEllipsize(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		maxWidth float32
		want     string
	}{
		{name: "fits untouched", text: "abc", maxWidth: 24, want: "abc"},
		{name: "exact fit", text: "abcdef", maxWidth: 48, want: "abcdef"},
		{name: "truncates", text: "abcdef", maxWidth: 40, want: "abcd…"},
		{name: "single rune budget", text: "abcdef", maxWidth: 16, want: "a…"},
		{name: "ellipsis only", text: "abcdef", maxWidth: 12, want: "…"},
		{name: "too narrow", text: "abcdef", maxWidth: 4, want: ""},
		{name: "multi byte runes", text: "标签页名称很长", maxWidth: 40, want: "标签页名…"},
		{name: "empty text", text: "", maxWidth: 40, want: ""},
		{name: "zero width", text: "abc", maxWidth: 0, want: "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Ellipsize(tc.text, tc.maxWidth, fixedMeasure)
			if got != tc.want {
				t.Fatalf("Ellipsize(%q, %v) = %q, want %q", tc.text, tc.maxWidth, got, tc.want)
			}
		})
	}
}

func TestEllipsizeNilMeasure(t *testing.T) {
	if got := Ellipsize("abcdef", 10, nil); got != "abcdef" {
		t.Fatalf("nil measure should pass text through, got %q", got)
	}
}

func TestEllipsizeNeverExceedsWidth(t *testing.T) {
	text := "a-very-long-ssh-tab-title@192.168.82.151:6022"
	for w := float32(8); w <= 200; w += 8 {
		got := Ellipsize(text, w, fixedMeasure)
		if got == "" {
			continue
		}
		if fixedMeasure(got) > w {
			t.Fatalf("Ellipsize(%v) = %q (%v px) exceeds budget %v", w, got, fixedMeasure(got), w)
		}
	}
}
