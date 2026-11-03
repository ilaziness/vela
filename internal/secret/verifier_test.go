package secret

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifierRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := SaveVerifier(dir, "secret-pass"); err != nil {
		t.Fatal(err)
	}
	ct, err := LoadVerifierCiphertext(dir)
	if err != nil || ct == "" {
		t.Fatalf("load: %q %v", ct, err)
	}
	if err := CheckPassword(ct, "secret-pass"); err != nil {
		t.Fatal(err)
	}
	if err := CheckPassword(ct, "wrong"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
	if err := EnsureVerifier(dir, "other"); err != nil {
		t.Fatal(err)
	}
	// Ensure should not overwrite
	ct2, _ := LoadVerifierCiphertext(dir)
	if ct2 != ct {
		t.Fatal("EnsureVerifier overwrote existing verifier")
	}
	if _, err := os.Stat(filepath.Join(dir, verifierFileName)); err != nil {
		t.Fatal(err)
	}
}
