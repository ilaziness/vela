package toolsui

import "github.com/go-gui-org/go-gui/gui"

// iconGlyph maps legacy MUI icon names from internal/tools to go-gui glyphs.
func iconGlyph(name string) string {
	switch name {
	case "DataObject":
		return gui.IconCode
	case "AccessTime":
		return gui.IconClock
	case "Code", "RegularExpression":
		return gui.IconCode
	case "Fingerprint":
		return gui.IconHash
	case "Schedule":
		return gui.IconCalendar
	case "Lan", "NetworkCheck":
		return gui.IconGlobe
	case "Key":
		return gui.IconKey
	case "Casino":
		return gui.IconRandom
	case "Pin":
		return gui.IconTag
	case "Compare":
		return gui.IconColumns
	case "Lock":
		return gui.IconLock
	default:
		return gui.IconWrench
	}
}
