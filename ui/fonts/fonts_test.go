package fonts

import "testing"

func TestSplitFamilies(t *testing.T) {
	got := splitFamilies(`'Cascadia Mono',"Consolas", Microsoft YaHei, monospace`)
	want := []string{"Cascadia Mono", "Consolas", "Microsoft YaHei", "monospace"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestUIFamilyEmptyUsesSansSerif(t *testing.T) {
	if got := uiFamily(""); got != "sans-serif" {
		t.Fatalf("got %q want sans-serif", got)
	}
}

func TestMonoFamilyEmptyUsesMonospace(t *testing.T) {
	if got := monoFamily(""); got != "monospace" {
		t.Fatalf("got %q want monospace", got)
	}
}

func TestMonoFamilySkipsPseudo(t *testing.T) {
	got := monoFamily("ui-monospace, Consolas")
	if got != "Consolas" {
		t.Fatalf("got %q want Consolas", got)
	}
}

func TestUIFamilyHonorsConfig(t *testing.T) {
	if got := uiFamily("Segoe UI, sans-serif"); got != "Segoe UI" {
		t.Fatalf("got %q", got)
	}
}
