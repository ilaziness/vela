package app

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
)

// ApplyGUILocale maps internal/i18n onto go-gui's active Locale so
// Dialog Str* follow BootstrapLanguage / settings language saves.
func ApplyGUILocale() {
	switch i18n.Language() {
	case i18n.LangZhCN:
		gui.SetLocale(gui.LocaleZhCN)
	case i18n.LangZhTW:
		gui.SetLocale(localeZhTW())
	default:
		gui.SetLocale(gui.LocaleEnUS)
	}
}

// localeZhTW is zh-CN with Traditional dialog button labels.
// go-gui ships no zh-TW preset; Keyboard Dialog reads these Str*.
func localeZhTW() gui.Locale {
	lc := gui.LocaleZhCN
	lc.ID = "zh-TW"
	lc.StrOK = "確定"
	lc.StrYes = "是"
	lc.StrNo = "否"
	lc.StrCancel = "取消"
	return lc
}
