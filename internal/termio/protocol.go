package termio

// Protocol is an explicit terminal session protocol. Empty/unknown is invalid.
type Protocol string

const (
	ProtocolSSH    Protocol = "ssh"
	ProtocolSerial Protocol = "serial"
	// ProtocolReplay marks a local .gtr playback pane. It is not dialable
	// and must stay out of ParseProtocol.
	ProtocolReplay Protocol = "replay"
)

// ParseProtocol returns a known protocol or false.
func ParseProtocol(s string) (Protocol, bool) {
	switch Protocol(s) {
	case ProtocolSSH, ProtocolSerial:
		return Protocol(s), true
	default:
		return "", false
	}
}
