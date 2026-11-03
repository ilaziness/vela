package sync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPackAndUnpackStream(t *testing.T) {
	// 创建临时源目录
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// 创建测试文件
	testFile1 := filepath.Join(srcDir, "file1.txt")
	testContent1 := []byte("Hello, World!")
	if err := os.WriteFile(testFile1, testContent1, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 创建子目录和文件
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	testFile2 := filepath.Join(subDir, "file2.txt")
	testContent2 := []byte("Nested file content")
	if err := os.WriteFile(testFile2, testContent2, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	userKey := "test-encryption-key"

	// 打包
	var packedBuffer bytes.Buffer
	err := PackStream(srcDir, &packedBuffer, userKey)
	if err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	packedData := packedBuffer.Bytes()
	if len(packedData) == 0 {
		t.Fatal("packed data should not be empty")
	}

	// 解包到新目录
	err = UnpackStream(bytes.NewReader(packedData), dstDir, userKey)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	// 验证文件1
	extractedFile1 := filepath.Join(dstDir, "file1.txt")
	extractedContent1, err := os.ReadFile(extractedFile1)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(extractedContent1, testContent1) {
		t.Errorf("file1 content mismatch: expected %s, got %s", testContent1, extractedContent1)
	}

	// 验证文件2
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

func TestPackStreamWithProgress(t *testing.T) {
	srcDir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(srcDir, "test.txt")
	testContent := []byte("Test content for progress tracking")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	userKey := "test-key"
	var packedBuffer bytes.Buffer

	var progressCalled bool
	err := PackStreamWithProgress(srcDir, &packedBuffer, userKey, func(done, total int64) {
		progressCalled = true
		if done < 0 || total < 0 {
			t.Error("progress values should be non-negative")
		}
		if done > total {
			t.Error("done should not exceed total")
		}
	})

	if err != nil {
		t.Fatalf("failed to pack with progress: %v", err)
	}

	if !progressCalled {
		t.Error("progress callback should have been called")
	}
}

func TestUnpackStreamWithProgress(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// 创建并打包测试文件
	testFile := filepath.Join(srcDir, "test.txt")
	testContent := []byte("Test content for unpack progress")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	userKey := "test-key"
	var packedBuffer bytes.Buffer
	err := PackStream(srcDir, &packedBuffer, userKey)
	if err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	// 解包并跟踪进度
	var progressCalled bool
	err = UnpackStreamWithProgress(bytes.NewReader(packedBuffer.Bytes()), dstDir, userKey, int64(packedBuffer.Len()), func(done, total int64) {
		progressCalled = true
		if done < 0 || total < 0 {
			t.Error("progress values should be non-negative")
		}
	})

	if err != nil {
		t.Fatalf("failed to unpack with progress: %v", err)
	}

	if !progressCalled {
		t.Error("progress callback should have been called")
	}

	// 验证文件
	extractedFile := filepath.Join(dstDir, "test.txt")
	extractedContent, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(extractedContent, testContent) {
		t.Errorf("content mismatch: expected %s, got %s", testContent, extractedContent)
	}
}
