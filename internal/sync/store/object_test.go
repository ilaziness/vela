package store

import (
	"reflect"
	"testing"
	"time"
)

func TestSanitizeName(t *testing.T) {
	got, err := SanitizeName(`dir\vela-backup-1.zip`)
	if err != nil || got != "vela-backup-1.zip" {
		t.Fatalf("SanitizeName=%q %v", got, err)
	}
	if _, err := SanitizeName("v1.dat"); err == nil {
		t.Fatal("expected reject non-zip")
	}
	if _, err := SanitizeName(".."); err == nil {
		t.Fatal("expected reject ..")
	}
}

func TestNameFromKey(t *testing.T) {
	got, err := NameFromKey("vela/foo.zip", "vela/")
	if err != nil || got != "foo.zip" {
		t.Fatalf("NameFromKey=%q %v", got, err)
	}
	if _, err := NameFromKey("vela/sub/foo.zip", "vela/"); err == nil {
		t.Fatal("expected reject nested key")
	}
}

func TestNormalizePrefixAndRemotePath(t *testing.T) {
	if got := NormalizePrefix("vela"); got != "vela/" {
		t.Fatalf("NormalizePrefix=%q", got)
	}
	if got := NormalizePrefix("/vela/"); got != "vela/" {
		t.Fatalf("NormalizePrefix trimmed=%q", got)
	}
	if got := NormalizePrefix(""); got != "" {
		t.Fatalf("NormalizePrefix empty=%q", got)
	}
	if got := NormalizeRemotePath("Vela/backups"); got != "/Vela/backups" {
		t.Fatalf("NormalizeRemotePath=%q", got)
	}
	if got := NormalizeRemotePath("/Vela/"); got != "/Vela" {
		t.Fatalf("NormalizeRemotePath trim=%q", got)
	}
}

func TestPaginateAndSortBackups(t *testing.T) {
	t1 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	backups := []BackupInfo{
		{Name: "a.zip", CreatedAt: t1},
		{Name: "c.zip", CreatedAt: t2},
		{Name: "b.zip", CreatedAt: t2},
	}
	SortBackupsDesc(backups)
	want := []string{"c.zip", "b.zip", "a.zip"}
	var got []string
	for _, b := range backups {
		got = append(got, b.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sort=%v want %v", got, want)
	}
	page := PaginateBackups(backups, 2, 1)
	if !reflect.DeepEqual([]string{page[0].Name, page[1].Name}, []string{"b.zip", "a.zip"}) {
		t.Fatalf("page=%v", page)
	}
}
