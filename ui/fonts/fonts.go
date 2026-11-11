package fonts

import (
	"os"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/config"
)

// Apply configures ThemeDark text sizes and font families:
//   - UI (TextStyleDef): system default sans (generic "sans-serif")
//   - Mono (theme code roles): system default monospace ("monospace")
//
// go-glyph maps those generics to the platform face (Segoe UI / Consolas on
// Windows, etc.) and falls missing glyphs (CJK) back to system script fonts
// such as Microsoft YaHei. Locale is seeded so that CJK tier prefers the
// right regional face when LANG is unset.
func Apply(cfg *config.Config) {
	if cfg == nil {
		cfg = config.Default()
	}
	ensureCJKLocale(cfg.General.Language)

	uiSize := float32(cfg.General.FontSize)
	if uiSize < 8 || uiSize > 72 {
		uiSize = 14
	}

	tcfg := gui.ThemeDark.Cfg
	tcfg.TextStyleDef.Family = uiFamily(cfg.General.Font)
	tcfg.TextStyleDef.Size = uiSize
	tcfg.MonoFontFamily = monoFamily(cfg.Terminal.Font)
	// Keep the bundled Feather icon face for TextStyleIcon* roles.
	tcfg.IconFontFamily = gui.IconFontName
	gui.SetTheme(gui.ThemeMaker(tcfg))
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

// uiFamily: empty config → system sans; otherwise first name in the stack,
// falling back to "sans-serif".
func uiFamily(configured string) string {
	return pickFamily(configured, []string{"sans-serif"})
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
	for _, name := range splitFamilies(configured) {
		if name == "" || isPseudoFamily(name) {
			continue
		}
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
