package sync

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndVerifyBackupZip(t *testing.T) {
	tmp := t.TempDir()
	payload := []byte("encrypted-payload-bytes")
	payloadPath := filepath.Join(tmp, payloadEntryName)
	if err := os.WriteFile(payloadPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(payload)
	manifestPath := filepath.Join(tmp, manifestEntryName)
	if err := writeManifest(manifestPath, sum, int64(len(payload))); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(tmp, "vela-backup-test.zip")
	if err := createStoreZip(zipPath, payloadPath, manifestPath); err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(tmp, "extract")
	gotPayload, gotManifest, err := extractBackupZip(zipPath, extractDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPayload(gotPayload, gotManifest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(gotManifest)
	if err != nil {
		t.Fatal(err)
	}
	var m BackupManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("manifest hash %s", m.SHA256)
	}
	if m.Size != int64(len(payload)) {
		t.Fatalf("manifest size %d", m.Size)
	}
}

func TestVerifyPayloadRejectsTamper(t *testing.T) {
	tmp := t.TempDir()
	payloadPath := filepath.Join(tmp, payloadEntryName)
	if err := os.WriteFile(payloadPath, []byte("good"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("good"))
	manifestPath := filepath.Join(tmp, manifestEntryName)
	if err := writeManifest(manifestPath, sum, 4); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(payloadPath, []byte("evil"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPayload(payloadPath, manifestPath); err == nil {
		t.Fatal("expected hash mismatch")
	}
}

func TestExtractRejectsDuplicateEntry(t *testing.T) {
	tmp := t.TempDir()
	payloadPath := filepath.Join(tmp, payloadEntryName)
	manifestPath := filepath.Join(tmp, manifestEntryName)
	if err := os.WriteFile(payloadPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("x"))
	if err := writeManifest(manifestPath, sum, 1); err != nil {
		t.Fatal(err)
	}

	dupZip := filepath.Join(tmp, "dup.zip")
	f, err := os.Create(dupZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if err := addStoreFile(zw, payloadEntryName, payloadPath); err != nil {
		t.Fatal(err)
	}
	if err := addStoreFile(zw, payloadEntryName, payloadPath); err != nil {
		t.Fatal(err)
	}
	if err := addStoreFile(zw, manifestEntryName, manifestPath); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := extractBackupZip(dupZip, filepath.Join(tmp, "out")); err == nil {
		t.Fatal("expected rejection of duplicate entry")
	}
}

func TestExtractRejectsExtraEntry(t *testing.T) {
	tmp := t.TempDir()
	payloadPath := filepath.Join(tmp, payloadEntryName)
	manifestPath := filepath.Join(tmp, manifestEntryName)
	if err := os.WriteFile(payloadPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("x"))
	if err := writeManifest(manifestPath, sum, 1); err != nil {
		t.Fatal(err)
	}

	extraZip := filepath.Join(tmp, "extra.zip")
	f, err := os.Create(extraZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if err := addStoreFile(zw, payloadEntryName, payloadPath); err != nil {
		t.Fatal(err)
	}
	if err := addStoreFile(zw, manifestEntryName, manifestPath); err != nil {
		t.Fatal(err)
	}
	w, err := zw.Create("evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("nope")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := extractBackupZip(extraZip, filepath.Join(tmp, "out")); err == nil {
		t.Fatal("expected rejection of extra entry")
	}
}
