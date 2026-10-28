package sshconfig

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Resolve merges Host blocks with OpenSSH first-obtained-value-wins semantics
// for each concrete (non-wildcard) alias found in Host patterns.
func Resolve(blocks []RawHostBlock) ([]HostEntry, []Warning) {
	var warnings []Warning
	seen := map[string]bool{}
	var aliases []string
	for _, b := range blocks {
		if b.IsMatch {
			continue
		}
		for _, p := range b.Patterns {
			if p == "" || isPattern(p) {
				continue
			}
			if seen[p] {
				continue
			}
			seen[p] = true
			aliases = append(aliases, p)
		}
	}

	var entries []HostEntry
	for _, alias := range aliases {
		e, warns := resolveAlias(alias, blocks)
		warnings = append(warnings, warns...)
		// Bare "Host alias" is valid OpenSSH: HostName defaults to the alias.
		entries = append(entries, e)
	}

	// Report wildcard-only blocks that never contributed a concrete alias of their own.
	for _, b := range blocks {
		if b.IsMatch {
			continue
		}
		hasConcrete := false
		for _, p := range b.Patterns {
			if !isPattern(p) {
				hasConcrete = true
				break
			}
		}
		if !hasConcrete && len(b.Patterns) > 0 {
			warnings = append(warnings, Warning{Message: fmt.Sprintf("%s: wildcard Host %s used for defaults only; not imported as a bookmark", b.Source, strings.Join(b.Patterns, " "))})
		}
	}
	return entries, warnings
}

func resolveAlias(alias string, blocks []RawHostBlock) (HostEntry, []Warning) {
	var warnings []Warning
	e := HostEntry{Alias: alias}
	got := map[string]bool{}

	take := func(key string) bool {
		if got[key] {
			return false
		}
		got[key] = true
		return true
	}

	for _, b := range blocks {
		if b.IsMatch {
			continue
		}
		if !patternsMatch(b.Patterns, alias) {
			continue
		}
		kw := b.Keywords
		if vals := kw["hostname"]; len(vals) > 0 && take("hostname") {
			e.HostName = unquoteSSH(vals[0])
		}
		if vals := kw["user"]; len(vals) > 0 && take("user") {
			e.User = unquoteSSH(vals[0])
		}
		if vals := kw["port"]; len(vals) > 0 && !got["port"] {
			raw := unquoteSSH(vals[0])
			if p, err := strconv.Atoi(raw); err == nil {
				got["port"] = true
				e.Port = p
			} else {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("Host %s: invalid Port %q", alias, vals[0])})
			}
		}
		if vals := kw["identityfile"]; len(vals) > 0 && take("identityfile") {
			for _, v := range vals {
				expanded, err := ExpandPath(v)
				if err != nil {
					warnings = append(warnings, Warning{Message: fmt.Sprintf("Host %s: IdentityFile expand failed %s: %v", alias, v, err)})
					// ExpandPath still returns an unquoted path on failure.
				}
				if expanded != "" {
					e.IdentityFiles = append(e.IdentityFiles, expanded)
				}
			}
			if len(vals) > 1 {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("Host %s: multiple IdentityFile entries; only the first is imported", alias)})
			}
		}
		if vals := kw["certificatefile"]; len(vals) > 0 && take("certificatefile") {
			expanded, err := ExpandPath(vals[0])
			if err != nil {
				warnings = append(warnings, Warning{Message: fmt.Sprintf("Host %s: CertificateFile expand failed: %v", alias, err)})
			}
			e.CertificateFile = expanded
		}
		if vals := kw["proxyjump"]; len(vals) > 0 && take("proxyjump") {
			e.ProxyJump = unquoteSSH(vals[0])
		}
		if vals := kw["forwardagent"]; len(vals) > 0 && take("forwardagent") {
			v := strings.ToLower(unquoteSSH(vals[0]))
			yes := v == "yes" || v == "true" || v == "1"
			e.ForwardAgent = &yes
		}
		if vals := kw["remotecommand"]; len(vals) > 0 && take("remotecommand") {
			e.RemoteCommand = unquoteSSH(vals[0])
		}
		// SetEnv: first-obtained value wins per key; merge across matching Host blocks.
		if vals := kw["setenv"]; len(vals) > 0 {
			for _, v := range vals {
				k, val, ok := strings.Cut(v, "=")
				k = strings.TrimSpace(k)
				if !ok || k == "" {
					warnings = append(warnings, Warning{Message: fmt.Sprintf("Host %s: invalid SetEnv %q", alias, v)})
					continue
				}
				val = unquoteSSH(val)
				if setEnvHasKey(e.SetEnv, k) {
					continue
				}
				e.SetEnv = append(e.SetEnv, KV{Key: k, Value: val})
			}
		}
	}
	return e, warnings
}

func setEnvHasKey(env []KV, key string) bool {
	for _, kv := range env {
		if strings.EqualFold(kv.Key, key) {
			return true
		}
	}
	return false
}

func isPattern(p string) bool {
	return strings.ContainsAny(p, "*?!")
}

// patternsMatch follows OpenSSH Host semantics: match if any positive pattern
// matches, unless a negated (!pat) pattern also matches.
func patternsMatch(patterns []string, alias string) bool {
	matched := false
	for _, p := range patterns {
		neg := strings.HasPrefix(p, "!")
		pat := p
		if neg {
			pat = p[1:]
		}
		ok := pat == alias || globMatch(pat, alias)
		if neg {
			if ok {
				return false
			}
			continue
		}
		if ok {
			matched = true
		}
	}
	return matched
}

func globMatch(pattern, name string) bool {
	// filepath.Match is close enough for * and ?
	ok, err := filepath.Match(pattern, name)
	return err == nil && ok
}
