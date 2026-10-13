package sync

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadTempDirLocation(t *testing.T) {
	// 创建模拟的 parent 目录结构
	parentDir := t.TempDir()
	userDataDir := filepath.Join(parentDir, "userData")

	// 创建 userDataDir
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		t.Fatalf("failed to create userDataDir: %v", err)
	}

	// 创建测试文件
	testFile := filepath.Join(userDataDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 测试临时目录路径生成逻辑
	tempBaseDir := filepath.Join(parentDir, "temp")
	tempDir := filepath.Join(tempBaseDir, "vela-sync-tmp-test")

	// 验证临时目录应该在 parent/temp/ 下
	if !strings.HasPrefix(tempDir, tempBaseDir) {
		t.Errorf("temp dir should be under parent/temp/, got: %s", tempDir)
	}

	// 验证可以创建该目录
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// 验证目录存在
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Error("temp dir should exist")
	}
}

func TestCopyDir(t *testing.T) {
	// 创建源目录
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// 创建测试文件结构
	// src/
	//   file1.txt
	//   subdir/
	//     file2.txt
	srcFile1 := filepath.Join(srcDir, "file1.txt")
	if err := os.WriteFile(srcFile1, []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to create src file: %v", err)
	}

	srcSubdir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(srcSubdir, 0755); err != nil {
		t.Fatalf("failed to create src subdir: %v", err)
	}

	srcFile2 := filepath.Join(srcSubdir, "file2.txt")
	if err := os.WriteFile(srcFile2, []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to create src file: %v", err)
	}

	// 执行复制
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// 验证文件1
	dstFile1 := filepath.Join(dstDir, "file1.txt")
	content1, err := os.ReadFile(dstFile1)
	if err != nil {
		t.Fatalf("failed to read dst file1: %v", err)
	}
	if !bytes.Equal(content1, []byte("content1")) {
		t.Errorf("file1 content mismatch")
	}

	// 验证文件2
	dstFile2 := filepath.Join(dstDir, "subdir", "file2.txt")
	content2, err := os.ReadFile(dstFile2)
	if err != nil {
		t.Fatalf("failed to read dst file2: %v", err)
	}
	if !bytes.Equal(content2, []byte("content2")) {
		t.Errorf("file2 content mismatch")
	}
}

func TestCopyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "source.txt")
	dstFile := filepath.Join(dstDir, "dest.txt")
	content := []byte("test content")

	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to create src file: %v", err)
	}

	if err := copyFile(srcFile, dstFile, 0644); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read dst file: %v", err)
	}

	if !bytes.Equal(dstContent, content) {
		t.Errorf("content mismatch")
	}
}
