package i18n

import (
	"strings"
	"testing"
)

func TestLocaleKeyParity(t *testing.T) {
	en := catalogs[LangEn]
	if len(en) == 0 {
		t.Fatal("en catalog empty")
	}
	for _, lang := range Supported {
		if lang == LangEn {
			continue
		}
		other := catalogs[lang]
		for k := range en {
			if _, ok := other[k]; !ok {
				t.Errorf("%s missing key %q", lang, k)
			}
		}
		for k := range other {
			if _, ok := en[k]; !ok {
				t.Errorf("%s has extra key %q not in en", lang, k)
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":          LangEn,
		"en":        LangEn,
		"en-US":     LangEn,
		"zh":        LangZhCN,
		"zh-CN":     LangZhCN,
		"zh_CN":     LangZhCN,
		"zh-Hans":   LangZhCN,
		"zh-TW":     LangZhTW,
		"zh_TW":     LangZhTW,
		"zh-Hant":   LangZhTW,
		"zh-HK":     LangZhTW,
		"fr-FR":     LangEn,
		"  zh-cn  ": LangZhCN,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTFallback(t *testing.T) {
	SetLanguage(LangZhCN)
	if got := T("window.settings"); got != "设置" {
		t.Fatalf("T window.settings = %q", got)
	}
	if got := T("missing.key"); got != "missing.key" {
		t.Fatalf("missing key = %q", got)
	}
	SetLanguage(LangEn)
	if got := Tf("ssh.auth_failed"); got == "" || got == "ssh.auth_failed" {
		t.Fatalf("Tf ssh.auth_failed = %q", got)
	}
}

func TestCatalogCopy(t *testing.T) {
	c := Catalog(LangEn)
	c["window.settings"] = "mutated"
	if Catalog(LangEn)["window.settings"] == "mutated" {
		t.Fatal("Catalog should return a copy")
	}
}

func TestDetectSystemUsesPlatformOrEnv(t *testing.T) {
	got := DetectSystem()
	switch got {
	case LangEn, LangZhCN, LangZhTW:
	default:
		t.Fatalf("DetectSystem() = %q", got)
	}
}

func TestSettingsNavKeysPresent(t *testing.T) {
	keys := []string{
		"settings.nav.general",
		"settings.nav.terminal",
		"settings.general.title",
		"settings.general.font_placeholder",
	}
	for _, lang := range Supported {
		for _, key := range keys {
			got := TFor(lang, key)
			if got == "" || got == key {
				t.Errorf("%s missing %s", lang, key)
			}
		}
	}
	if got := TFor(LangZhCN, "settings.nav.general"); got != "通用" {
		t.Fatalf("zh-CN settings.nav.general = %q", got)
	}
}

func TestTitleKeysAreNotKeySuffixPlaceholders(t *testing.T) {
	if got := TFor(LangZhCN, "sftp.confirm_delete_title"); got != "确认删除" {
		t.Fatalf("zh-CN sftp.confirm_delete_title = %q", got)
	}
	for _, lang := range Supported {
		for key, val := range catalogs[lang] {
			if !strings.HasSuffix(key, "title") {
				continue
			}
			if strings.HasSuffix(val, "标题") || strings.HasSuffix(val, "標題") || strings.HasSuffix(val, " Title") {
				t.Errorf("%s %s = %q (key suffix leaked into copy)", lang, key, val)
			}
			if val == "Title" || val == "标题" || val == "標題" {
				t.Errorf("%s %s is a placeholder %q", lang, key, val)
			}
		}
	}
}

func TestThemeKeysPresent(t *testing.T) {
	for _, lang := range Supported {
		for _, key := range []string{"theme.light", "theme.dark", "theme.eyeCare", "theme.switch"} {
			got := TFor(lang, key)
			if got == "" || got == key {
				t.Errorf("%s missing %s", lang, key)
			}
		}
	}
	if got := TFor(LangEn, "command.input_helper"); got == "Input Helper" {
		t.Fatal("command.input_helper still a key-suffix placeholder")
	}
}

func TestFormatVerbsForKnownSprintfKeys(t *testing.T) {
	keys := []string{
		"command.confirm_delete_message",
		"hostkey.old_fingerprint",
		"tools.regex.match_n",
	}
	for _, lang := range Supported {
		for _, key := range keys {
			got := TFor(lang, key)
			if !strings.Contains(got, "%") {
				t.Errorf("%s %s missing format verb: %q", lang, key, got)
			}
		}
	}
}
