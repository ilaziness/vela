package config

import "testing"

func TestGeneralConfigNormalize(t *testing.T) {
	cfg := GeneralConfig{Font: "  Fira Code; } body { color: red  ", FontSize: 0}
	cfg.Normalize()
	if cfg.Font != "Fira Code  body  color: red" {
		t.Fatalf("font = %q", cfg.Font)
	}
	if cfg.FontSize != 14 {
		t.Fatalf("font size = %d", cfg.FontSize)
	}

	cfg = GeneralConfig{Font: "\"Segoe UI\"", FontSize: 80}
	cfg.Normalize()
	if cfg.Font != "Segoe UI" {
		t.Fatalf("font = %q", cfg.Font)
	}
	if cfg.FontSize != 14 {
		t.Fatalf("font size = %d", cfg.FontSize)
	}

	cfg = GeneralConfig{Font: "  ", FontSize: 16}
	cfg.Normalize()
	if cfg.Font != "" || cfg.FontSize != 16 {
		t.Fatalf("cfg = %+v", cfg)
	}
}
