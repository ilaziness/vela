package app

import (
	"strings"

	"github.com/go-gui-org/go-term/term"
)

// defaultTermTheme is the built-in go-term palette, offered as "Default"
// alongside the bundled themes.
func defaultTermTheme() term.NamedTheme {
	return term.NamedTheme{Name: "Default", Theme: term.DefaultTheme}
}

// defaultTermThemes is the selectable catalog: Default first, then every
// bundled go-term theme. The bundled slice is cached inside go-term and must
// not be mutated; we copy it into our own.
func defaultTermThemes() []term.NamedTheme {
	bundled := term.BundledThemes()
	out := make([]term.NamedTheme, 0, len(bundled)+1)
	out = append(out, defaultTermTheme())
	return append(out, bundled...)
}

// termCatalogLocked lazily builds the theme catalog on first use so a
// startup with the default theme never pays for decoding the bundled table.
func (c *Controller) termCatalogLocked() []term.NamedTheme {
	if c.termThemes == nil {
		c.termThemes = defaultTermThemes()
	}
	return c.termThemes
}

// termThemeByNameLocked resolves name against the catalog: "Default" hits
// before any bundled decode, then exact match, then case-insensitive.
func (c *Controller) termThemeByNameLocked(name string) (term.NamedTheme, bool) {
	name = strings.TrimSpace(name)
	def := defaultTermTheme()
	if strings.EqualFold(name, def.Name) {
		return def, true
	}
	if name == "" {
		return term.NamedTheme{}, false
	}
	themes := c.termCatalogLocked()
	for i := range themes {
		if themes[i].Name == name {
			return themes[i], true
		}
	}
	for i := range themes {
		if strings.EqualFold(themes[i].Name, name) {
			return themes[i], true
		}
	}
	return term.NamedTheme{}, false
}

// TermThemeName returns the canonical catalog name for name, or "Default"
// when name is empty or unknown. Safe to call from any thread.
func (c *Controller) TermThemeName(name string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if nt, ok := c.termThemeByNameLocked(name); ok {
		return nt.Name
	}
	return defaultTermTheme().Name
}

// TermThemeItems returns the selectable terminal theme names ("Default"
// first, then the bundled themes). The slice is cached and must not be
// mutated. Safe to call from any thread.
func (c *Controller) TermThemeItems() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.termThemeItems) == 0 {
		themes := c.termCatalogLocked()
		c.termThemeItems = make([]string, len(themes))
		for i := range themes {
			c.termThemeItems[i] = themes[i].Name
		}
	}
	return c.termThemeItems
}

// SetTermTheme switches the terminal color theme by name (case-insensitive;
// unknown or empty falls back to Default). It updates the theme new sessions
// start with and applies it to every live terminal. Call on the UI thread.
func (c *Controller) SetTermTheme(name string) {
	c.mu.Lock()
	nt, ok := c.termThemeByNameLocked(name)
	if !ok {
		nt = defaultTermTheme()
	}
	c.termTheme = nt
	live := make([]*term.Term, 0, len(c.terms))
	for _, tm := range c.terms {
		if tm != nil {
			live = append(live, tm)
		}
	}
	c.mu.Unlock()

	for _, tm := range live {
		tm.SetTheme(nt.Theme)
	}
}
