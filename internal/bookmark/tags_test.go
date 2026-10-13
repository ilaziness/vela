package bookmark

import (
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got := normalizeTags([]string{"  Prod ", "prod", "DB", "", "a\nb", "x"})
	want := []string{"Prod", "DB", "x"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestNormalizeTagsTruncateRuneAndCount(t *testing.T) {
	long := strings.Repeat("字", maxTagRunes+5)
	got := normalizeTags([]string{long})
	if len(got) != 1 || len([]rune(got[0])) != maxTagRunes {
		t.Fatalf("rune truncate: %q (%d runes)", got, len([]rune(got[0])))
	}
	many := make([]string, maxTagCount+5)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	got = normalizeTags(many)
	if len(got) != maxTagCount {
		t.Fatalf("count=%d want %d", len(got), maxTagCount)
	}
}

func TestEncodeDecodeTags(t *testing.T) {
	if encodeTags(nil) != "" || encodeTags([]string{}) != "" {
		t.Fatal("empty should encode to empty string")
	}
	raw := encodeTags([]string{"prod", "db"})
	got := decodeTags(raw)
	if len(got) != 2 || got[0] != "prod" || got[1] != "db" {
		t.Fatalf("roundtrip %q -> %v", raw, got)
	}
	if len(decodeTags("")) != 0 || len(decodeTags("[]")) != 0 || len(decodeTags("{bad}")) != 0 {
		t.Fatal("empty/invalid decode should be empty slice")
	}
}

func TestNormalizeColor(t *testing.T) {
	if normalizeColor("  RED ") != "red" {
		t.Fatal("case fold")
	}
	if normalizeColor("magenta") != "" || normalizeColor("#ff0000") != "" {
		t.Fatal("unknown should clear")
	}
	if normalizeColor("") != "" {
		t.Fatal("empty stays empty")
	}
}

func TestNormalizeMeta(t *testing.T) {
	b := Bookmark{Tags: []string{" A ", "a"}, Color: "BLUE"}
	normalizeMeta(&b)
	if len(b.Tags) != 1 || b.Tags[0] != "A" || b.Color != "blue" {
		t.Fatalf("got tags=%v color=%q", b.Tags, b.Color)
	}
}

func TestCopyPreservesTagsAndColor(t *testing.T) {
	svc, id := testBookmarkService(t, "", "")
	src, err := svc.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	src.Tags = []string{"prod", "db"}
	src.Color = "red"
	src.Password = PasswordMask
	src.PrivateKeyPassword = PasswordMask
	src.ProxyPassword = PasswordMask
	if _, err := svc.Save(*src); err != nil {
		t.Fatal(err)
	}
	copied, err := svc.Copy(id)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ID == id {
		t.Fatal("copy should get new id")
	}
	got, err := svc.Get(copied.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "prod" || got.Tags[1] != "db" {
		t.Fatalf("tags=%v", got.Tags)
	}
	if got.Color != "red" {
		t.Fatalf("color=%q", got.Color)
	}
}
