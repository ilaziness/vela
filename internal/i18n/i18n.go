package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

const (
	LangEn   = "en"
	LangZhCN = "zh-CN"
	LangZhTW = "zh-TW"
)

// Supported lists application languages in stable display order.
var Supported = []string{LangEn, LangZhCN, LangZhTW}

var (
	mu       sync.RWMutex
	current  = LangEn
	catalogs map[string]map[string]string
)

func init() {
	catalogs = make(map[string]map[string]string, len(Supported))
	for _, lang := range Supported {
		raw, err := localeFS.ReadFile("locales/" + lang + ".json")
		if err != nil {
			panic("i18n: load locale " + lang + ": " + err.Error())
		}
		var nested map[string]any
		if err := json.Unmarshal(raw, &nested); err != nil {
			panic("i18n: parse locale " + lang + ": " + err.Error())
		}
		flat := make(map[string]string)
		flatten("", nested, flat)
		catalogs[lang] = flat
	}
}

func flatten(prefix string, node map[string]any, out map[string]string) {
	for k, v := range node {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch tv := v.(type) {
		case string:
			out[key] = tv
		case map[string]any:
			flatten(key, tv, out)
		default:
			// ignore non-string leaves
		}
	}
}

// Normalize maps a language tag to a supported app language.
func Normalize(lang string) string {
	s := strings.TrimSpace(lang)
	if s == "" {
		return LangEn
	}
	s = strings.ReplaceAll(s, "_", "-")
	lower := strings.ToLower(s)

	switch {
	case lower == "zh-tw" || lower == "zh-hk" || lower == "zh-mo" ||
		strings.HasPrefix(lower, "zh-hant") || strings.HasPrefix(lower, "zh-tw") ||
		strings.HasPrefix(lower, "zh-hk") || strings.HasPrefix(lower, "zh-mo"):
		return LangZhTW
	case lower == "zh" || lower == "zh-cn" || lower == "zh-sg" ||
		strings.HasPrefix(lower, "zh-hans") || strings.HasPrefix(lower, "zh-cn") ||
		strings.HasPrefix(lower, "zh-sg") || strings.HasPrefix(lower, "zh-"):
		return LangZhCN
	case strings.HasPrefix(lower, "en"):
		return LangEn
	default:
		return LangEn
	}
}

// DetectSystem returns a supported language from the OS UI locale.
// Platform locale is preferred over LANG/LC_ALL so a Chinese Windows install
// is not forced to English by a developer shell env.
func DetectSystem() string {
	if loc := platformLocale(); loc != "" {
		return Normalize(loc)
	}
	for _, key := range []string{"LC_ALL", "LANG"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			v = strings.Split(v, ".")[0]
			return Normalize(v)
		}
	}
	return LangEn
}

// SetLanguage sets the process UI language. Unknown values normalize to en.
func SetLanguage(lang string) {
	mu.Lock()
	defer mu.Unlock()
	current = Normalize(lang)
}

// Language returns the current UI language.
func Language() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Catalog returns a copy of the flattened catalog for lang (normalized).
func Catalog(lang string) map[string]string {
	lang = Normalize(lang)
	src := catalogs[lang]
	if src == nil {
		src = catalogs[LangEn]
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// T returns the translation for key in the current language, falling back to en then the key.
func T(key string) string {
	return TFor(Language(), key)
}

// Tf is T with fmt.Sprintf args.
func Tf(key string, args ...any) string {
	return TfFor(Language(), key, args...)
}

// TFor looks up key in lang, then en, else returns key.
func TFor(lang, key string) string {
	lang = Normalize(lang)
	if cat := catalogs[lang]; cat != nil {
		if v, ok := cat[key]; ok {
			return v
		}
	}
	if lang != LangEn {
		if cat := catalogs[LangEn]; cat != nil {
			if v, ok := cat[key]; ok {
				return v
			}
		}
	}
	return key
}

// TfFor is TFor with fmt.Sprintf.
func TfFor(lang, key string, args ...any) string {
	msg := TFor(lang, key)
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}
