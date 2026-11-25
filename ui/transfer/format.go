package transferui

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/utils"
)

// transferDisplayName picks the most useful path for a queue row.
func transferDisplayName(p transfer.ProgressData) string {
	if p.TransferType == transfer.TypeUpload {
		if p.LocalFile != "" {
			return filepath.Base(p.LocalFile)
		}
		return path.Base(p.RemoteFile)
	}
	if p.RemoteFile != "" {
		return path.Base(p.RemoteFile)
	}
	return filepath.Base(p.LocalFile)
}

// transferTypeIcon marks upload vs download rows.
func transferTypeIcon(t string) string {
	if t == transfer.TypeUpload {
		return gui.IconUpload
	}
	return gui.IconDownload
}

// transferPathText renders the full "source → destination" path pair.
func transferPathText(p transfer.ProgressData) string {
	if p.TransferType == transfer.TypeUpload {
		return joinPath(p.LocalFile, p.RemoteFile)
	}
	return joinPath(p.RemoteFile, p.LocalFile)
}

// transferNameTip is the full path shown in the name tooltip: the
// local file for uploads, the remote file for downloads.
func transferNameTip(p transfer.ProgressData) string {
	if p.TransferType == transfer.TypeUpload && p.LocalFile != "" {
		return p.LocalFile
	}
	if p.RemoteFile != "" {
		return p.RemoteFile
	}
	return transferDisplayName(p)
}

func joinPath(src, dst string) string {
	switch {
	case src == "":
		return dst
	case dst == "":
		return src
	default:
		return src + " → " + dst
	}
}

// transferStatusText renders percent + rate or a terminal status.
func transferStatusText(p transfer.ProgressData) string {
	if p.Done {
		if p.Error != "" {
			return p.Error
		}
		return i18n.T("transfer.done")
	}
	parts := make([]string, 0, 2)
	if p.TotalSize > 0 {
		parts = append(parts, fmt.Sprintf("%.0f%%",
			clamp01(float64(p.Transferred)/float64(p.TotalSize))*100))
	}
	if p.Rate > 0 {
		parts = append(parts, formatRate(p.Rate))
	}
	return strings.Join(parts, " ")
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// formatRate renders bytes/sec humanely.
func formatRate(rate float64) string {
	return utils.FormatBytes(int64(rate)) + "/s"
}
