package tools

import (
	"strings"

	"github.com/ilaziness/vela/internal/i18n"
)

const (
	maxDiffLines = 2000
	maxDiffCells = 1_000_000 // LCS DP 表上限，避免大文本 OOM
)

// DiffLine 单行 diff
type DiffLine struct {
	Type    string `json:"type"` // equal | add | remove
	Content string `json:"content"`
	OldLine int    `json:"oldLine,omitempty"`
	NewLine int    `json:"newLine,omitempty"`
}

// DiffResult 文本对比结果
type DiffResult struct {
	Success bool       `json:"success"`
	Lines   []DiffLine `json:"lines"`
	Error   string     `json:"error,omitempty"`
}

// DiffText 按行对比两段文本（LCS 行级 diff）
func (ts *Service) DiffText(left string, right string) DiffResult {
	a := splitLines(left)
	b := splitLines(right)
	if len(a) > maxDiffLines || len(b) > maxDiffLines {
		return DiffResult{
			Success: false,
			Error:   i18n.Tf("tool.err.diff_too_many_lines", maxDiffLines),
		}
	}
	if int64(len(a))*int64(len(b)) > maxDiffCells {
		return DiffResult{
			Success: false,
			Error:   i18n.T("tool.err.diff_too_large"),
		}
	}
	ops := lcsDiff(a, b)
	return DiffResult{Success: true, Lines: ops}
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, "\n")
	for i, p := range parts {
		parts[i] = strings.TrimSuffix(p, "\r")
	}
	return parts
}

func lcsDiff(a, b []string) []DiffLine {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				dp[i][j] = dp[i+1][j+1] + 1
			case dp[i+1][j] >= dp[i][j+1]:
				dp[i][j] = dp[i+1][j]
			default:
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var lines []DiffLine
	i, j := 0, 0
	oldLine, newLine := 1, 1
	for i < m && j < n {
		switch {
		case a[i] == b[j]:
			lines = append(lines, DiffLine{Type: "equal", Content: a[i], OldLine: oldLine, NewLine: newLine})
			i++
			j++
			oldLine++
			newLine++
		case dp[i+1][j] >= dp[i][j+1]:
			lines = append(lines, DiffLine{Type: "remove", Content: a[i], OldLine: oldLine})
			i++
			oldLine++
		default:
			lines = append(lines, DiffLine{Type: "add", Content: b[j], NewLine: newLine})
			j++
			newLine++
		}
	}
	for i < m {
		lines = append(lines, DiffLine{Type: "remove", Content: a[i], OldLine: oldLine})
		i++
		oldLine++
	}
	for j < n {
		lines = append(lines, DiffLine{Type: "add", Content: b[j], NewLine: newLine})
		j++
		newLine++
	}
	return lines
}
