package ssh

import "testing"

func TestAliveCountMax(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, "", nil, nil)
	if got := m.aliveCountMax(); got != defaultServerAliveCountMax {
		t.Fatalf("default=%d want %d", got, defaultServerAliveCountMax)
	}

	m.SetOptions(Options{ServerAliveCountMax: 5})
	if got := m.aliveCountMax(); got != 5 {
		t.Fatalf("countMax=%d want 5", got)
	}

	m.SetOptions(Options{ServerAliveCountMax: -1})
	if got := m.aliveCountMax(); got != defaultServerAliveCountMax {
		t.Fatalf("negative=%d want %d", got, defaultServerAliveCountMax)
	}
}
