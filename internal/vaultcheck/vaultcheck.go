// Package vaultcheck detects whether the process vault must be unlocked
// and probes a candidate master password against stored ciphertexts.
package vaultcheck

import (
	"strings"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/secret"
)

// HasEncryptedSecrets reports whether config or DB holds master-password
// ciphertext (API keys, sync secrets, bookmark passwords, SSH private keys).
func HasEncryptedSecrets(cfgStore *config.Store, db *database.Database) bool {
	if cfgStore != nil && cfgStore.Config != nil {
		aiCfg := cfgStore.Config.AI
		if strings.TrimSpace(aiCfg.APIKeyEncrypted) != "" || strings.TrimSpace(aiCfg.WebSearch.APIKeyEncrypted) != "" {
			return true
		}
		syncCfg := cfgStore.Config.Sync
		if strings.TrimSpace(syncCfg.SelfHosted.UserKeyEncrypted) != "" ||
			strings.TrimSpace(syncCfg.WebDAV.PasswordEncrypted) != "" ||
			strings.TrimSpace(syncCfg.S3.SecretKeyEncrypted) != "" {
			return true
		}
	}
	if db == nil {
		return false
	}
	if bookmarks, err := db.BookmarkRepo.GetAllBookmarks(); err == nil {
		for _, b := range bookmarks {
			if b == nil {
				continue
			}
			if b.Password != "" || b.PrivateKeyPassword != "" || b.ProxyPassword != "" {
				return true
			}
		}
	}
	if keys, err := db.SSHKeyRepo.List(); err == nil {
		for _, k := range keys {
			if k != nil && k.PrivateKey != "" {
				return true
			}
		}
	}
	return false
}

// ProbePassword checks the master password against a few ciphertexts.
// Argon2 is expensive; cap attempts so a wrong password cannot decrypt every row.
func ProbePassword(cfgStore *config.Store, db *database.Database, password string) bool {
	const maxAttempts = 5
	attempts := 0
	try := func(enc string) bool {
		enc = strings.TrimSpace(enc)
		if enc == "" || attempts >= maxAttempts {
			return false
		}
		attempts++
		_, err := secret.Decrypt(password, enc)
		return err == nil
	}
	if cfgStore != nil && cfgStore.Config != nil {
		aiCfg := cfgStore.Config.AI
		if try(aiCfg.APIKeyEncrypted) || try(aiCfg.WebSearch.APIKeyEncrypted) {
			return true
		}
		syncCfg := cfgStore.Config.Sync
		if try(syncCfg.SelfHosted.UserKeyEncrypted) ||
			try(syncCfg.WebDAV.PasswordEncrypted) ||
			try(syncCfg.S3.SecretKeyEncrypted) {
			return true
		}
	}
	if db == nil {
		return false
	}
	if bookmarks, err := db.BookmarkRepo.GetAllBookmarks(); err == nil {
		for _, b := range bookmarks {
			if b == nil {
				continue
			}
			if try(b.Password) || try(b.PrivateKeyPassword) || try(b.ProxyPassword) {
				return true
			}
			if attempts >= maxAttempts {
				return false
			}
		}
	}
	if keys, err := db.SSHKeyRepo.List(); err == nil {
		for _, k := range keys {
			if k != nil && try(k.PrivateKey) {
				return true
			}
			if attempts >= maxAttempts {
				return false
			}
		}
	}
	return false
}

// NeedsUnlock is true when encrypted secrets or a verifier exist and the vault is locked.
func NeedsUnlock(vault *secret.Vault, userDataDir string, cfgStore *config.Store, db *database.Database) bool {
	if _, err := vault.Get(); err == nil {
		return false
	}
	ct, err := secret.LoadVerifierCiphertext(userDataDir)
	if err == nil && ct != "" {
		return true
	}
	return HasEncryptedSecrets(cfgStore, db)
}
