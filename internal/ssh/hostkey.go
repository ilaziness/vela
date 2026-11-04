package ssh

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ilaziness/vela/internal/usererr"
)

type hostKeyStore struct {
	mu      sync.Mutex
	pending map[string]chan bool
}

func newHostKeyStore() *hostKeyStore {
	return &hostKeyStore{pending: make(map[string]chan bool)}
}

func (s *hostKeyStore) begin(host string) (chan bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[host]; ok {
		return nil, usererr.Coded("ssh.host_key_in_progress", "A host key prompt is already in progress for %s", host)
	}
	ch := make(chan bool, 1)
	s.pending[host] = ch
	return ch, nil
}

func (s *hostKeyStore) finish(host string) {
	s.mu.Lock()
	delete(s.pending, host)
	s.mu.Unlock()
}

func (s *hostKeyStore) decide(host string, accept bool) error {
	s.mu.Lock()
	ch := s.pending[host]
	s.mu.Unlock()
	if ch == nil {
		return usererr.Coded("ssh.host_key_none", "No pending host key prompt")
	}
	select {
	case ch <- accept:
	default:
	}
	return nil
}

// KnownHostEntry is one trusted host key line for management UI.
type KnownHostEntry struct {
	Host        string `json:"host"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
}

func (m *Manager) knownHostsFile() string {
	if m.knownHostsPath != "" {
		return m.knownHostsPath
	}
	return filepath.Join("data", "known_hosts")
}

func ensureKnownHostsExists(path string) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.WriteFile(path, []byte(""), 0600)
	}
	return nil
}

func appendKnownHost(path, host, keyType, keyBase64 string) error {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			tokens := strings.Fields(line)
			if len(tokens) >= 3 && tokens[0] == host && tokens[1] == keyType && tokens[2] == keyBase64 {
				return nil
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	f2, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f2.Close()
	_, err = fmt.Fprintf(f2, "%s %s %s\n", host, keyType, keyBase64)
	return err
}

// replaceKnownHost replaces the entry for host+keyType with the new key material.
func replaceKnownHost(path, host, keyType, keyBase64 string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var out strings.Builder
	replaced := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			out.WriteString(trimmed)
			out.WriteByte('\n')
			continue
		}
		tokens := strings.Fields(trimmed)
		if len(tokens) >= 3 && tokens[0] == host && tokens[1] == keyType {
			if !replaced {
				fmt.Fprintf(&out, "%s %s %s\n", host, keyType, keyBase64)
				replaced = true
			}
			continue
		}
		out.WriteString(trimmed)
		out.WriteByte('\n')
	}
	if !replaced {
		fmt.Fprintf(&out, "%s %s %s\n", host, keyType, keyBase64)
	}
	return os.WriteFile(path, []byte(out.String()), 0600)
}

func deleteKnownHost(path, host, keyType string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var out strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			out.WriteString(trimmed)
			out.WriteByte('\n')
			continue
		}
		tokens := strings.Fields(trimmed)
		if len(tokens) >= 3 && tokens[0] == host {
			if keyType == "" || tokens[1] == keyType {
				continue
			}
		}
		out.WriteString(trimmed)
		out.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(out.String()), 0600)
}

func fingerprintFromBase64(keyBase64 string) string {
	raw, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return ""
	}
	pub, err := cryptossh.ParsePublicKey(raw)
	if err != nil {
		return ""
	}
	return cryptossh.FingerprintSHA256(pub)
}

// checkKnownHostInFile returns found, mismatch, oldKeyType, oldKeyBase64, err.
// Same host + same type + different key → mismatch.
// Same host + different type → not found (treat as unknown algorithm, allow append).
func checkKnownHostInFile(path, host, keyType, keyBase64 string) (found, mismatch bool, oldType, oldBase64 string, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, "", "", nil
		}
		return false, false, "", "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) < 3 || tokens[0] != host {
			continue
		}
		if tokens[1] != keyType {
			continue
		}
		if tokens[2] == keyBase64 {
			return true, false, "", "", nil
		}
		return false, true, tokens[1], tokens[2], nil
	}
	if err := scanner.Err(); err != nil {
		return false, false, "", "", err
	}
	return false, false, "", "", nil
}

func (m *Manager) hostKeyCallback(host string, remote net.Addr, key cryptossh.PublicKey) error {
	keyType := key.Type()
	keyBase64 := base64.StdEncoding.EncodeToString(key.Marshal())
	knownPath := m.knownHostsFile()
	m.knownHostsMu.Lock()
	if err := ensureKnownHostsExists(knownPath); err != nil {
		m.knownHostsMu.Unlock()
		return err
	}
	found, mismatch, _, oldBase64, err := checkKnownHostInFile(knownPath, host, keyType, keyBase64)
	m.knownHostsMu.Unlock()
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	if m.prompter == nil {
		if mismatch {
			return usererr.Coded("ssh.host_key_mismatch", "Host key mismatch for %s", host)
		}
		return usererr.Coded("ssh.host_key_untrusted", "Host key is not trusted")
	}

	ch, err := m.hostKey.begin(host)
	if err != nil {
		return err
	}
	defer m.hostKey.finish(host)

	prompt := HostKeyPrompt{
		Host:        host,
		Address:     remote.String(),
		Fingerprint: cryptossh.FingerprintSHA256(key),
		KeyType:     keyType,
		Mismatch:    mismatch,
	}
	if mismatch {
		prompt.OldFingerprint = fingerprintFromBase64(oldBase64)
	}

	if err := m.prompter.Prompt(prompt); err != nil {
		return err
	}

	select {
	case accept := <-ch:
		if !accept {
			if mismatch {
				return usererr.Coded("ssh.host_key_mismatch", "Host key mismatch for %s", host)
			}
			return usererr.Coded("ssh.host_key_untrusted", "Host key is not trusted")
		}
		m.knownHostsMu.Lock()
		defer m.knownHostsMu.Unlock()
		if mismatch {
			return replaceKnownHost(knownPath, host, keyType, keyBase64)
		}
		return appendKnownHost(knownPath, host, keyType, keyBase64)
	case <-time.After(30 * time.Second):
		return usererr.Coded("ssh.host_key_timeout", "Host key confirmation timed out")
	}
}

func (m *Manager) ListKnownHosts() ([]KnownHostEntry, error) {
	path := m.knownHostsFile()
	m.knownHostsMu.Lock()
	defer m.knownHostsMu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []KnownHostEntry{}, nil
		}
		return nil, err
	}
	entries := make([]KnownHostEntry, 0)
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		tokens := strings.Fields(trimmed)
		if len(tokens) < 3 {
			continue
		}
		entries = append(entries, KnownHostEntry{
			Host:        tokens[0],
			KeyType:     tokens[1],
			Fingerprint: fingerprintFromBase64(tokens[2]),
		})
	}
	return entries, nil
}

func (m *Manager) DeleteKnownHost(host, keyType string) error {
	if host == "" {
		return usererr.Coded("ssh.host_required", "Host is required")
	}
	path := m.knownHostsFile()
	m.knownHostsMu.Lock()
	defer m.knownHostsMu.Unlock()
	return deleteKnownHost(path, host, keyType)
}
