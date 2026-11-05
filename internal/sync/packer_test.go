package sync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPackAndUnpackStream(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	testFile1 := filepath.Join(srcDir, "file1.txt")
	testContent1 := []byte("Hello, World!")
	if err := os.WriteFile(testFile1, testContent1, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	testFile2 := filepath.Join(subDir, "file2.txt")
	testContent2 := []byte("Nested file content")
	if err := os.WriteFile(testFile2, testContent2, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	password := "test-encryption-key"

	var packedBuffer bytes.Buffer
	err := PackStream(srcDir, &packedBuffer, password)
	if err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	packedData := packedBuffer.Bytes()
	if len(packedData) == 0 {
		t.Fatal("packed data should not be empty")
	}

	err = UnpackStream(bytes.NewReader(packedData), dstDir, password)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	extractedFile1 := filepath.Join(dstDir, "file1.txt")
	extractedContent1, err := os.ReadFile(extractedFile1)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(extractedContent1, testContent1) {
		t.Errorf("file1 content mismatch: expected %s, got %s", testContent1, extractedContent1)
	}

	extractedFile2 := filepath.Join(dstDir, "subdir", "file2.txt")
	extractedContent2, err := os.ReadFile(extractedFile2)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(extractedContent2, testContent2) {
		t.Errorf("file2 content mismatch: expected %s, got %s", testContent2, extractedContent2)
	}
}

func TestPackSkipsLocalState(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "local-state.toml"), []byte("last_dialog_dir=\"x\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var packed bytes.Buffer
	if err := PackStream(srcDir, &packed, "key"); err != nil {
		t.Fatal(err)
	}
	if err := UnpackStream(bytes.NewReader(packed.Bytes()), dstDir, "key"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "keep.txt")); err != nil {
		t.Fatalf("keep.txt missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "local-state.toml")); !os.IsNotExist(err) {
		t.Fatalf("local-state.toml should be skipped, err=%v", err)
	}
}
