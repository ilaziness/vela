package sshconfig

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxIncludeDepth = 3

// known keys we map; others generate warnings but do not drop the host.
var mappedKeys = map[string]bool{
	"hostname": true, "user": true, "port": true,
	"identityfile": true, "certificatefile": true,
	"proxyjump": true, "forwardagent": true,
	"remotecommand": true, "setenv": true,
}

var warnOnlyKeys = map[string]bool{
	"proxycommand": true, "localforward": true, "remoteforward": true,
	"dynamicforward": true, "controlmaster": true, "controlpath": true,
	"controlpersist": true, "sendenv": true, "requesttty": true,
	"stricthostkeychecking": true, "userknownhostsfile": true,
	"canonicalizename": true,
}

// ParseFile reads an OpenSSH config file with Include expansion.
func ParseFile(path string) ([]RawHostBlock, []Warning, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return Parse(f, filepath.Dir(abs), abs, 0)
}

// Parse reads config content. baseDir is used for relative Include paths.
func Parse(r io.Reader, baseDir, source string, depth int) ([]RawHostBlock, []Warning, error) {
	var blocks []RawHostBlock
	var warnings []Warning
	sc := bufio.NewScanner(r)
	var cur *RawHostBlock

	flush := func() {
		if cur != nil {
			blocks = append(blocks, *cur)
			cur = nil
		}
	}

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value := splitKV(line)
		if key == "" {
			warnings = append(warnings, Warning{Message: fmt.Sprintf("%s:%d: unable to parse line", source, lineNo)})
			continue
		}
		lk := strings.ToLower(key)

		if lk == "include" {
			if depth >= maxIncludeDepth {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("%s:%d: Include nested too deep; skipped %s", source, lineNo, value)})
				continue
			}
			incBlocks, incWarns, err := parseIncludes(value, baseDir, depth+1)
			warnings = append(warnings, incWarns...)
			if err != nil {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("%s:%d: Include failed: %v", source, lineNo, err)})
				continue
			}
			flush()
			blocks = append(blocks, incBlocks...)
			continue
		}

		if lk == "host" || lk == "match" {
			flush()
			if lk == "match" {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("%s:%d: skipped Match block", source, lineNo)})
				cur = &RawHostBlock{IsMatch: true, Keywords: map[string][]string{}, Source: source}
				continue
			}
			patterns := splitPatterns(value)
			cur = &RawHostBlock{Patterns: patterns, Keywords: map[string][]string{}, Source: source}
			continue
		}

		if cur == nil {
			// Global keywords before first Host — treat as Host *
			cur = &RawHostBlock{Patterns: []string{"*"}, Keywords: map[string][]string{}, Source: source}
		}
		if cur.IsMatch {
			continue
		}
		if !mappedKeys[lk] {
			if warnOnlyKeys[lk] {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("%s: directive %s not imported (Host %s)", source, key, strings.Join(cur.Patterns, " "))})
			} else {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("%s: unknown/unsupported directive %s (Host %s)", source, key, strings.Join(cur.Patterns, " "))})
			}
			continue
		}
		cur.Keywords[lk] = append(cur.Keywords[lk], value)
	}
	flush()
	if err := sc.Err(); err != nil {
		return blocks, warnings, err
	}
	return blocks, warnings, nil
}

func parseIncludes(pattern, baseDir string, depth int) ([]RawHostBlock, []Warning, error) {
	var all []RawHostBlock
	var warnings []Warning
	for _, p := range strings.Fields(pattern) {
		expanded, err := ExpandPath(p)
		if err != nil {
			warnings = append(warnings, Warning{Message: fmt.Sprintf("Include path expand failed %s: %v", p, err)})
			expanded = p
		}
		if !filepath.IsAbs(expanded) {
			expanded = filepath.Join(baseDir, expanded)
		}
		matches, err := filepath.Glob(expanded)
		if err != nil {
			return all, warnings, err
		}
		if len(matches) == 0 {
			warnings = append(warnings, Warning{Message: fmt.Sprintf("Include matched nothing: %s", expanded)})
			continue
		}
		for _, m := range matches {
			b, w, err := parseFileAtDepth(m, depth)
			warnings = append(warnings, w...)
			if err != nil {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("Include read failed %s: %v", m, err)})
				continue
			}
			all = append(all, b...)
		}
	}
	return all, warnings, nil
}

func parseFileAtDepth(path string, depth int) ([]RawHostBlock, []Warning, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return Parse(f, filepath.Dir(abs), abs, depth)
}

func splitKV(line string) (key, value string) {
	// Allow "Key=value" or "Key value"
	line = strings.TrimSpace(line)
	if i := strings.IndexByte(line, '='); i > 0 && !strings.ContainsAny(line[:i], " \t") {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	key = fields[0]
	if len(fields) == 1 {
		return key, ""
	}
	// Rejoin remainder preserving internal spaces for RemoteCommand etc.
	idx := strings.Index(line, key)
	rest := strings.TrimSpace(line[idx+len(key):])
	return key, rest
}

func splitPatterns(value string) []string {
	return splitSSHTokens(value)
}
