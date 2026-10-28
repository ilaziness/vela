//go:build windows

package i18n

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultUILanguage = modKernel32.NewProc("GetUserDefaultUILanguage")
	procLCIDToLocaleName         = modKernel32.NewProc("LCIDToLocaleName")
	procGetUserDefaultLocaleName = modKernel32.NewProc("GetUserDefaultLocaleName")
)

const localeNameMax = 85 // LOCALE_NAME_MAX_LENGTH

func platformLocale() string {
	if name := localeNameFromUILanguage(); name != "" {
		return name
	}
	return localeNameDefault()
}

func localeNameFromUILanguage() string {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	if r == 0 {
		return ""
	}
	buf := make([]uint16, localeNameMax)
	n, _, _ := procLCIDToLocaleName.Call(
		r, // LANGID is a valid LCID with SORT_DEFAULT
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(localeNameMax),
		0,
	)
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func localeNameDefault() string {
	buf := make([]uint16, localeNameMax)
	r, _, _ := procGetUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(localeNameMax),
	)
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
