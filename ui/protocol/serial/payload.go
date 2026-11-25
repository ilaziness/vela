package serialui

// Payload is the serial connect form state stored in protocol.Draft.Payload.
// Advanced line settings (parity/stop/flow/charset) stay for bookmarks.
type Payload struct {
	Device   string
	BaudRate int
}

// DefaultPayload returns a blank serial form with a common baud rate.
func DefaultPayload() Payload {
	return Payload{BaudRate: 115200}
}
