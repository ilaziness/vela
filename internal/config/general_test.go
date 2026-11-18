package config

import (
	"testing"

	"github.com/pelletier/go-toml/v2"
)

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

func TestGeneralConfigAutoCheckUpdateDefaults(t *testing.T) {
	if !Default().General.AutoCheckUpdate {
		t.Fatal("default auto check update = false")
	}

	cfg := Default()
	if err := toml.Unmarshal([]byte("[general]\nfont_size = 16\n"), cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.General.AutoCheckUpdate {
		t.Fatal("missing auto_check_update should default to true")
	}

	cfg = Default()
	if err := toml.Unmarshal([]byte("[general]\nauto_check_update = false\n"), cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.General.AutoCheckUpdate {
		t.Fatal("explicit auto_check_update=false should stay false")
	}
}
