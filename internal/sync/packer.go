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
	skipWindowStateName = "window-state.toml"
)

func shouldSkipSyncPath(relPath string) bool {
	base := filepath.Base(relPath)
	return base == skipWindowStateName
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
// userKey: 用户密钥
func PackStream(srcDir string, writer io.Writer, userKey string) error {
	// 创建加密流
	encryptStream, err := NewEncryptStream(writer, userKey)
	if err != nil {
		return fmt.Errorf("failed to create encrypt stream: %w", err)
	}
	defer encryptStream.Close()

	// 创建带缓冲的写入器，提高性能
	bufferedWriter := bufio.NewWriterSize(encryptStream, bufferSize)
	defer bufferedWriter.Flush()

	// 创建 gzip 压缩流，使用最佳压缩级别
	gzipWriter, err := gzip.NewWriterLevel(bufferedWriter, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzipWriter.Close()

	// 创建 tar 归档流
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// 遍历目录并打包
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if shouldSkipSyncPath(relPath) {
			return nil
		}

		// 创建 tar 头
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = relPath

		// 写入头
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是文件，写入内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}

			// 使用大缓冲区复制文件内容
			buf := make([]byte, bufferSize)
			_, err = io.CopyBuffer(tarWriter, file, buf)
			file.Close()

			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to pack: %w", err)
	}

	return nil
}

// PackStreamWithProgress 带进度的流式打包压缩加密
func PackStreamWithProgress(srcDir string, writer io.Writer, userKey string, onProgress func(done, total int64)) error {
	// 先计算总大小
	var totalSize int64
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		if shouldSkipSyncPath(relPath) {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to calculate total size: %w", err)
	}

	// 创建加密流
	encryptStream, err := NewEncryptStream(writer, userKey)
	if err != nil {
		return fmt.Errorf("failed to create encrypt stream: %w", err)
	}
	defer encryptStream.Close()

	// 创建带缓冲的写入器
	bufferedWriter := bufio.NewWriterSize(encryptStream, bufferSize)
	defer bufferedWriter.Flush()

	// 创建 gzip 压缩流
	gzipWriter, err := gzip.NewWriterLevel(bufferedWriter, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzipWriter.Close()

	// 创建 tar 归档流
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// 遍历目录并打包
	var doneSize int64
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if shouldSkipSyncPath(relPath) {
			return nil
		}

		// 创建 tar 头
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = relPath

		// 写入头
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是文件，写入内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}

			// 使用自定义复制以跟踪进度
			buf := make([]byte, bufferSize)
			for {
				n, err := file.Read(buf)
				if n > 0 {
					if _, werr := tarWriter.Write(buf[:n]); werr != nil {
						file.Close()
						return werr
					}
					doneSize += int64(n)
					if onProgress != nil {
						onProgress(doneSize, totalSize)
					}
				}
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					file.Close()
					return err
				}
			}
			file.Close()
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to pack: %w", err)
	}

	return nil
}

// UnpackStream 流式解密解压解包
// reader: 输入流（加密的数据）
// dstDir: 目标目录
// userKey: 用户密钥
func UnpackStream(reader io.Reader, dstDir string, userKey string) error {
	// 创建带缓冲的读取器
	bufferedReader := bufio.NewReaderSize(reader, bufferSize)

	// 创建解密流
	decryptStream, err := NewDecryptStream(bufferedReader, userKey)
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

// UnpackStreamWithProgress 带进度的流式解密解压解包
func UnpackStreamWithProgress(reader io.Reader, dstDir string, userKey string, totalSize int64, onProgress func(done, total int64)) error {
	// 创建带缓冲的读取器
	bufferedReader := bufio.NewReaderSize(reader, bufferSize)

	// 创建解密流
	decryptStream, err := NewDecryptStream(bufferedReader, userKey)
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
	var doneSize int64
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

			// 使用自定义复制以跟踪进度
			for {
				n, err := tarReader.Read(buf)
				if n > 0 {
					if _, werr := file.Write(buf[:n]); werr != nil {
						file.Close()
						return werr
					}
					doneSize += int64(n)
					if onProgress != nil {
						onProgress(doneSize, totalSize)
					}
				}
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					file.Close()
					return err
				}
			}
			file.Close()
		}
	}

	return nil
}
