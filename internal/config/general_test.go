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

	cfg = GeneralConfig{}
	cfg.Normalize()
	if cfg.Theme != defaultGeneralTheme {
		t.Fatalf("empty theme = %q", cfg.Theme)
	}

	cfg = GeneralConfig{Theme: "  Light  "}
	cfg.Normalize()
	if cfg.Theme != "Light" {
		t.Fatalf("theme = %q", cfg.Theme)
	}

	if Default().General.Theme != defaultGeneralTheme {
		t.Fatalf("default theme = %q", Default().General.Theme)
	}
}
