package mcpserver

import (
	"context"
	"testing"

	"github.com/ilaziness/vela/internal/ai"
)

func TestStartDisabled(t *testing.T) {
	s := New(Deps{})
	if err := s.Start(context.Background(), ai.MCPServerConfig{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if st.Running {
		t.Fatal("disabled server must not run")
	}
	// Idempotent: stopping again is fine.
	s.Stop()
}

func TestMCPServerConfigNormalize(t *testing.T) {
	var c ai.MCPServerConfig
	c.Normalize()
	if c.Port != ai.DefaultMCPServerPort {
		t.Fatalf("default port: %d", c.Port)
	}
	c.Port = 70000
	c.Normalize()
	if c.Port != 65535 {
		t.Fatalf("clamped port: %d", c.Port)
	}
}
