package tunnel

import (
	"testing"

	"github.com/ilaziness/vela/internal/usererr"
)

func TestValidateListenAddr(t *testing.T) {
	valid := []string{
		"127.0.0.1:8080",
		"0.0.0.0:1080",
		"[::1]:1080",
	}
	for _, addr := range valid {
		if err := ValidateListenAddr(addr); err != nil {
			t.Errorf("ValidateListenAddr(%q) = %v, want nil", addr, err)
		}
	}

	invalid := []string{
		"",
		"127.0.0.1",
		"127.0.0.1:",
		"localhost:8080",
		":8080",
		"127.0.0.1:0",
		"127.0.0.1:65536",
		"127.0.0.1:abc",
		"999.1.1.1:80",
	}
	for _, addr := range invalid {
		if err := ValidateListenAddr(addr); err == nil {
			t.Errorf("ValidateListenAddr(%q) = nil, want error", addr)
		}
	}
}

func TestValidateTargetAddr(t *testing.T) {
	valid := []string{
		"127.0.0.1:22",
		"localhost:3306",
		"db.internal:5432",
		"[::1]:80",
	}
	for _, addr := range valid {
		if err := ValidateTargetAddr(addr); err != nil {
			t.Errorf("ValidateTargetAddr(%q) = %v, want nil", addr, err)
		}
	}

	invalid := []string{
		"",
		"example.com",
		"example.com:",
		":3306",
		"host name:80",
		"example.com:0",
		"example.com:70000",
	}
	for _, addr := range invalid {
		if err := ValidateTargetAddr(addr); err == nil {
			t.Errorf("ValidateTargetAddr(%q) = nil, want error", addr)
		}
	}
}

func TestValidatePort(t *testing.T) {
	for _, p := range []int{1, 22, 8080, 65535} {
		if err := ValidatePort(p); err != nil {
			t.Errorf("ValidatePort(%d) = %v, want nil", p, err)
		}
	}
	for _, p := range []int{0, -1, 65536} {
		if err := ValidatePort(p); err == nil {
			t.Errorf("ValidatePort(%d) = nil, want error", p)
		}
	}
}

func TestValidationErrorsAreCoded(t *testing.T) {
	err := ValidateListenAddr("localhost:8080")
	ue, ok := usererr.As(err)
	if !ok || ue.Code == "" {
		t.Fatalf("ValidateListenAddr error is not coded: %v", err)
	}
}
