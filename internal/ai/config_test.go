package ai

import "testing"

func TestEffectiveMaxAgentTurns(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, DefaultMaxAgentTurns},
		{-1, DefaultMaxAgentTurns},
		{30, DefaultMaxAgentTurns},
		{DefaultMaxAgentTurns, DefaultMaxAgentTurns},
		{DefaultMaxAgentTurns + 10, DefaultMaxAgentTurns + 10},
	}
	for _, tc := range cases {
		cfg := &Config{MaxAgentTurns: tc.in}
		if got := cfg.EffectiveMaxAgentTurns(); got != tc.want {
			t.Fatalf("MaxAgentTurns=%d: got %d, want %d", tc.in, got, tc.want)
		}
	}
	if got := (*Config)(nil).EffectiveMaxAgentTurns(); got != DefaultMaxAgentTurns {
		t.Fatalf("nil config: got %d, want %d", got, DefaultMaxAgentTurns)
	}
}
