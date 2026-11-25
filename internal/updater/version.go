package updater

import (
	"strconv"
	"strings"
)

// normalizeVersion strips a leading v/V and whitespace for comparison.
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	return v
}

// compareVersions compares dotted numeric versions. It returns 1 when a>b,
// -1 when a<b and 0 when equal. Non-numeric suffixes are ignored per segment.
func compareVersions(a, b string) int {
	as := strings.Split(normalizeVersion(a), ".")
	bs := strings.Split(normalizeVersion(b), ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		ai, bi := versionPart(as, i), versionPart(bs, i)
		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}

func versionPart(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	s := strings.TrimSpace(parts[i])
	j := 0
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == 0 {
		return 0
	}
	n, err := strconv.Atoi(s[:j])
	if err != nil {
		return 0
	}
	return n
}
