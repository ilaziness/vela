package bookmarks

import (
	"sort"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/i18n"
)

// Legacy named colors → hex for display / ColorPicker.
var legacyColorHex = map[string]string{
	"red": "#e53935", "orange": "#fb8c00", "yellow": "#fdd835", "green": "#43a047",
	"cyan": "#00acc1", "blue": "#1e88e5", "purple": "#8e24aa", "pink": "#d81b60",
}

// iconPresets use Feather glyphs (gui.Icon*) so Tree TextStyleIcon aligns with labels.
var iconPresets = []struct {
	Token string
	Glyph string
}{
	{Token: "", Glyph: gui.IconBookmark},
	{Token: "server", Glyph: gui.IconDesktop},
	{Token: "cloud", Glyph: gui.IconCloud},
	{Token: "router", Glyph: gui.IconSitemap},
	{Token: "db", Glyph: gui.IconTable},
	{Token: "terminal", Glyph: gui.IconTerminal},
}

func iconGlyph(token string) string {
	for _, p := range iconPresets {
		if p.Token == token {
			return p.Glyph
		}
	}
	if token != "" {
		// Unknown tokens may be emoji/legacy; fall back to bookmark so tree stays aligned.
		return gui.IconBookmark
	}
	return gui.IconBookmark
}

func parseBookmarkColor(raw string) (gui.Color, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return gui.Color{}, false
	}
	if hex, ok := legacyColorHex[strings.ToLower(raw)]; ok {
		raw = hex
	}
	c, ok := gui.ColorFromHex(raw)
	return c, ok
}

func bookmarkSubtitle(b bookmark.Bookmark) string {
	if b.Protocol == "serial" {
		return strings.TrimSpace(b.SerialDevice)
	}
	host := strings.TrimSpace(b.Host)
	user := strings.TrimSpace(b.User)
	switch {
	case user != "" && host != "":
		return user + "@" + host
	case host != "":
		return host
	default:
		return ""
	}
}

func treeBookmarkText(b bookmark.Bookmark) string {
	sub := bookmarkSubtitle(b)
	if sub == "" {
		return b.Title
	}
	return b.Title + " — " + sub
}

func matchesQuery(b bookmark.Bookmark, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	fields := []string{b.Title, b.Host, b.User, b.SerialDevice}
	fields = append(fields, b.Tags...)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func filterGroups(groups []*bookmark.Group, q string) []*bookmark.Group {
	q = strings.TrimSpace(q)
	if q == "" {
		return groups
	}
	ql := strings.ToLower(q)
	out := make([]*bookmark.Group, 0, len(groups))
	for _, g := range groups {
		if g == nil {
			continue
		}
		groupHit := strings.Contains(strings.ToLower(displayGroupName(g.Name)), ql)
		ng := &bookmark.Group{Name: g.Name, Icon: g.Icon}
		for _, b := range g.Bookmarks {
			if matchesQuery(b, q) {
				ng.Bookmarks = append(ng.Bookmarks, b)
			}
		}
		if len(ng.Bookmarks) > 0 {
			out = append(out, ng)
			continue
		}
		if groupHit {
			cp := *g
			out = append(out, &cp)
		}
	}
	return out
}

func parseEnvLines(text string) (map[string]string, error) {
	m := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			return nil, errEnvLine(line)
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		if k == "" {
			return nil, errEnvLine(line)
		}
		m[k] = v
	}
	return m, nil
}

type envLineError string

func (e envLineError) Error() string { return string(e) }

func errEnvLine(line string) error {
	return envLineError(i18n.Tf("bookmark.env_line_invalid", line))
}

func formatEnvLines(rawJSON string) string {
	m, err := bookmark.ParseEnvVars(rawJSON)
	if err != nil || len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m[k])
	}
	return b.String()
}

func emptySSHBookmark(group string, n int) bookmark.Bookmark {
	if group == "" {
		group = bookmark.DefaultGroupName
	}
	return bookmark.Bookmark{
		GroupName: group,
		Title:     i18n.Tf("bookmark.new", n),
		Protocol:  "ssh",
		Port:      22,
		ProxyMode: "inherit",
		Charset:   "utf-8",
		UseAgent:  true,
	}
}

func emptySerialDefaults(b *bookmark.Bookmark) {
	b.BaudRate = 115200
	b.DataBits = 8
	b.Parity = "none"
	b.StopBits = 1
	b.FlowControl = "none"
	b.Charset = "utf-8"
}

func clearSSHFields(b *bookmark.Bookmark) {
	b.Host = ""
	b.Port = 0
	b.User = ""
	b.Password = ""
	b.PrivateKey = ""
	b.PrivateKeyPassword = ""
	b.ProxyJumpID = ""
	b.SshKeyID = ""
	b.Certificate = ""
	b.ForwardAgent = false
	b.ProxyMode = "inherit"
	b.ProxyType = ""
	b.ProxyHost = ""
	b.ProxyPort = 0
	b.ProxyUser = ""
	b.ProxyPassword = ""
	b.StartupCmd = ""
	b.EnvVars = ""
	b.Term = ""
}

func clearSerialFields(b *bookmark.Bookmark) {
	b.SerialDevice = ""
	b.BaudRate = 0
	b.DataBits = 0
	b.Parity = ""
	b.StopBits = 0
	b.FlowControl = ""
}

func nodeGroup(name string) string { return "g:" + name }
func nodeBook(id string) string    { return "b:" + id }
func parseNode(id string) (kind, key string) {
	switch {
	case strings.HasPrefix(id, "g:"):
		return selGroup, strings.TrimPrefix(id, "g:")
	case strings.HasPrefix(id, "b:"):
		return selBookmark, strings.TrimPrefix(id, "b:")
	default:
		return "", id
	}
}
