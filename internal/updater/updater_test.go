package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

type fakeSource struct {
	rel *Release
	err error
}

func (f fakeSource) Name() string { return "fake" }
func (f fakeSource) Check(context.Context, string) (*Release, error) {
	return f.rel, f.err
}

func TestManagerCheckCachesLatest(t *testing.T) {
	m := NewManager(zap.NewNop(), "1.0.0", fakeSource{rel: &Release{Version: "2.0.0", Tag: "v2.0.0"}})
	rel, err := m.Check(context.Background())
	if err != nil || rel == nil {
		t.Fatalf("rel=%v err=%v", rel, err)
	}
	if m.Latest() != rel {
		t.Fatal("latest not cached")
	}
}

func TestExtractZipAndTarGz(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("Vela.exe")
	_, _ = w.Write([]byte("exe"))
	_, _ = zw.Create("../evil.exe")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zf.Close()
	out, err := extractExecutable(zipPath, "a.zip", dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(out) != "newbin"+binExt() {
		t.Fatalf("out=%s", out)
	}

	tarPath := filepath.Join(dir, "b.tar.gz")
	tf, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(tf)
	tw := tar.NewWriter(gz)
	body := []byte("bin")
	if err := tw.WriteHeader(&tar.Header{Name: "Vela", Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	tf.Close()
	out2, err := extractExecutable(tarPath, "b.tar.gz", dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(out2) != "newbin"+binExt() {
		t.Fatalf("out2=%s", out2)
	}
}
