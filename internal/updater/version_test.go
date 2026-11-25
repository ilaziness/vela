package updater

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "1.2.3", 0},
		{"v1.10.0", "v1.9.9", 1},
		{"1.0.0", "v1.0.1", -1},
		{"v2", "v2.0.0", 0},
		{"v1.2", "v1.2.1", -1},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "aaa  Vela_v1.2.3_windows_amd64.zip\nbbb *Vela_v1.2.3_linux_amd64.tar.gz\n"
	if got := ChecksumFor(sums, "Vela_v1.2.3_linux_amd64.tar.gz"); got != "bbb" {
		t.Fatalf("got %q", got)
	}
	if got := ChecksumFor(sums, "missing"); got != "" {
		t.Fatalf("got %q", got)
	}
}
