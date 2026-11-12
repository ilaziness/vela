package vaultcheck

import (
	"testing"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/secret"
)

func TestNeedsUnlockVerifier(t *testing.T) {
	dir := t.TempDir()
	vault := secret.NewVault()
	store := &config.Store{Config: &config.Config{}}

	if NeedsUnlock(vault, dir, store, nil) {
		t.Fatal("empty vault/dir should not need unlock")
	}
	if err := secret.SaveVerifier(dir, "master"); err != nil {
		t.Fatal(err)
	}
	if !NeedsUnlock(vault, dir, store, nil) {
		t.Fatal("verifier present and locked vault should need unlock")
	}
	vault.Set("master")
	if NeedsUnlock(vault, dir, store, nil) {
		t.Fatal("unlocked vault should not need unlock")
	}
}

func TestHasEncryptedSecretsConfig(t *testing.T) {
	store := &config.Store{Config: &config.Config{}}
	if HasEncryptedSecrets(store, nil) {
		t.Fatal("empty config should have no secrets")
	}
	store.Config.AI.APIKeyEncrypted = "cipher"
	if !HasEncryptedSecrets(store, nil) {
		t.Fatal("API key ciphertext should count")
	}
}

func TestProbePasswordConfig(t *testing.T) {
	enc, err := secret.Encrypt("master", "api-key-value")
	if err != nil {
		t.Fatal(err)
	}
	store := &config.Store{Config: &config.Config{
		AI: ai.Config{APIKeyEncrypted: enc},
	}}
	if !ProbePassword(store, nil, "master") {
		t.Fatal("correct password should probe ok")
	}
	if ProbePassword(store, nil, "wrong") {
		t.Fatal("wrong password should fail probe")
	}
}

func TestNeedsUnlockHasSecretsWithoutVerifier(t *testing.T) {
	vault := secret.NewVault()
	store := &config.Store{Config: &config.Config{
		AI: ai.Config{APIKeyEncrypted: "cipher"},
	}}
	if !NeedsUnlock(vault, t.TempDir(), store, nil) {
		t.Fatal("encrypted secrets without verifier should need unlock")
	}
}
