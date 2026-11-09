package serial

import (
	"strconv"
	"strings"

	goserial "go.bug.st/serial"

	"github.com/ilaziness/vela/internal/usererr"
)

// OpenConfig is the explicit serial connect/test payload.
type OpenConfig struct {
	Device      string
	BaudRate    int
	DataBits    int
	Parity      string // none | even | odd | mark | space
	StopBits    int    // 1 or 2 (15 = 1.5 stored as special; we use 1/2)
	FlowControl string // none | hardware
	Charset     string
}

func (c OpenConfig) validate() error {
	if strings.TrimSpace(c.Device) == "" {
		return usererr.Coded("serial.device_required", "Serial device is required")
	}
	if c.BaudRate <= 0 {
		return usererr.Coded("serial.baud_required", "Baud rate is required")
	}
	bits := c.DataBits
	if bits == 0 {
		bits = 8
	}
	if bits < 5 || bits > 8 {
		return usererr.Coded("serial.data_bits_invalid", "Data bits must be 5–8")
	}
	stop := c.StopBits
	if stop == 0 {
		stop = 1
	}
	if stop != 1 && stop != 2 {
		return usererr.Coded("serial.stop_bits_invalid", "Stop bits must be 1 or 2")
	}
	switch strings.ToLower(strings.TrimSpace(c.Parity)) {
	case "", "none", "even", "odd", "mark", "space":
	default:
		return usererr.Coded("serial.parity_invalid", "Invalid parity: %s", c.Parity)
	}
	switch strings.ToLower(strings.TrimSpace(c.FlowControl)) {
	case "", "none", "hardware":
	default:
		return usererr.Coded("serial.flow_invalid", "Invalid flow control: %s", c.FlowControl)
	}
	return nil
}

func (c OpenConfig) normalized() OpenConfig {
	out := c
	out.Device = strings.TrimSpace(c.Device)
	if out.DataBits == 0 {
		out.DataBits = 8
	}
	if out.StopBits == 0 {
		out.StopBits = 1
	}
	parity := strings.ToLower(strings.TrimSpace(c.Parity))
	if parity == "" {
		parity = "none"
	}
	out.Parity = parity
	flow := strings.ToLower(strings.TrimSpace(c.FlowControl))
	if flow == "" {
		flow = "none"
	}
	out.FlowControl = flow
	return out
}

func (c OpenConfig) mode() (*goserial.Mode, error) {
	c = c.normalized()
	if err := c.validate(); err != nil {
		return nil, err
	}
	mode := &goserial.Mode{
		BaudRate: c.BaudRate,
		DataBits: c.DataBits,
	}
	switch c.Parity {
	case "even":
		mode.Parity = goserial.EvenParity
	case "odd":
		mode.Parity = goserial.OddParity
	case "mark":
		mode.Parity = goserial.MarkParity
	case "space":
		mode.Parity = goserial.SpaceParity
	default:
		mode.Parity = goserial.NoParity
	}
	if c.StopBits == 2 {
		mode.StopBits = goserial.TwoStopBits
	} else {
		mode.StopBits = goserial.OneStopBit
	}
	return mode, nil
}

func (c OpenConfig) label() string {
	c = c.normalized()
	if c.BaudRate > 0 {
		return c.Device + " @ " + strconv.Itoa(c.BaudRate)
	}
	return c.Device
}
