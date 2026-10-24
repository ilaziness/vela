package zmodem

// Hex header autostart: ZPAD ZPAD ZDLE ZHEX ("**\x18B")
var hexAutostartPrefix = []byte{0x2a, 0x2a, 0x18, 0x42}

const (
	autostartHeaderLen = 6 // "**\x18B" + 2 type hex digits
	lookbehindMax      = 5 // headerLen - 1
)

// Direction relative to this client.
type Direction int

const (
	DirectionReceive Direction = iota // remote sz → we download
	DirectionSend                     // remote rz → we upload
)

type detectResult struct {
	index int
	dir   Direction
}

func findHexAutostart(data []byte) (detectResult, bool) {
	for i := 0; i+autostartHeaderLen <= len(data); i++ {
		if data[i] != hexAutostartPrefix[0] ||
			data[i+1] != hexAutostartPrefix[1] ||
			data[i+2] != hexAutostartPrefix[2] ||
			data[i+3] != hexAutostartPrefix[3] {
			continue
		}
		ft, ok := parseHexType(data[i+4], data[i+5])
		if !ok {
			continue
		}
		switch ft {
		case 0x00: // ZRQINIT
			return detectResult{index: i, dir: DirectionReceive}, true
		case 0x01: // ZRINIT
			return detectResult{index: i, dir: DirectionSend}, true
		}
	}
	return detectResult{}, false
}

func parseHexType(hiDigit, loDigit byte) (byte, bool) {
	hi, ok1 := fromHexDigit(hiDigit)
	lo, ok2 := fromHexDigit(loDigit)
	if !ok1 || !ok2 {
		return 0, false
	}
	return hi<<4 | lo, true
}

func fromHexDigit(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

// trailingPartialAutostart returns the longest suffix that is a proper prefix
// of a hex autostart header (1..5 bytes). Unlike a raw "last N bytes" window,
// this keeps "**\x18B0" intact when preceded by other data (e.g. "x**\x18B0").
func trailingPartialAutostart(data []byte) []byte {
	n := lookbehindMax
	if n > len(data) {
		n = len(data)
	}
	for ; n > 0; n-- {
		suf := data[len(data)-n:]
		if isAutostartPrefix(suf) {
			out := make([]byte, n)
			copy(out, suf)
			return out
		}
	}
	return nil
}

func isAutostartPrefix(suf []byte) bool {
	if len(suf) == 0 || len(suf) > lookbehindMax {
		return false
	}
	for i := 0; i < len(suf) && i < len(hexAutostartPrefix); i++ {
		if suf[i] != hexAutostartPrefix[i] {
			return false
		}
	}
	if len(suf) <= len(hexAutostartPrefix) {
		return true
	}
	_, ok := fromHexDigit(suf[4])
	return ok
}
