package bookmark

import (
	"encoding/json"
	"strings"
	"unicode"
)

const (
	maxTagRunes  = 32
	maxTagCount  = 16
	maxTagsBytes = 2048
)

// Allowed bookmark color tokens (empty = none).
var allowedColors = map[string]struct{}{
	"red": {}, "orange": {}, "yellow": {}, "green": {},
	"cyan": {}, "blue": {}, "purple": {}, "pink": {},
}

func decodeTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return []string{}
	}
	return normalizeTags(tags)
}

func encodeTags(tags []string) string {
	normalized := normalizeTags(tags)
	if len(normalized) == 0 {
		return ""
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	if len(b) > maxTagsBytes {
		// Truncate by dropping trailing tags until under limit.
		for len(normalized) > 0 && len(b) > maxTagsBytes {
			normalized = normalized[:len(normalized)-1]
			b, err = json.Marshal(normalized)
			if err != nil {
				return ""
			}
		}
		if len(normalized) == 0 {
			return ""
		}
	}
	return string(b)
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || hasControlOrNewline(t) {
			continue
		}
		runes := []rune(t)
		if len(runes) > maxTagRunes {
			t = string(runes[:maxTagRunes])
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
		if len(out) >= maxTagCount {
			break
		}
	}
	return out
}

func hasControlOrNewline(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func normalizeColor(color string) string {
	color = strings.TrimSpace(strings.ToLower(color))
	if color == "" {
		return ""
	}
	if _, ok := allowedColors[color]; ok {
		return color
	}
	return ""
}

func normalizeMeta(b *Bookmark) {
	b.Tags = normalizeTags(b.Tags)
	b.Color = normalizeColor(b.Color)
}
