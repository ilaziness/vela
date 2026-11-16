package config

import "testing"

func TestSSHConfigNormalize(t *testing.T) {
	t.Parallel()

	def := Default().SSH

	c := SSHConfig{}
	c.Normalize()
	if c.ServerAliveCountMax != def.ServerAliveCountMax {
		t.Fatalf("countMax zero=%d want %d", c.ServerAliveCountMax, def.ServerAliveCountMax)
	}
	if c.DialTimeoutSec != def.DialTimeoutSec {
		t.Fatalf("dialTimeout zero=%d want %d", c.DialTimeoutSec, def.DialTimeoutSec)
	}

	c = SSHConfig{ServerAliveCountMax: -2, DialTimeoutSec: 5, ServerAliveInterval: 10}
	c.Normalize()
	if c.ServerAliveCountMax != def.ServerAliveCountMax {
		t.Fatalf("countMax negative=%d want %d", c.ServerAliveCountMax, def.ServerAliveCountMax)
	}
	if c.DialTimeoutSec != 5 {
		t.Fatalf("dialTimeout=%d", c.DialTimeoutSec)
	}

	c = SSHConfig{ServerAliveCountMax: 7, DialTimeoutSec: 15, ServerAliveInterval: 20}
	c.Normalize()
	if c.ServerAliveCountMax != 7 {
		t.Fatalf("countMax=%d want 7", c.ServerAliveCountMax)
	}
}
