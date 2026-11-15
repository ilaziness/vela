package sftp

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/usererr"
)

// archive/zip rejects entry modification times before 1980 when writing.
var zipModTimeFloor = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// CompressRemotePaths builds a zip archive at archivePath (remote) from srcPaths,
// streaming file contents through SFTP without a local staging copy. Runs
// asynchronously under one upload-typed tracker so the transfer panel shows
// progress and the remote pane refreshes on completion.
func (sft *Manager) CompressRemotePaths(sessionID string, srcPaths []string, archivePath string) error {
	if sessionID == "" || len(srcPaths) == 0 || archivePath == "" {
		return usererr.Coded("sftp.invalid_compress", "Invalid compress parameters")
	}
	client, err := sft.getSftpClient(sessionID)
	if err != nil {
		return err
	}
	if _, statErr := client.Stat(archivePath); statErr == nil {
		return usererr.Coded("sftp.compress_target_exists", "%s already exists", archivePath)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	var total int64
	for _, src := range srcPaths {
		info, statErr := client.Stat(src)
		if statErr != nil {
			return usererr.Wrap(statErr, "sftp.remote_path", "Remote path %s: %s", src)
		}
		if info.IsDir() {
			dirTotal, err := sft.calcRemoteDirSize(sessionID, src)
			if err != nil {
				return usererr.Wrap(err, "sftp.remote_path", "Remote path %s: %s", src)
			}
			total += dirTotal
		} else {
			total += info.Size()
		}
	}
	names := make([]string, 0, len(srcPaths))
	for _, src := range srcPaths {
		names = append(names, path.Base(src))
	}
	tracker, err := sft.newTracker(sessionID, transfer.TypeUpload, strings.Join(names, ", "), archivePath, total, transfer.NewOpts{})
	if err != nil {
		return err
	}
	go func() {
		err := sft.compressTree(client, srcPaths, archivePath, tracker)
		sft.stopTracker(tracker, err)
	}()
	sft.notifyQueued(sessionID, 1)
	return nil
}

// compressTree writes the zip directly onto the remote archive file. zip.Writer
// is sequential, so streaming over the SFTP writer needs no local temp file.
// On any failure the partial archive is removed.
func (sft *Manager) compressTree(client *sftp.Client, srcPaths []string, archivePath string, tracker *transfer.Tracker) error {
	out, err := client.Create(archivePath)
	if err != nil {
		return err
	}
	// zipWalk names every top entry by its basename, so a single directory
	// keeps itself as the archive root (Windows explorer behavior) and multiple
	// selections each land at the root — no extra prefix needed here.
	zw := zip.NewWriter(out)
	var walkErr error
	for _, src := range srcPaths {
		if walkErr != nil {
			break
		}
		walkErr = sft.zipWalk(client, zw, src, "", tracker)
	}
	zipErr := zw.Close()
	fileErr := out.Close()
	if walkErr != nil || zipErr != nil || fileErr != nil {
		if rmErr := client.Remove(archivePath); rmErr != nil {
			sft.logger.Warn("compress: remove partial archive failed", zap.String("archive", archivePath), zap.Error(rmErr))
		}
	}
	if walkErr != nil {
		return walkErr
	}
	if zipErr != nil {
		return zipErr
	}
	return fileErr
}

// zipWalk adds one entry (recursively for directories) into the zip writer.
func (sft *Manager) zipWalk(client *sftp.Client, zw *zip.Writer, src, prefix string, tracker *transfer.Tracker) error {
	info, err := client.Stat(src)
	if err != nil {
		return err
	}
	name := prefix + path.Base(src)
	if info.IsDir() {
		entries, err := client.ReadDir(src)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			hdr := &zip.FileHeader{Name: name + "/"}
			hdr.SetModTime(safeZipModTime(info.ModTime()))
			if _, err := zw.CreateHeader(hdr); err != nil {
				return err
			}
			return nil
		}
		for _, entry := range entries {
			if err := sft.zipWalk(client, zw, joinRemotePath(src, entry.Name()), name+"/", tracker); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := client.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
	hdr.SetModTime(safeZipModTime(info.ModTime()))
	entryWriter, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(entryWriter, &transfer.Reader{Reader: in, Tracker: tracker})
	return err
}

func safeZipModTime(mod time.Time) time.Time {
	if mod.Before(zipModTimeFloor) {
		return zipModTimeFloor
	}
	return mod
}
