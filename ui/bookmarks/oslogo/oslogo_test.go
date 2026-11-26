package oslogo

import (
	"strings"
	"testing"
)

func TestCatalogIntegrity(t *testing.T) {
	if len(Catalog) < 20 {
		t.Fatalf("catalog too small: %d", len(Catalog))
	}
	seen := map[string]bool{}
	for _, e := range Catalog {
		if e.Token == "" || !strings.HasPrefix(e.Token, "os:") {
			t.Fatalf("bad token %q", e.Token)
		}
		if seen[e.Token] {
			t.Fatalf("duplicate token %q", e.Token)
		}
		seen[e.Token] = true
		if e.Name == "" || e.SVG == "" {
			t.Fatalf("entry %q missing name/svg", e.Token)
		}
	}
}

func TestSVGsAreXML(t *testing.T) {
	svgs := []string{DefaultSVG}
	for _, e := range Catalog {
		svgs = append(svgs, e.SVG)
	}
	for _, svg := range svgs {
		s := strings.TrimSpace(svg)
		if !strings.HasPrefix(s, "<") || !strings.Contains(s, "<svg") {
			t.Fatalf("not an svg document: %.80s", s)
		}
	}
}
