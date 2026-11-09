package serial

import "testing"

func TestValidateRejectsSoftwareFlow(t *testing.T) {
	cfg := OpenConfig{Device: "COM1", BaudRate: 9600, FlowControl: "software"}.normalized()
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRejectsUnknownFlow(t *testing.T) {
	cfg := OpenConfig{Device: "COM1", BaudRate: 9600, FlowControl: "bogus"}.normalized()
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error")
	}
}
