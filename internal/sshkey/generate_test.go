package sshkey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cryptossh "golang.org/x/crypto/ssh"
)

func TestGenerateAlgorithms(t *testing.T) {
	for _, algo := range []string{AlgoEd25519, AlgoEcdsaP256, AlgoRsa4096} {
		t.Run(algo, func(t *testing.T) {
			pub, privatePEM, err := Generate(algo, "user@host")
			if err != nil {
				t.Fatal(err)
			}
			signer, err := cryptossh.ParsePrivateKey([]byte(privatePEM))
			if err != nil {
				t.Fatal(err)
			}
			line := strings.TrimSpace(string(cryptossh.MarshalAuthorizedKey(signer.PublicKey())))
			trimmed := strings.TrimSpace(pub.PublicKey)
			if !strings.HasPrefix(trimmed, line) || !strings.HasSuffix(trimmed, "user@host") || !strings.HasSuffix(pub.PublicKey, "\n") {
				t.Fatalf("public key = %q", pub.PublicKey)
			}
			if pub.Fingerprint != cryptossh.FingerprintSHA256(signer.PublicKey()) {
				t.Fatalf("fingerprint = %s", pub.Fingerprint)
			}
			if pub.Algorithm != algo {
				t.Fatalf("algorithm = %s", pub.Algorithm)
			}
		})
	}
}

func TestGenerateRejectsUnknownAlgorithm(t *testing.T) {
	_, _, err := Generate("dsa", "")
	if err == nil || !strings.Contains(err.Error(), "Unsupported key algorithm") {
		t.Fatalf("err = %v", err)
	}
}

func TestWritePrivateKeyFilePassphrase(t *testing.T) {
	_, privatePEM, err := Generate(AlgoEd25519, "comment")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := WritePrivateKeyFile(path, privatePEM, "comment", "secret"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cryptossh.ParsePrivateKeyWithPassphrase(data, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if _, err := cryptossh.ParsePrivateKey(data); err == nil {
		t.Fatal("expected passphrase to be required")
	}
}
