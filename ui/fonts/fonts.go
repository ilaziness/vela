package fonts

import (
	"os"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/config"
)

// Apply installs the go-gui theme selected by general.theme (dark when unset
// or unknown) with Vela font overrides on top:
//   - UI Family (TextStyleDef): opt-in — only set when general.font is a
//     real family; otherwise keep the theme's platform default
//   - UI Size: from general.font_size
//   - Mono (theme code roles): from terminal.font (platform mono defaults)
//
// Locale is seeded so that CJK glyph fallback prefers the right regional
// face when LANG is unset. Missing glyphs still fall back through go-glyph.
func Apply(cfg *config.Config) {
	if cfg == nil {
		cfg = config.Default()
	}
	ensureCJKLocale(cfg.General.Language)

	uiSize := float32(cfg.General.FontSize)
	if uiSize < 8 || uiSize > 72 {
		uiSize = 14
	}

	tcfg := themeCfg(cfg.General.Theme)
	if fam := configuredUIFamily(cfg.General.Font); fam != "" {
		tcfg.TextStyleDef.Family = fam
	}
	tcfg.TextStyleDef.Size = uiSize
	tcfg.MonoFontFamily = monoFamily(cfg.Terminal.Font)
	// Keep the bundled Feather icon face for TextStyleIcon* roles.
	tcfg.IconFontFamily = gui.IconFontName
	gui.SetTheme(gui.ThemeMaker(tcfg))
}

// CanonicalThemeName returns the registered go-gui theme name matching name
// (exact first, then case-insensitive), falling back to the dark preset for
// empty or unknown values.
func CanonicalThemeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return gui.ThemeDark.Name
	}
	names := gui.ThemeRegisteredNames()
	for _, n := range names {
		if n == name {
			return n
		}
	}
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return n
		}
	}
	return gui.ThemeDark.Name
}

// themeCfg returns the ThemeCfg for the canonical go-gui theme name.
func themeCfg(name string) gui.ThemeCfg {
	if t, ok := gui.ThemeGet(CanonicalThemeName(name)); ok {
		return t.Cfg
	}
	return gui.ThemeDark.Cfg
}

// TermStyle is the go-term face: system monospace first; non-mono glyphs
// (CJK, emoji, …) fall back through go-glyph to the system default fonts.
func TermStyle(cfg config.TerminalConfig) gui.TextStyle {
	size := float32(cfg.FontSize)
	if size < 1 || size > 72 {
		size = 14
	}
	return gui.TextStyle{
		Family: monoFamily(cfg.Font),
		Size:   size,
	}
}

// configuredUIFamily returns the first usable family from a CSS-like stack,
// or "" when none is configured (Apply must not override ThemeDark Family).
func configuredUIFamily(configured string) string {
	for _, name := range splitFamilies(configured) {
		if name == "" || isPseudoFamily(name) {
			continue
		}
		return name
	}
	return ""
}

// monoFamily: empty config → system mono; otherwise first name in the stack,
// then platform mono names, then generic "monospace".
func monoFamily(configured string) string {
	return pickFamily(configured, defaultMonoFamilies())
}

func ensureCJKLocale(lang string) {
	if os.Getenv("LC_ALL") != "" || os.Getenv("LC_CTYPE") != "" || os.Getenv("LANG") != "" {
		return
	}
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh-cn", "zh_cn", "zh-hans":
		_ = os.Setenv("LANG", "zh_CN.UTF-8")
	case "zh-tw", "zh_tw", "zh-hant":
		_ = os.Setenv("LANG", "zh_TW.UTF-8")
	case "ja", "ja-jp":
		_ = os.Setenv("LANG", "ja_JP.UTF-8")
	case "ko", "ko-kr":
		_ = os.Setenv("LANG", "ko_KR.UTF-8")
	}
}

// pickFamily returns the first usable family from a CSS-like stack, else the
// first default. go-gui takes a single Family; commas are config-only.
func pickFamily(configured string, defaults []string) string {
	if name := configuredUIFamily(configured); name != "" {
		return name
	}
	for _, name := range defaults {
		if name != "" {
			return name
		}
	}
	return "sans-serif"
}

func splitFamilies(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isPseudoFamily skips CSS-only / web-only tokens that go-glyph does not map.
// Real generics "sans-serif", "serif", "monospace" are kept.
func isPseudoFamily(name string) bool {
	switch strings.ToLower(name) {
	case "ui-monospace", "ui-sans-serif", "system-ui", "emoji", "fangsong":
		return true
	default:
		return false
	}
}
