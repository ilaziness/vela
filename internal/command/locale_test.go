package command

import (
	"testing"
	"testing/fstest"

	"github.com/ilaziness/vela/internal/i18n"
)

func TestLoaderFromFSAndReload(t *testing.T) {
	en := `[{"category":"Sys","commands":[{"name":"Load","command":"uptime","description":"en"}]}]`
	zh := `[{"category":"系统","commands":[{"name":"负载","command":"uptime","description":"zh"}]}]`
	fsys := fstest.MapFS{
		"data/commands/en.json":    &fstest.MapFile{Data: []byte(en)},
		"data/commands/zh-CN.json": &fstest.MapFile{Data: []byte(zh)},
		"data/commands/zh-TW.json": &fstest.MapFile{Data: []byte(zh)},
	}
	loader := LoaderFromFS(fsys, "data/commands")
	s := New(nil, nil, loader, nil)

	i18n.SetLanguage(i18n.LangEn)
	if err := s.ReloadBuiltin(i18n.LangEn); err != nil {
		t.Fatal(err)
	}
	b := s.Builtin()
	if _, ok := b["Sys"]; !ok {
		t.Fatalf("en builtin missing Sys: %#v", b)
	}
	if got := b["Sys"][0].Command; got != "uptime" {
		t.Fatalf("command = %q", got)
	}

	if err := s.ReloadBuiltin(i18n.LangZhCN); err != nil {
		t.Fatal(err)
	}
	b = s.Builtin()
	if _, ok := b["系统"]; !ok {
		t.Fatalf("zh builtin missing 系统: %#v", b)
	}
	if got := b["系统"][0].Command; got != "uptime" {
		t.Fatalf("command must be identical across locales, got %q", got)
	}
	if got := b["系统"][0].Name; got != "负载" {
		t.Fatalf("name = %q", got)
	}
}
