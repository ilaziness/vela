package sync

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// 缓冲区大小: 1MB，优化大文件传输性能
	bufferSize = 1024 * 1024
	// Machine-local; must not travel with sync packs.
	skipLocalStateName = "local-state.toml"
)

func shouldSkipSyncPath(relPath string) bool {
	base := filepath.Base(relPath)
	return base == skipLocalStateName
}

// archiveTargetPath joins dstDir with an archive entry name and rejects path escapes.
func archiveTargetPath(dstDir, name string) (string, error) {
	if name == "" || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("invalid archive entry name")
	}
	clean := filepath.Clean("/" + filepath.ToSlash(name))
	rel := strings.TrimPrefix(clean, "/")
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("invalid archive entry path: %q", name)
	}
	target := filepath.Join(dstDir, filepath.FromSlash(rel))
	absBase, err := filepath.Abs(dstDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	sep := string(os.PathSeparator)
	if absTarget != absBase && !strings.HasPrefix(absTarget, absBase+sep) {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	return target, nil
}

// PackStream 流式打包压缩加密
// srcDir: 源目录
// writer: 输出流（加密后的数据）
// password: 主密码（用于派生加密密钥）
func PackStream(srcDir string, writer io.Writer, password string) error {
	encryptStream, err := NewEncryptStream(writer, password)
	if err != nil {
		return fmt.Errorf("failed to create encrypt stream: %w", err)
	}

	bufferedWriter := bufio.NewWriterSize(encryptStream, bufferSize)
	gzipWriter, err := gzip.NewWriterLevel(bufferedWriter, gzip.BestSpeed)
	if err != nil {
		_ = encryptStream.Close()
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	tarWriter := tar.NewWriter(gzipWriter)

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if shouldSkipSyncPath(relPath) {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}

			buf := make([]byte, bufferSize)
			_, err = io.CopyBuffer(tarWriter, file, buf)
			file.Close()
			if err != nil {
				return err
			}
		}

		return nil
	})

	if closeErr := closePackWriters(tarWriter, gzipWriter, bufferedWriter, encryptStream); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to pack: %w", err)
	}
	return nil
}

func closePackWriters(tarWriter *tar.Writer, gzipWriter *gzip.Writer, bufferedWriter *bufio.Writer, encryptStream *EncryptStream) error {
	var firstErr error
	if err := tarWriter.Close(); err != nil {
		firstErr = err
	}
	if err := gzipWriter.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := bufferedWriter.Flush(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := encryptStream.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// UnpackStream 流式解密解压解包
// reader: 输入流（加密的数据）
// dstDir: 目标目录
// password: 主密码（用于派生解密密钥）
func UnpackStream(reader io.Reader, dstDir string, password string) error {
	bufferedReader := bufio.NewReaderSize(reader, bufferSize)

	decryptStream, err := NewDecryptStream(bufferedReader, password)
	if err != nil {
		return fmt.Errorf("failed to create decrypt stream: %w", err)
	}

	// 创建带缓冲的解密读取器
	bufferedDecryptReader := bufio.NewReaderSize(decryptStream, bufferSize)

	// 创建 gzip 解压流
	gzipReader, err := gzip.NewReader(bufferedDecryptReader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// 创建 tar 解包流
	tarReader := tar.NewReader(gzipReader)

	// 解包
	buf := make([]byte, bufferSize)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		targetPath, err := archiveTargetPath(dstDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			// 确保目录存在
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			// 创建文件
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// 使用大缓冲区复制文件内容
			_, err = io.CopyBuffer(file, tarReader, buf)
			file.Close()

			if err != nil {
				return err
			}
		}
	}

	return nil
}
