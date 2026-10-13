package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ilaziness/vela/internal/database"
)

// SyncManager 同步管理器
type SyncManager struct {
	config *SyncConfig
	client *Client
}

// NewSyncManager 创建同步管理器
func NewSyncManager(config *SyncConfig) *SyncManager {
	return &SyncManager{
		config: config,
		client: NewClient(config),
	}
}

// IsConfigured 检查是否已配置
func (sm *SyncManager) IsConfigured() bool {
	return sm.config.IsConfigured()
}

// Upload 上传本地数据到服务器
func (sm *SyncManager) Upload(srcDir string) error {
	if !sm.IsConfigured() {
		return fmt.Errorf("sync not configured")
	}

	// 创建管道用于流式上传
	pr, pw := io.Pipe()

	// 在后台执行打包加密
	go func() {
		defer pw.Close()
		err := PackStream(srcDir, pw, sm.config.UserKey)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	// 上传
	version, err := sm.client.Upload(pr)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}

	fmt.Printf("Upload successful, version: %d\n", version.VersionNumber)
	return nil
}

// copyDir 递归复制目录内容
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// 复制文件
		return copyFile(path, targetPath, info.Mode())
	})
}

// copyFile 复制单个文件
func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// restoreBackup 恢复备份目录
func restoreBackup(backupDir, dstDir string) error {
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return nil // 备份不存在，无需恢复
	}
	// 先清理目标目录（如果存在）
	if err := os.RemoveAll(dstDir); err != nil {
		// 清理失败但仍尝试恢复备份
		_ = os.Rename(backupDir, dstDir)
		return fmt.Errorf("failed to remove dst dir before restore: %w", err)
	}
	return os.Rename(backupDir, dstDir)
}

// Download 从服务器下载数据到本地
// db 参数用于在替换文件前关闭数据库连接
func (sm *SyncManager) Download(dstDir string, version int, db *database.Database) error {
	if !sm.IsConfigured() {
		return fmt.Errorf("sync not configured")
	}

	// 下载数据
	reader, err := sm.client.Download(version)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer reader.Close()

	// 创建临时目录在 parent/temp/ 下
	parentDir := filepath.Dir(dstDir)
	tempBaseDir := filepath.Join(parentDir, "temp")
	tmpDir := filepath.Join(tempBaseDir, "vela-sync-tmp-"+strconv.Itoa(int(time.Now().UnixNano())))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 流式解密解压解包到临时目录
	if err := UnpackStream(reader, tmpDir, sm.config.UserKey); err != nil {
		return fmt.Errorf("failed to unpack: %w", err)
	}

	// 验证临时目录不为空
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to read temp dir: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("temp dir is empty, unpack may have failed")
	}

	// 关闭数据库连接（解压完成后，替换文件前）
	if db != nil {
		db.Close()
	}

	// 备份原目录
	backupDir := dstDir + ".backup"
	if _, err := os.Stat(dstDir); err == nil {
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("failed to remove old backup: %w", err)
		}
		if err := os.Rename(dstDir, backupDir); err != nil {
			return fmt.Errorf("failed to backup: %w", err)
		}
	}

	// 创建新的目标目录
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		// 恢复备份
		if err := restoreBackup(backupDir, dstDir); err != nil {
			return fmt.Errorf("failed to create dst dir and restore backup: %w", err)
		}
		return fmt.Errorf("failed to create dst dir: %w", err)
	}

	// 将临时目录中的文件复制到目标目录
	if err := copyDir(tmpDir, dstDir); err != nil {
		// 恢复备份
		os.RemoveAll(dstDir)
		if err := restoreBackup(backupDir, dstDir); err != nil {
			return fmt.Errorf("failed to copy data and restore backup: %w", err)
		}
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// 清理备份
	os.RemoveAll(backupDir)

	fmt.Printf("Download successful\n")
	return nil
}

// ListVersions 获取版本列表（支持分页，limit=0表示不分页）
func (sm *SyncManager) ListVersions(limit, offset int) ([]VersionInfo, error) {
	if !sm.IsConfigured() {
		return nil, fmt.Errorf("sync not configured")
	}

	return sm.client.ListVersions(limit, offset)
}

// DeleteVersion 删除指定版本
func (sm *SyncManager) DeleteVersion(version int) error {
	if !sm.IsConfigured() {
		return fmt.Errorf("sync not configured")
	}

	return sm.client.DeleteVersion(version)
}

// HealthCheck 健康检查
func (sm *SyncManager) HealthCheck() error {
	if !sm.IsConfigured() {
		return fmt.Errorf("sync not configured")
	}

	return sm.client.HealthCheck()
}
