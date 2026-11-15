//go:build windows

package chrome

import (
	"syscall"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	pSystemParamW    = user32.NewProc("SystemParametersInfoW")
	pGetDpiForSystem = user32.NewProc("GetDpiForSystem")
)

// spiGetWorkArea is SPI_GETWORKAREA: the primary monitor's work area
// (screen minus taskbar), in physical desktop pixels for a
// DPI-aware process.
const spiGetWorkArea = 0x0030

type winRect struct {
	left, top, right, bottom int32
}

// CenterOnScreen centers w on the primary monitor's work area.
// Position/SetPosition speak physical pixels of the client area, and
// a frameless window always spawns at the virtual origin — the
// primary monitor — so the primary work area is the right target
// even on multi-monitor setups.
func CenterOnScreen(w *gui.Window) {
	if w == nil || w.IsMaximized() || w.IsFullscreen() {
		return
	}
	if _, _, ok := w.Position(); !ok {
		return
	}
	var work winRect
	// SystemParametersInfoW(action, uiParam, pvParam, fWinIni)
	r, _, _ := pSystemParamW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&work)), 0)
	if r == 0 {
		return
	}
	dpi, _, _ := pGetDpiForSystem.Call()
	if dpi == 0 {
		return
	}
	scale := float32(dpi) / 96.0
	lw, lh := w.WindowSize()
	pw := int(float32(lw) * scale)
	ph := int(float32(lh) * scale)
	x := int(work.left) + (int(work.right)-int(work.left)-pw)/2
	y := int(work.top) + (int(work.bottom)-int(work.top)-ph)/2
	w.SetPosition(x, y)
}
