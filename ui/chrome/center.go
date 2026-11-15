// CenterOnScreen moves a frameless window to the center of the
// primary monitor's work area. go-gui has no screen-size API, so each
// platform file implements what it can; unsupported platforms keep
// the position the window manager chose.
//
// Call it from WindowCfg.OnInit: that runs after the first frame,
// when the backend is attached and Position/SetPosition answer.
// Only frameless windows need it — decorated ones get a sensible
// cascade from the OS, while DecorationNone on Win32 is created as
// WS_POPUP and lands at (0,0).
package chrome
