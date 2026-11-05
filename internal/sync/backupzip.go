package sync

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	payloadEntryName  = "payload.bin"
	manifestEntryName = "manifest.json"
	tempDirPrefix     = "vela-sync-"
	zipNameLayout     = "vela-backup-20060102T150405Z"
)

// BackupManifest describes the encrypted payload inside a backup zip.
type BackupManifest struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func newSyncTempDir() (string, error) {
	dir, err := os.MkdirTemp("", tempDirPrefix)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func backupZipFileName() (string, error) {
	stamp := time.Now().UTC().Format(zipNameLayout)
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s.zip", stamp, hex.EncodeToString(suffix)), nil
}

func hashFileSHA256(path string) (sum [sha256.Size]byte, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return sum, 0, err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return sum, 0, err
	}
	copy(sum[:], h.Sum(nil))
	return sum, n, nil
}

func writeManifest(path string, sum [sha256.Size]byte, size int64) error {
	manifest := BackupManifest{
		SHA256: hex.EncodeToString(sum[:]),
		Size:   size,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func createStoreZip(zipPath, payloadPath, manifestPath string) error {
	out, err := os.OpenFile(zipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(out)
	if err := addStoreFile(zw, payloadEntryName, payloadPath); err != nil {
		_ = zw.Close()
		_ = out.Close()
		return err
	}
	if err := addStoreFile(zw, manifestEntryName, manifestPath); err != nil {
		_ = zw.Close()
		_ = out.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func addStoreFile(zw *zip.Writer, name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Store

	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func extractBackupZip(zipPath, destDir string) (payloadPath, manifestPath string, err error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", err
	}
	defer zr.Close()

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", "", err
	}

	var seenPayload, seenManifest bool
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if name != payloadEntryName && name != manifestEntryName {
			return "", "", fmt.Errorf("unexpected zip entry: %q", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("symlink not allowed: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			return "", "", fmt.Errorf("directory entry not allowed: %q", f.Name)
		}
		if name == payloadEntryName && seenPayload {
			return "", "", fmt.Errorf("duplicate zip entry: %q", name)
		}
		if name == manifestEntryName && seenManifest {
			return "", "", fmt.Errorf("duplicate zip entry: %q", name)
		}

		outPath := filepath.Join(destDir, name)
		if err := writeZipEntryLimited(f, outPath, f.UncompressedSize64); err != nil {
			return "", "", err
		}
		switch name {
		case payloadEntryName:
			seenPayload = true
			payloadPath = outPath
		case manifestEntryName:
			seenManifest = true
			manifestPath = outPath
		}
	}
	if !seenPayload || !seenManifest {
		return "", "", fmt.Errorf("backup zip missing required entries")
	}
	return payloadPath, manifestPath, nil
}

func writeZipEntryLimited(f *zip.File, outPath string, limit uint64) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, io.LimitReader(rc, int64(limit)+1))
	if err != nil {
		return err
	}
	if uint64(written) > limit {
		return fmt.Errorf("zip entry %q exceeds declared size", f.Name)
	}
	return nil
}

func verifyPayload(payloadPath, manifestPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest BackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}
	expected, err := hex.DecodeString(manifest.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return fmt.Errorf("invalid manifest sha256")
	}

	sum, size, err := hashFileSHA256(payloadPath)
	if err != nil {
		return err
	}
	if size != manifest.Size {
		return fmt.Errorf("payload size mismatch: got %d want %d", size, manifest.Size)
	}
	if subtle.ConstantTimeCompare(sum[:], expected) != 1 {
		return fmt.Errorf("payload sha256 mismatch")
	}
	return nil
}
