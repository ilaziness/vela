package secret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	verifierFileName = "vault_verifier"
	verifierSentinel = "vela-vault-v1"
)

// VerifierPath returns the path of the master-password verifier file under userDataDir.
func VerifierPath(userDataDir string) string {
	return filepath.Join(strings.TrimSpace(userDataDir), verifierFileName)
}

// LoadVerifierCiphertext reads the persisted verifier blob, or "" if missing.
func LoadVerifierCiphertext(userDataDir string) (string, error) {
	path := VerifierPath(userDataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveVerifier encrypts the sentinel with password and writes it under userDataDir.
func SaveVerifier(userDataDir, password string) error {
	if strings.TrimSpace(userDataDir) == "" {
		return errors.New("user data dir is empty")
	}
	if password == "" {
		return errors.New("password is empty")
	}
	enc, err := Encrypt(password, verifierSentinel)
	if err != nil {
		return err
	}
	path := VerifierPath(userDataDir)
	return os.WriteFile(path, []byte(enc+"\n"), 0o600)
}

// CheckPassword validates password against the verifier ciphertext.
func CheckPassword(ciphertext, password string) error {
	plain, err := Decrypt(password, ciphertext)
	if err != nil {
		return err
	}
	if plain != verifierSentinel {
		return errors.New("invalid vault verifier")
	}
	return nil
}

// EnsureVerifier creates a verifier file if none exists yet.
func EnsureVerifier(userDataDir, password string) error {
	existing, err := LoadVerifierCiphertext(userDataDir)
	if err != nil {
		return err
	}
	if existing != "" {
		return nil
	}
	return SaveVerifier(userDataDir, password)
}
