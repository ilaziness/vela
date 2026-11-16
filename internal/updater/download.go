package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"
)

// Install downloads rel.Asset, verifies SHA256, extracts the binary and
// schedules the self-replace helper. progress receives written/total bytes.
func (m *Manager) Install(ctx context.Context, rel *Release, progress func(written, total int64)) error {
	m.logger.Debug("update install started", releaseLogFields(rel)...)
	err := m.install(ctx, rel, progress)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			m.logger.Info("update install canceled", releaseLogFields(rel)...)
		case errors.Is(err, errUpdateInProgress):
			m.logger.Warn("update install already in progress", releaseLogFields(rel)...)
		default:
			m.logger.Error("update install failed", append(releaseLogFields(rel), zap.Error(err))...)
		}
	}
	return err
}

func (m *Manager) install(ctx context.Context, rel *Release, progress func(written, total int64)) error {
	if rel == nil {
		return errors.New("no release")
	}
	if rel.Asset.URL == "" {
		return errors.New("release has no downloadable asset")
	}
	if strings.TrimSpace(rel.Asset.SHA256) == "" {
		return errors.New("release checksum unavailable")
	}

	dlCtx, cancel := context.WithCancel(ctx)

	m.installMu.Lock()
	if m.status.Busy() {
		m.installMu.Unlock()
		cancel()
		return errUpdateInProgress
	}
	m.status = InstallStatus{Phase: PhaseDownload, Total: rel.Asset.Size}
	m.cancel = cancel
	m.installMu.Unlock()
	defer func() {
		m.setCancel(nil)
		cancel()
	}()

	tmp, err := os.MkdirTemp("", "vela-update-")
	if err != nil {
		return m.failInstall(err)
	}
	installed := false
	defer func() {
		if !installed {
			_ = os.RemoveAll(tmp)
		}
	}()

	assetName := filepath.Base(rel.Asset.Name)
	if assetName == "." || assetName == string(filepath.Separator) || assetName == "" {
		assetName = "asset"
	}
	archivePath := filepath.Join(tmp, assetName)
	if err := m.download(dlCtx, rel.Asset, archivePath, progress); err != nil {
		return m.failInstall(err)
	}
	m.logger.Debug("update asset downloaded",
		zap.String("asset", rel.Asset.Name), zap.Int64("size", rel.Asset.Size))

	m.setStatus(InstallStatus{Phase: PhaseVerify, Total: rel.Asset.Size, Written: rel.Asset.Size})
	if err := verifySHA256(archivePath, rel.Asset.SHA256); err != nil {
		return m.failInstall(err)
	}
	m.logger.Debug("update checksum verified", zap.String("asset", rel.Asset.Name))

	m.setStatus(InstallStatus{Phase: PhaseInstall})
	exePath, err := extractExecutable(archivePath, rel.Asset.Name, tmp)
	if err != nil {
		return m.failInstall(err)
	}

	manifest := applyManifest{
		ParentPID: os.Getpid(),
		OldExe:    exePathOfSelf(),
		NewExe:    exePath,
		WorkDir:   mustGetwd(),
		Args:      cleanArgs(os.Args[1:]),
		TempDir:   tmp,
	}
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return m.failInstall(err)
	}
	if err := spawnHelper(manifest.OldExe, manifestPath); err != nil {
		return m.failInstall(err)
	}
	m.logger.Debug("update helper spawned",
		zap.String("old_exe", manifest.OldExe), zap.String("new_exe", manifest.NewExe))

	installed = true
	m.setStatus(InstallStatus{Phase: PhaseDone, Done: true})
	return nil
}

func releaseLogFields(rel *Release) []zap.Field {
	if rel == nil {
		return []zap.Field{zap.String("release", "<nil>")}
	}
	return []zap.Field{
		zap.String("source", rel.Source),
		zap.String("tag", rel.Tag),
		zap.String("asset", rel.Asset.Name),
	}
}

func (m *Manager) failInstall(err error) error {
	m.setStatus(InstallStatus{Phase: PhaseIdle, Err: err})
	return err
}

func (m *Manager) download(ctx context.Context, asset Asset, dest string, progress func(written, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "vela-updater")
	resp, err := m.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	cw := &countWriter{w: out, total: total, fn: func(written, total int64) {
		m.setStatus(InstallStatus{Phase: PhaseDownload, Written: written, Total: total})
		if progress != nil {
			progress(written, total)
		}
	}}
	_, err = io.Copy(cw, resp.Body)
	return err
}

type countWriter struct {
	w       io.Writer
	written int64
	total   int64
	fn      func(written, total int64)
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.written += int64(n)
	if c.fn != nil {
		c.fn(c.written, c.total)
	}
	return n, err
}

func verifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want)) {
		return errors.New("checksum mismatch")
	}
	return nil
}

func extractExecutable(archivePath, assetName, workDir string) (string, error) {
	payload := filepath.Join(workDir, "payload")
	if err := os.MkdirAll(payload, 0o755); err != nil {
		return "", err
	}
	expected := expectedBinaryNames()
	name := strings.ToLower(assetName)
	var out string
	var err error
	switch {
	case strings.HasSuffix(name, ".zip"):
		out, err = extractZip(archivePath, payload, expected)
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		out, err = extractTarGz(archivePath, payload, expected)
	default:
		return "", fmt.Errorf("unsupported archive %q", assetName)
	}
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", errors.New("no executable found in archive")
	}
	if err := os.Chmod(out, 0o755); err != nil && runtime.GOOS != "windows" {
		return "", err
	}
	return out, nil
}

func expectedBinaryNames() map[string]bool {
	set := map[string]bool{
		"vela": true, "vela.exe": true, "vexo": true, "vexo.exe": true,
	}
	if exe, err := os.Executable(); err == nil {
		set[strings.ToLower(filepath.Base(exe))] = true
	}
	return set
}

func extractZip(archivePath, destDir string, expected map[string]bool) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	out := ""
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		clean := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		base := strings.ToLower(path.Base(clean))
		if strings.Contains(clean, "..") || !expected[base] {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out = filepath.Join(destDir, "newbin"+binExt())
		if err := writeFile(out, rc); err != nil {
			rc.Close()
			return "", err
		}
		rc.Close()
		break
	}
	return out, nil
}

func extractTarGz(archivePath, destDir string, expected map[string]bool) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := ""
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		clean := path.Clean(strings.ReplaceAll(hdr.Name, "\\", "/"))
		base := strings.ToLower(path.Base(clean))
		if strings.Contains(clean, "..") || !expected[base] {
			continue
		}
		out = filepath.Join(destDir, "newbin"+binExt())
		if err := writeFile(out, tr); err != nil {
			return "", err
		}
		break
	}
	return out, nil
}

func writeFile(dest string, r io.Reader) error {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func binExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func exePathOfSelf() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func cleanArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == applyArg {
			continue
		}
		out = append(out, a)
	}
	return out
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// spawnHelper re-executes the current binary in apply mode, detached.
func spawnHelper(exePath, manifestPath string) error {
	cmd := exec.Command(exePath, applyArg, manifestPath)
	cmd.SysProcAttr = detachProcAttr()
	return cmd.Start()
}
